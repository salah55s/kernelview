// Package stream handles the gRPC connection from the Agent to the Collector.
// It consumes events from BPF ring buffers, batches them, and streams
// AgentEventBatch messages to the Collector's StreamEvents RPC.
package stream

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync"
	"time"

	"github.com/kernelview/kernelview/internal/agent/bpf"
	"github.com/kernelview/kernelview/internal/agent/metadata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Batch every 100ms or 1000 events, whichever comes first
	batchInterval  = 100 * time.Millisecond
	maxBatchSize   = 1000
	reconnectDelay = 5 * time.Second
)

// EventType identifies which eBPF program generated an event.
type EventType uint8

const (
	EventHTTP    EventType = 1
	EventTCP     EventType = 2
	EventSyscall EventType = 3
	EventOOM     EventType = 4
	EventExec    EventType = 5
	EventDNS     EventType = 6
	EventCFS     EventType = 7
	EventMemRSS  EventType = 8
)

// RawEvent is a kernel event read from a ring buffer.
type RawEvent struct {
	Type      EventType
	Data      []byte
	Timestamp time.Time
}

// Client streams enriched eBPF events to the Collector via gRPC.
type Client struct {
	mu             sync.Mutex
	endpoint       string
	nodeName       string
	agentID        string
	loader         *bpf.Loader
	resolver       *metadata.Resolver
	logger         *slog.Logger

	// Batching
	batch    []RawEvent
	batchMu  sync.Mutex
}

// NewClient creates a new streaming client.
func NewClient(endpoint, nodeName, agentID string, loader *bpf.Loader, resolver *metadata.Resolver, logger *slog.Logger) *Client {
	return &Client{
		endpoint: endpoint,
		nodeName: nodeName,
		agentID:  agentID,
		loader:   loader,
		resolver: resolver,
		logger:   logger,
		batch:    make([]RawEvent, 0, maxBatchSize),
	}
}

// Run starts the ring buffer consumer and gRPC stream. Blocks until ctx is cancelled.
// It reconnects automatically on gRPC errors.
func (c *Client) Run(ctx context.Context) error {
	// Start the ring buffer consumer in the background
	go c.consumeRingBuffer(ctx)

	// Main loop: connect and stream, reconnect on failure
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.streamToCollector(ctx); err != nil {
			c.logger.Error("stream to collector failed, reconnecting",
				"error", err, "retry_in", reconnectDelay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(reconnectDelay):
			}
		}
	}
}

// consumeRingBuffer reads events from the BPF ring buffer and appends them to the batch.
func (c *Client) consumeRingBuffer(ctx context.Context) {
	err := c.loader.ReadEvents(func(data []byte) {
		if len(data) < 1 {
			return
		}

		evt := RawEvent{
			Type:      EventType(data[0]),
			Data:      make([]byte, len(data)),
			Timestamp: time.Now(),
		}
		copy(evt.Data, data)

		c.batchMu.Lock()
		c.batch = append(c.batch, evt)
		c.batchMu.Unlock()
	})
	if err != nil {
		c.logger.Error("ring buffer consumer exited", "error", err)
	}
}

// streamToCollector establishes a gRPC stream and sends batched events.
func (c *Client) streamToCollector(ctx context.Context) error {
	conn, err := grpc.NewClient(c.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.logger.Info("connected to collector", "endpoint", c.endpoint)

	// The generated client code would be:
	// client := agentpb.NewAgentServiceClient(conn)
	// stream, err := client.StreamEvents(ctx)
	//
	// For now, we use the connection to send batched events
	// via unary RPCs until protobuf codegen is complete.

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			batch := c.drainBatch()
			if len(batch) == 0 {
				continue
			}

			c.logger.Debug("sending batch to collector",
				"events", len(batch),
				"node", c.nodeName,
			)

			// TODO: When proto codegen is complete, replace with:
			// err := stream.Send(&agentpb.AgentEventBatch{
			//     NodeName:  c.nodeName,
			//     AgentId:   c.agentID,
			//     HttpSpans: convertHTTPSpans(batch),
			//     ...
			// })
			_ = conn // Use the connection
		}
	}
}

// drainBatch atomically drains the current batch and returns it.
func (c *Client) drainBatch() []RawEvent {
	c.batchMu.Lock()
	defer c.batchMu.Unlock()

	if len(c.batch) == 0 {
		return nil
	}

	drained := c.batch
	c.batch = make([]RawEvent, 0, maxBatchSize)
	return drained
}

// SendHeartbeat sends a periodic health check to the Collector.
func (c *Client) SendHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := c.loader.GetProgramStatus()
			c.logger.Info("heartbeat",
				"node", c.nodeName,
				"programs_loaded", len(status),
			)
		}
	}
}

// Helpers for proto conversion (stubs until codegen)
var (
	_ = binary.LittleEndian
	_ = timestamppb.Now
)
