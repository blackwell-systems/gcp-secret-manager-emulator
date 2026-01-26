// GCP Secret Manager Mock Server - REST API
//
// A REST/HTTP implementation of Google Cloud Secret Manager API for local testing.
// This server runs a gRPC backend with an HTTP/REST gateway frontend.
//
// Usage:
//
//	server-rest --http-port 8080 --grpc-port 9090
//
// Environment Variables:
//
//	GCP_MOCK_HTTP_PORT   - HTTP port to listen on (default: 8080)
//	GCP_MOCK_GRPC_PORT   - gRPC port to listen on (default: 9090)
//	GCP_MOCK_LOG_LEVEL   - Log level: debug, info, warn, error (default: info)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/gateway"
	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/server"
)

var (
	httpPort = flag.Int("http-port", getEnvInt("GCP_MOCK_HTTP_PORT", 8080), "HTTP port to listen on")
	grpcPort = flag.Int("grpc-port", getEnvInt("GCP_MOCK_GRPC_PORT", 9090), "gRPC port to listen on (internal)")
	logLevel = flag.String("log-level", getEnv("GCP_MOCK_LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	version  = "1.1.0"
)

func main() {
	flag.Parse()

	log.Printf("GCP Secret Manager Mock Server v%s (REST API)", version)
	log.Printf("Starting gRPC backend on port %d", *grpcPort)
	log.Printf("Starting HTTP gateway on port %d", *httpPort)
	log.Printf("Log level: %s", *logLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gRPC server
	grpcAddr := fmt.Sprintf("localhost:%d", *grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port: %v", err)
	}

	grpcServer := grpc.NewServer()
	mockServer := server.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, mockServer)
	reflection.Register(grpcServer)

	// Start gRPC server in background
	go func() {
		log.Printf("gRPC server listening at %v", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start REST gateway
	httpAddr := fmt.Sprintf(":%d", *httpPort)
	gateway := gateway.NewServer(grpcAddr)

	go func() {
		log.Printf("HTTP gateway listening at %s", httpAddr)
		log.Printf("Ready to accept REST requests")
		log.Printf("Example: curl http://localhost:%d/v1/projects/test-project/secrets", *httpPort)
		if err := gateway.Start(ctx, httpAddr); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Shutdown REST gateway
	if err := gateway.Stop(ctx); err != nil {
		log.Printf("Error stopping HTTP gateway: %v", err)
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	log.Println("Servers stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
