// GCP Secret Manager Mock Server
//
// A lightweight mock implementation of Google Cloud Secret Manager API for local testing.
// This server implements the gRPC Secret Manager API without requiring GCP credentials.
//
// Usage:
//
//	gcp-secret-manager-mock --port 9090
//
// Environment Variables:
//
//	GCP_MOCK_PORT        - Port to listen on (default: 9090)
//	GCP_MOCK_LOG_LEVEL   - Log level: debug, info, warn, error (default: info)
package main

import (
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

	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/server"
)

var (
	port     = flag.Int("port", getEnvInt("GCP_MOCK_PORT", 9090), "Port to listen on")
	logLevel = flag.String("log-level", getEnv("GCP_MOCK_LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	version  = "1.1.0" // Will be updated during releases
)

func main() {
	flag.Parse()

	log.Printf("GCP Secret Manager Mock Server v%s", version)
	log.Printf("Starting on port %d with log level: %s", *port, *logLevel)

	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create and register mock service
	mockServer := server.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, mockServer)

	// Register reflection service (for grpc_cli debugging)
	reflection.Register(grpcServer)

	log.Printf("Server listening at %v", lis.Addr())
	log.Printf("Ready to accept connections")

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
