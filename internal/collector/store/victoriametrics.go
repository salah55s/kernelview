// Package store provides storage backends for the Collector:
// VictoriaMetrics for time-series metrics and BadgerDB for raw event traces.
package store

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// VictoriaMetrics writes aggregated metrics via Prometheus remote_write.
type VictoriaMetrics struct {
	mu             sync.Mutex
	remoteWriteURL string
	client         *http.Client
	writeBuffer    []byte
	bufferMaxSize  int
	logger         *slog.Logger

	// Prometheus metrics registered by this component
	httpRequestsTotal    *prometheus.CounterVec
	httpDurationHist     *prometheus.HistogramVec
	tcpActiveConnections *prometheus.GaugeVec
	podSyscallRate       *prometheus.GaugeVec
	ebpfEventsPerSecond  *prometheus.GaugeVec
}

// NewVictoriaMetrics creates a VictoriaMetrics writer.
func NewVictoriaMetrics(remoteWriteURL string, bufferMaxSize int, logger *slog.Logger) *VictoriaMetrics {
	vm := &VictoriaMetrics{
		remoteWriteURL: remoteWriteURL,
		client:         &http.Client{Timeout: 10 * time.Second},
		bufferMaxSize:  bufferMaxSize,
		logger:         logger,
	}

	// Define metrics as specified in PRD §5.3.1
	vm.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests observed",
		},
		[]string{"service", "namespace", "method", "status_code", "path"},
	)

	vm.httpDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "namespace"},
	)

	vm.tcpActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tcp_active_connections",
			Help: "Active TCP connections between services",
		},
		[]string{"src_service", "dst_service", "namespace"},
	)

	vm.podSyscallRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pod_syscall_rate",
			Help: "Syscalls per second per pod",
		},
		[]string{"pod", "namespace", "node"},
	)

	vm.ebpfEventsPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_ebpf_events_per_second",
			Help: "Internal: eBPF events received per second per node",
		},
		[]string{"node"},
	)

	return vm
}

// RegisterMetrics registers all Prometheus metrics.
func (vm *VictoriaMetrics) RegisterMetrics(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		vm.httpRequestsTotal,
		vm.httpDurationHist,
		vm.tcpActiveConnections,
		vm.podSyscallRate,
		vm.ebpfEventsPerSecond,
	}
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return fmt.Errorf("registering metric: %w", err)
		}
	}
	return nil
}

// RecordHTTPRequest increments HTTP request counters and histogram.
func (vm *VictoriaMetrics) RecordHTTPRequest(service, namespace, method string, statusCode int, path string, durationSec float64) {
	vm.httpRequestsTotal.WithLabelValues(service, namespace, method, fmt.Sprintf("%d", statusCode), path).Inc()
	vm.httpDurationHist.WithLabelValues(service, namespace).Observe(durationSec)
}

// RecordSyscallRate updates the syscall rate gauge.
func (vm *VictoriaMetrics) RecordSyscallRate(pod, namespace, node string, rate float64) {
	vm.podSyscallRate.WithLabelValues(pod, namespace, node).Set(rate)
}

// Close cleans up resources.
func (vm *VictoriaMetrics) Close() error {
	return nil
}
