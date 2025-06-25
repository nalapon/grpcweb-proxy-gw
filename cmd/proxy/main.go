package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nalapon/grpcweb-proxy-gw/internal/config"
	"github.com/nalapon/grpcweb-proxy-gw/internal/fabric"
	"github.com/nalapon/grpcweb-proxy-gw/internal/server"
)

func newLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func main() {
	// 1. Load and validate configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		// Use standard log here because logger is not yet initialized
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize structured logger
	logger := newLogger(cfg.Log)
	slog.SetDefault(logger) // Set as global logger
	logger.Info("starting fabric-web-gateway-client gRPC-Web proxy...")
	logger.Debug("configuration loaded successfully", "config", cfg)

	// 3. Load TLS credentials from config
	var clientCert tls.Certificate
	var caCertPool *x509.CertPool
	if cfg.Fabric.Tls.Enabled {
		logger.Info("loading TLS credentials",
			"clientCert", cfg.Fabric.Tls.ClientCertPath,
			"clientKey", cfg.Fabric.Tls.ClientKeyPath)

		clientCert, err = tls.LoadX509KeyPair(
			cfg.Fabric.Tls.ClientCertPath,
			cfg.Fabric.Tls.ClientKeyPath,
		)
		if err != nil {
			logger.Error("failed to load client key pair", "error", err)
			os.Exit(1)
		}

		logger.Info("loading CA certificate", "caCert", cfg.Fabric.Tls.CaCertPath)
		caCert, err := os.ReadFile(cfg.Fabric.Tls.CaCertPath)
		if err != nil {
			logger.Error("failed to read peer TLS CA certificate", "error", err)
			os.Exit(1)
		}
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			logger.Error("failed to add peer TLS CA certificate to pool")
			os.Exit(1)
		}
	} else {
		logger.Warn("TLS is disabled. This is not recommended for production environments.")
	}

	// 4. Initialize components with dependencies
	connManagerLogger := logger.With(slog.String("component", "connection_manager"))
	connManager := fabric.NewConnectionManager(
		clientCert,
		caCertPool,
		cfg.Fabric.Tls.Enabled,
		connManagerLogger,
	)
	defer connManager.CloseConnection()

	serverLogger := logger.With(slog.String("component", "server"))
	proxyServer := server.New(cfg, connManager, serverLogger)

	// 5. Start server and handle graceful shutdown
	go func() {
		logger.Info("proxy server listening", "address", cfg.Server.ListenAddr)
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("failed to start proxy server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down fabric-web-gateway-client gRPC-Web proxy...")

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := proxyServer.Shutdown(ctx); err != nil {
		logger.Error("failed to gracefully shutdown proxy server", "error", err)
	}

	logger.Info("fabric-web-gateway-client gRPC-Web proxy shutdown complete")
}
