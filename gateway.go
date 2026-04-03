package gcpemulator

import (
	"net/http"

	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/gateway"
)

// NewGatewayHandler returns an http.Handler that proxies REST requests to the
// Secret Manager gRPC service at grpcAddr. Used by gcp-emulator to mount the
// SM REST API onto a unified HTTP server.
func NewGatewayHandler(grpcAddr string) (http.Handler, error) {
	srv, err := gateway.NewServer(grpcAddr)
	if err != nil {
		return nil, err
	}
	return srv.Handler(), nil
}
