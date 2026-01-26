// GCP Secret Manager Mock Server - Dual Protocol
//
// Provides both gRPC and REST/HTTP APIs for Google Cloud Secret Manager.
// This server exposes both protocols simultaneously for maximum flexibility.
//
// Usage:
//
//	server-dual --grpc-port 9090 --http-port 8080
//
// Environment Variables:
//
//	GCP_MOCK_GRPC_PORT   - gRPC port to listen on (default: 9090)
//	GCP_MOCK_HTTP_PORT   - HTTP port to listen on (default: 8080)
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
	grpcPort = flag.Int("grpc-port", getEnvInt("GCP_MOCK_GRPC_PORT", 9090), "gRPC port to listen on")
	httpPort = flag.Int("http-port", getEnvInt("GCP_MOCK_HTTP_PORT", 8080), "HTTP port to listen on")
	logLevel = flag.String("log-level", getEnv("GCP_MOCK_LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	version  = "1.1.0"
)

func main() {
	flag.Parse()

	log.Printf("GCP Secret Manager Mock Server v%s (Dual Protocol)", version)
	log.Printf("Log level: %s", *logLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", *grpcPort)
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
	gatewayServer := gateway.NewServer(fmt.Sprintf("localhost:%d", *grpcPort))

	go func() {
		log.Printf("HTTP gateway listening at %s", httpAddr)
		log.Printf("Ready to accept both gRPC and REST requests")
		log.Printf("gRPC: localhost:%d", *grpcPort)
		log.Printf("REST: http://localhost:%d/v1/projects/{project}/secrets", *httpPort)
		if err := gatewayServer.Start(ctx, httpAddr); err != nil {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Shutdown REST gateway
	if err := gatewayServer.Stop(ctx); err != nil {
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
