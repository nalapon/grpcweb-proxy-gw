// internal/server/server.go
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/mwitkow/grpc-proxy/proxy"
	"github.com/nalapon/grpcweb-proxy-gw/internal/config"
	"github.com/nalapon/grpcweb-proxy-gw/internal/fabric" // Import correcto
	"github.com/rs/cors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Server struct {
	httpServer  *http.Server
	connManager *fabric.ConnectionManager // Tipo correcto del paquete fabric
	config      config.Config             // Añadido para tener acceso a la config
	logger      *slog.Logger              // Añadido para el logging
}

// New - El constructor ahora acepta config y logger
func New(cfg config.Config, connManager *fabric.ConnectionManager, logger *slog.Logger) *Server {
	s := &Server{
		connManager: connManager,
		config:      cfg,
		logger:      logger,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ws/deliver", s.handleDeliverWebSocket)
	s.logger.Info("registered handler for /ws/deliver")

	mux.Handle("/", s.grpcWebHandler())
	s.logger.Info("registered handler for /")

	s.httpServer = &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: s.corsHandler(mux), // Envolvemos el mux con CORS
	}
	return s
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) grpcWebHandler() http.Handler {
	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, _ := metadata.FromIncomingContext(ctx)

		// Usamos los nombres correctos de la configuración
		targetPeerAddress := s.config.Fabric.GatewayAddress
		hostnameOverride := s.config.Fabric.Hostname

		if targets := md.Get("x-fabric-target-peer"); len(targets) > 0 {
			targetPeerAddress = targets[0]
			s.logger.Debug("rerouting to target peer from header", "peer", targetPeerAddress)
		}

		conn, err := s.connManager.GetConnection(targetPeerAddress, hostnameOverride)
		if err != nil {
			s.logger.Error(
				"failed to get connection for target",
				"peer",
				targetPeerAddress,
				"error",
				err,
			)
			return nil, nil, err
		}

		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		return outCtx, conn, nil
	}

	grpcProxy := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
	)

	return grpcweb.WrapServer(grpcProxy)
}

// Función helper para la configuración de CORS
func (s *Server) corsHandler(h http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins:   s.config.Server.AllowedOrigins,
		AllowedMethods:   []string{"POST", "GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "x-grpc-web", "x-fabric-target-peer"},
		ExposedHeaders:   []string{"grpc-status", "grpc-message"},
		AllowCredentials: true,
	}).Handler(h)
}
