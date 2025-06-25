package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/proto"
)

func (s *Server) createUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, allowedOrigin := range s.config.Server.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					return true
				}
			}
			s.logger.Warn("websocket upgrade request from disallowed origin", "origin", origin)
			return false
		},
	}
}

func (s *Server) handleDeliverWebSocket(w http.ResponseWriter, r *http.Request) {
	handlerLogger := s.logger.With(slog.String("handler", "deliver_websocket"))
	handlerLogger.Info("received new websocket upgrade request for deliver service")

	targetPeerAddress := r.URL.Query().Get("target")
	if targetPeerAddress == "" {
		handlerLogger.Warn("missing target peer address in request query parameters")
		http.Error(
			w,
			"missing target peer address in request query parameters",
			http.StatusBadRequest,
		)
		return
	}

	hostnameOverride := r.URL.Query().Get("hostname")
	if hostnameOverride == "" {
		// Use target peer address as hostname if not specified, a common practice
		hostnameOverride = targetPeerAddress
	}

	handlerLogger.Info("routing request", "peer", targetPeerAddress, "hostname", hostnameOverride)

	peerConn, err := s.connManager.GetConnection(targetPeerAddress, hostnameOverride)
	if err != nil {
		handlerLogger.Error(
			"failed to get connection to peer",
			"peer",
			targetPeerAddress,
			"error",
			err,
		)
		http.Error(w, "failed to connect to backend Fabric peer", http.StatusInternalServerError)
		return
	}

	upgrader := s.createUpgrader()
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handlerLogger.Error("failed to upgrade connection", "error", err)
		return
	}
	defer wsConn.Close()

	deliverClient := peer.NewDeliverClient(peerConn)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	grpcStream, err := deliverClient.DeliverFiltered(ctx)
	if err != nil {
		handlerLogger.Error("failed to create gRPC bidi stream 'DeliverFiltered'", "error", err)
		wsConn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(
				websocket.CloseInternalServerErr,
				"failed to create gRPC stream",
			),
		)
		return
	}

	// Goroutine: Fabric (gRPC) -> Client (WebSocket)
	go func() {
		defer cancel()
		defer wsConn.Close()
		for {
			deliverResponse, err := grpcStream.Recv()
			if err != nil {
				handlerLogger.Warn(
					"failed to receive from gRPC stream (Fabric -> WS)",
					"error",
					err,
				)
				wsConn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(
						websocket.CloseNormalClosure,
						"gRPC stream closed",
					),
				)
				return
			}

			// CORRECCIÓN: Usar deliverResponse
			responseBytes, err := proto.Marshal(deliverResponse)
			if err != nil {
				handlerLogger.Error("failed to marshal gRPC DeliverResponse", "error", err)
				continue
			}

			if err := wsConn.WriteMessage(websocket.BinaryMessage, responseBytes); err != nil {
				handlerLogger.Warn("error writing to WebSocket (Fabric -> WS)", "error", err)
				return
			}

			handlerLogger.Debug(
				"relayed message from Fabric to WebSocket client",
				"bytes",
				len(responseBytes),
			)
		}
	}()

	// Loop: Client (WebSocket) -> Fabric (gRPC)
	for {
		msgType, p, err := wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
			) {
				handlerLogger.Error("error reading from WebSocket (WS->Fabric)", "error", err)
			} else {
				handlerLogger.Info("websocket client disconnected gracefully")
			}
			return
		}

		if msgType == websocket.BinaryMessage {
			envelope := &common.Envelope{}
			if err := proto.Unmarshal(p, envelope); err != nil {
				handlerLogger.Error("failed to unmarshal Envelope from WebSocket", "error", err)
				continue
			}

			if err := grpcStream.Send(envelope); err != nil {
				handlerLogger.Error("error sending to gRPC stream (WS->Fabric)", "error", err)
				return
			}
			handlerLogger.Debug("relayed Envelope from WebSocket client to Fabric")
		}
	}
}
