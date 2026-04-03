// Package gateway provides HTTP/REST gateway access to the gRPC Secret Manager
// service using grpc-gateway v2 to transcode HTTP/JSON ↔ gRPC.
package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	smv1 "github.com/blackwell-systems/gcp-secret-manager-emulator/internal/gen/google/cloud/secretmanager/v1"
)

// jsonErrorHandler intercepts proto/JSON unmarshal errors from grpc-gateway and
// returns a clean 400 instead of leaking raw Go parse internals to the caller.
func jsonErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	msg := err.Error()
	if strings.HasPrefix(msg, "proto:") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "unexpected end of JSON") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"request body is not valid JSON"}`))
		return
	}
	runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
}

// Server represents the REST gateway server.
type Server struct {
	mux     *runtime.ServeMux
	httpSrv *http.Server
	conn    *grpc.ClientConn
}

// NewServer creates a new REST gateway server that proxies to a gRPC server.
func NewServer(grpcAddr string) (*Server, error) {
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	mux := runtime.NewServeMux(runtime.WithErrorHandler(jsonErrorHandler))
	ctx := context.Background()

	if err := smv1.RegisterSecretManagerServiceHandlerClient(ctx, mux, smv1.NewSecretManagerServiceClient(conn)); err != nil {
		conn.Close()
		return nil, err
	}

	// Health endpoints.
	healthHandler := func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	_ = mux.HandlePath("GET", "/healthz", healthHandler)
	_ = mux.HandlePath("GET", "/readyz", healthHandler)

	return &Server{mux: mux, conn: conn}, nil
}

// Handler returns the HTTP handler for this gateway, suitable for mounting
// into a parent mux (e.g. the unified gcp-emulator gateway).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start starts the REST gateway server on the specified address.
func (s *Server) Start(ctx context.Context, addr string) error {
	s.httpSrv = &http.Server{Addr: addr, Handler: s.mux}
	return s.httpSrv.ListenAndServe()
}

// Stop gracefully stops the REST gateway server.
func (s *Server) Stop(ctx context.Context) error {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}
