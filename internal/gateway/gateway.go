// Package gateway provides HTTP/REST API access to the gRPC Secret Manager service
package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Server represents the REST gateway server
type Server struct {
	grpcClient secretmanagerpb.SecretManagerServiceClient
	httpServer *http.Server
	conn       *grpc.ClientConn
}

// NewServer creates a new REST gateway server that proxies to a gRPC server
func NewServer(grpcAddr string) *Server {
	// Connect to gRPC server
	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to dial gRPC server: %v", err))
	}

	return &Server{
		grpcClient: secretmanagerpb.NewSecretManagerServiceClient(conn),
		conn:       conn,
	}
}

// Start starts the REST gateway server on the specified address
func (s *Server) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	// Register routes matching GCP's REST API
	mux.HandleFunc("/v1/", s.handleRequest)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy"}`)
	})

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the REST gateway server
func (s *Server) Stop(ctx context.Context) error {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleRequest routes REST requests to appropriate gRPC calls
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse path: /v1/projects/{project}/secrets/{secret}/versions/{version}
	path := strings.TrimPrefix(r.URL.Path, "/v1/")

	// Handle :verb suffixes (like :addVersion, :access, etc.)
	// Split on both / and :
	parts := strings.Split(path, "/")

	// Set JSON content type
	w.Header().Set("Content-Type", "application/json")

	// Route based on path structure and HTTP method
	if len(parts) >= 2 && parts[0] == "projects" {
		project := fmt.Sprintf("projects/%s", parts[1])

		// Secrets operations
		if len(parts) == 3 && parts[2] == "secrets" {
			switch r.Method {
			case http.MethodGet:
				s.listSecrets(ctx, w, r, project)
			case http.MethodPost:
				s.createSecret(ctx, w, r, project)
			default:
				http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			}
			return
		}

		// AddVersion operation (handles /secrets/name:addVersion path structure)
		// Check this BEFORE individual secret operations
		if len(parts) == 4 && parts[2] == "secrets" && strings.HasSuffix(parts[3], ":addVersion") {
			secretName := fmt.Sprintf("%s/secrets/%s", project, strings.TrimSuffix(parts[3], ":addVersion"))
			if r.Method == http.MethodPost {
				s.addSecretVersion(ctx, w, r, secretName)
			} else {
				http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			}
			return
		}

		// Individual secret operations
		if len(parts) == 4 && parts[2] == "secrets" {
			secretName := fmt.Sprintf("%s/secrets/%s", project, parts[3])
			switch r.Method {
			case http.MethodGet:
				s.getSecret(ctx, w, r, secretName)
			case http.MethodPatch:
				s.updateSecret(ctx, w, r, secretName)
			case http.MethodDelete:
				s.deleteSecret(ctx, w, r, secretName)
			default:
				http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			}
			return
		}

		// Secret versions operations
		if len(parts) == 5 && parts[4] == "versions" {
			secretName := fmt.Sprintf("%s/secrets/%s", project, parts[3])
			switch r.Method {
			case http.MethodGet:
				s.listSecretVersions(ctx, w, r, secretName)
			default:
				http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			}
			return
		}

		// Individual version operations
		if len(parts) == 6 && parts[4] == "versions" {
			versionName := fmt.Sprintf("%s/secrets/%s/versions/%s", project, parts[3], parts[5])

			// Check for :access suffix
			if strings.HasSuffix(parts[5], ":access") {
				versionName = strings.TrimSuffix(versionName, ":access")
				s.accessSecretVersion(ctx, w, r, versionName)
				return
			}

			// Check for :enable suffix
			if strings.HasSuffix(parts[5], ":enable") {
				versionName = strings.TrimSuffix(versionName, ":enable")
				s.enableSecretVersion(ctx, w, r, versionName)
				return
			}

			// Check for :disable suffix
			if strings.HasSuffix(parts[5], ":disable") {
				versionName = strings.TrimSuffix(versionName, ":disable")
				s.disableSecretVersion(ctx, w, r, versionName)
				return
			}

			// Check for :destroy suffix
			if strings.HasSuffix(parts[5], ":destroy") {
				versionName = strings.TrimSuffix(versionName, ":destroy")
				s.destroySecretVersion(ctx, w, r, versionName)
				return
			}

			switch r.Method {
			case http.MethodGet:
				s.getSecretVersion(ctx, w, r, versionName)
			case http.MethodDelete:
				s.destroySecretVersion(ctx, w, r, versionName)
			default:
				http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			}
			return
		}
	}

	http.Error(w, `{"error":"Not found"}`, http.StatusNotFound)
}

// Helper to write protobuf response as JSON
func writeProtoJSON(w http.ResponseWriter, msg interface{}) {
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}

	// Type assert to proto.Message
	protoMsg, ok := msg.(interface{ ProtoReflect() protoreflect.Message })
	if !ok {
		http.Error(w, `{"error":"Failed to marshal response: not a proto message"}`, http.StatusInternalServerError)
		return
	}

	data, err := marshaler.Marshal(protoMsg)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to marshal response: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(data); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to write response: %v"}`, err), http.StatusInternalServerError)
	}
}

// Secrets operations
func (s *Server) listSecrets(ctx context.Context, w http.ResponseWriter, r *http.Request, parent string) {
	req := &secretmanagerpb.ListSecretsRequest{
		Parent:    parent,
		PageSize:  100,
		PageToken: r.URL.Query().Get("pageToken"),
	}

	resp, err := s.grpcClient.ListSecrets(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) createSecret(ctx context.Context, w http.ResponseWriter, r *http.Request, parent string) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var secret secretmanagerpb.Secret
	if err := protojson.Unmarshal(body, &secret); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	secretID := r.URL.Query().Get("secretId")

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret:   &secret,
	}

	resp, err := s.grpcClient.CreateSecret(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeProtoJSON(w, resp)
}

func (s *Server) getSecret(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.GetSecretRequest{Name: name}

	resp, err := s.grpcClient.GetSecret(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusNotFound)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) updateSecret(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var secret secretmanagerpb.Secret
	if err := protojson.Unmarshal(body, &secret); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	secret.Name = name

	req := &secretmanagerpb.UpdateSecretRequest{
		Secret: &secret,
	}

	resp, err := s.grpcClient.UpdateSecret(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) deleteSecret(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.DeleteSecretRequest{Name: name}

	_, err := s.grpcClient.DeleteSecret(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Secret version operations
func (s *Server) addSecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, parent string) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var reqBody struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &reqBody); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(reqBody.Payload.Data)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid base64 data: %v"}`, err), http.StatusBadRequest)
		return
	}

	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	}

	resp, err := s.grpcClient.AddSecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) listSecretVersions(ctx context.Context, w http.ResponseWriter, r *http.Request, parent string) {
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent:    parent,
		PageSize:  100,
		PageToken: r.URL.Query().Get("pageToken"),
	}

	resp, err := s.grpcClient.ListSecretVersions(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) getSecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.GetSecretVersionRequest{Name: name}

	resp, err := s.grpcClient.GetSecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusNotFound)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) accessSecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.AccessSecretVersionRequest{Name: name}

	resp, err := s.grpcClient.AccessSecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) enableSecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.EnableSecretVersionRequest{Name: name}

	resp, err := s.grpcClient.EnableSecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) disableSecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.DisableSecretVersionRequest{Name: name}

	resp, err := s.grpcClient.DisableSecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}

func (s *Server) destroySecretVersion(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) {
	req := &secretmanagerpb.DestroySecretVersionRequest{Name: name}

	resp, err := s.grpcClient.DestroySecretVersion(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	writeProtoJSON(w, resp)
}
