// Package ingest provides the OTLP-compatible gRPC server for event ingestion.
package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"time"
)

// EventHandler processes incoming agent events.
type EventHandler interface {
	HandleHTTPSpan(ctx context.Context, span interface{}) error
	HandleTCPEvent(ctx context.Context, event interface{}) error
	HandleSyscallSummary(ctx context.Context, summary interface{}) error
	HandleOOMEvent(ctx context.Context, event interface{}) error
	HandleExecEvent(ctx context.Context, event interface{}) error
	HandleHeartbeat(ctx context.Context, heartbeat interface{}) error
}

// Server is the OTLP-compatible gRPC ingestion server.
type Server struct {
	mu       sync.RWMutex
	listener net.Listener
	grpc     *grpc.Server
	handler  EventHandler
	logger   *slog.Logger

	// Metrics
	eventsReceived atomic.Int64
	eventsDropped  atomic.Int64
	activeStreams   atomic.Int32
}

// NewServer creates a new ingestion server.
func NewServer(handler EventHandler, logger *slog.Logger) *Server {
	s := &Server{
		handler: handler,
		logger:  logger,
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
		grpc.MaxRecvMsgSize(16 * 1024 * 1024), // 16MB max message
	}

	s.grpc = grpc.NewServer(opts...)

	// TODO: Register AgentService gRPC handler
	// pb.RegisterAgentServiceServer(s.grpc, s)

	return s
}

// Start begins listening on the given address.
func (s *Server) Start(addr string) error {
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	s.logger.Info("ingestion server listening", "addr", addr)

	go func() {
		if err := s.grpc.Serve(s.listener); err != nil {
			s.logger.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server.
func (s *Server) Stop() {
	s.grpc.GracefulStop()
	if s.listener != nil {
		s.listener.Close()
	}
}

// StreamEvents handles the bidirectional streaming from agents.
// This will be the actual gRPC handler once protobuf codegen is done.
func (s *Server) StreamEvents(stream interface{}) error {
	s.activeStreams.Add(1)
	defer s.activeStreams.Add(-1)

	s.logger.Info("new agent stream connected")

	// TODO: Read from stream, dispatch to handler
	// This is a placeholder that will be filled in when proto codegen is set up

	return nil
}

// Stats returns server statistics.
func (s *Server) Stats() ServerStats {
	return ServerStats{
		EventsReceived: s.eventsReceived.Load(),
		EventsDropped:  s.eventsDropped.Load(),
		ActiveStreams:   s.activeStreams.Load(),
	}
}

// ServerStats contains server metrics.
type ServerStats struct {
	EventsReceived int64
	EventsDropped  int64
	ActiveStreams   int32
}

// Ensure io.Reader is imported (used elsewhere in the package).
var _ io.Reader
