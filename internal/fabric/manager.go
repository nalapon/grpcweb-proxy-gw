package fabric

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ConnectionManager struct {
	mu            sync.Mutex
	connections   map[string]*grpc.ClientConn
	baseTlsConfig *tls.Config
	tlsEnabled    bool
	logger        *slog.Logger
}

// NewConnectionManager - Esta es la firma correcta con 4 argumentos.
func NewConnectionManager(
	clientCert tls.Certificate,
	caCertPool *x509.CertPool,
	tlsEnabled bool, // Argumento 3: para saber si usar TLS
	logger *slog.Logger, // Argumento 4: el logger
) *ConnectionManager {
	var tlsConfig *tls.Config
	if tlsEnabled {
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
		}
	}
	return &ConnectionManager{
		connections:   make(map[string]*grpc.ClientConn),
		baseTlsConfig: tlsConfig,
		tlsEnabled:    tlsEnabled,
		logger:        logger,
	}
}

func (m *ConnectionManager) GetConnection(
	peerAddress string,
	hostnameOverride string,
) (*grpc.ClientConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.connections[peerAddress]; ok {
		currentState := conn.GetState()
		m.logger.Debug(
			"reusing existing connection",
			"peer",
			peerAddress,
			"state",
			currentState.String(),
		)
		if currentState != connectivity.Shutdown {
			return conn, nil
		}
		m.logger.Warn("connection is in shutdown state, creating a new one", "peer", peerAddress)
		delete(m.connections, peerAddress)
	}

	m.logger.Info("creating new gRPC connection", "peer", peerAddress, "hostname", hostnameOverride)

	var transportCreds credentials.TransportCredentials
	if m.tlsEnabled {
		tlsConf := m.baseTlsConfig.Clone()
		tlsConf.ServerName = hostnameOverride
		transportCreds = credentials.NewTLS(tlsConf)
	} else {
		m.logger.Warn("creating insecure gRPC connection as TLS is disabled")
		transportCreds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(
		peerAddress,
		grpc.WithTransportCredentials(transportCreds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s: %w", peerAddress, err)
	}

	m.connections[peerAddress] = conn
	return conn, nil
}

func (m *ConnectionManager) CloseConnection() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("closing all gRPC connections")
	for addr, conn := range m.connections {
		if err := conn.Close(); err != nil {
			m.logger.Error("error closing connection", "address", addr, "error", err)
		}
		delete(m.connections, addr)
	}
	m.logger.Info("all gRPC connections closed.")
}
