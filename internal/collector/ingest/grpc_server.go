// Package ingest provides the gRPC server that receives event streams from
// all KernelView agents and dispatches them to the storage and anomaly
// detection pipelines.
package ingest

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"time"
)

// EventHandler processes a batch of events from an agent.
type EventHandler func(nodeName string, events []byte) error

// GRPCServer receives gRPC streams from agents and dispatches events.
type GRPCServer struct {
	logger    *slog.Logger
	server    *grpc.Server
	handlers  []EventHandler
	mu        sync.RWMutex

	// Metrics
	connectedAgents atomic.Int64
	totalBatches    atomic.Int64
	totalEvents     atomic.Int64
}

// NewGRPCServer creates the collector's gRPC ingest server.
func NewGRPCServer(logger *slog.Logger) *GRPCServer {
	s := &GRPCServer{
		logger:   logger,
		handlers: make([]EventHandler, 0),
	}

	s.server = grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
		grpc.MaxRecvMsgSize(16 * 1024 * 1024), // 16MB max message
	)

	// Register the AgentService handler
	// When proto codegen is complete:
	// agentpb.RegisterAgentServiceServer(s.server, s)

	return s
}

// OnEvent registers a handler that is called for each event batch.
// Multiple handlers can be registered (storage, anomaly engine, etc.)
func (s *GRPCServer) OnEvent(handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
}

// Serve starts listening for agent connections on the given address.
func (s *GRPCServer) Serve(ctx context.Context, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.logger.Info("gRPC ingest server listening", "addr", addr)

	// Graceful shutdown on context cancellation
	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down gRPC server")
		s.server.GracefulStop()
	}()

	return s.server.Serve(lis)
}

// dispatchToHandlers sends an event batch to all registered handlers.
func (s *GRPCServer) dispatchToHandlers(nodeName string, data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, handler := range s.handlers {
		if err := handler(nodeName, data); err != nil {
			s.logger.Error("event handler failed", "node", nodeName, "error", err)
		}
	}
}

// Stats returns current server statistics.
func (s *GRPCServer) Stats() map[string]int64 {
	return map[string]int64{
		"connected_agents": s.connectedAgents.Load(),
		"total_batches":    s.totalBatches.Load(),
		"total_events":     s.totalEvents.Load(),
	}
}

// StreamEvents implements the AgentService.StreamEvents RPC.
// This is the bidirectional streaming handler called by each agent.
//
// Once proto codegen is complete, uncomment the method signature:
// func (s *GRPCServer) StreamEvents(stream agentpb.AgentService_StreamEventsServer) error {
//
// For now, the event dispatch logic is defined here as a reference implementation:
func (s *GRPCServer) handleAgentStream(ctx context.Context, nodeName string, recv func() ([]byte, error)) error {
	s.connectedAgents.Add(1)
	defer s.connectedAgents.Add(-1)

	s.logger.Info("agent connected", "node", nodeName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		data, err := recv()
		if err != nil {
			s.logger.Error("agent stream error", "node", nodeName, "error", err)
			return err
		}

		s.totalBatches.Add(1)
		s.dispatchToHandlers(nodeName, data)
	}
}
