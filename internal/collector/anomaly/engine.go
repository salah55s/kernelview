// Package anomaly provides statistical anomaly detection for KernelView.
// It implements latency, error rate, and noisy neighbor detection using
// simple statistical methods (no ML models at v1).
package anomaly

import (
	"log/slog"
	"math"
	"sync"
	"time"
)

// AnomalyType identifies the kind of anomaly detected.
type AnomalyType string

const (
	AnomalyLatencySpike  AnomalyType = "latency_spike"
	AnomalyErrorRate     AnomalyType = "error_rate_spike"
	AnomalyNoisyNeighbor AnomalyType = "noisy_neighbor"
)

// DetectedAnomaly is emitted when the engine detects an anomaly.
type DetectedAnomaly struct {
	Type        AnomalyType
	Service     string
	Namespace   string
	Pod         string
	Node        string
	Severity    string // "warning" or "critical"
	Description string
	DetectedAt  time.Time
	Value       float64 // The value that triggered the anomaly
	Baseline    float64 // The baseline value
	Threshold   float64 // The threshold that was exceeded
}

// AnomalyCallback is called when an anomaly is detected.
type AnomalyCallback func(anomaly DetectedAnomaly)

// Engine runs anomaly detection on incoming metrics.
type Engine struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Callbacks
	onAnomaly AnomalyCallback

	// Configuration
	latencyBaselineWindow  time.Duration
	latencyAnomalyFactor   float64
	errorRateBaselineWindow time.Duration
	errorRateThresholdPP   float64
	errorRateAbsoluteMax   float64
	noisyNeighborSigma     float64
	noisyNeighborDuration  time.Duration

	// State
	latencyBaselines map[string]*RollingPercentile // service → rolling p99
	errorBaselines   map[string]*RollingAverage    // service → rolling error rate
	syscallBaselines map[string]*NodeSyscallStats  // node → per-pod syscall stats

	// Anomaly tracking (for consecutive-window requirement)
	latencyAnomalyWindows map[string]int  // service → consecutive anomaly count
	noisyNeighborWindows  map[string]int  // pod → consecutive seconds above threshold

	// Learning state
	serviceFirstSeen map[string]time.Time
}

// NewEngine creates a new anomaly detection engine.
func NewEngine(logger *slog.Logger, callback AnomalyCallback) *Engine {
	return &Engine{
		logger:    logger,
		onAnomaly: callback,

		// Defaults from PRD
		latencyBaselineWindow:  1 * time.Hour,
		latencyAnomalyFactor:   3.0,
		errorRateBaselineWindow: 24 * time.Hour,
		errorRateThresholdPP:   15.0,
		errorRateAbsoluteMax:   20.0,
		noisyNeighborSigma:     3.0,
		noisyNeighborDuration:  30 * time.Second,

		latencyBaselines:      make(map[string]*RollingPercentile),
		errorBaselines:        make(map[string]*RollingAverage),
		syscallBaselines:      make(map[string]*NodeSyscallStats),
		latencyAnomalyWindows: make(map[string]int),
		noisyNeighborWindows:  make(map[string]int),
		serviceFirstSeen:      make(map[string]time.Time),
	}
}

// RecordLatency records a request latency observation for a service.
func (e *Engine) RecordLatency(service, namespace string, latencyMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := namespace + "/" + service

	// Track first-seen time for LEARNING suppression
	if _, ok := e.serviceFirstSeen[key]; !ok {
		e.serviceFirstSeen[key] = time.Now()
		e.logger.Info("new service discovered (LEARNING state)", "service", key)
	}

	// Initialize baseline if needed
	if _, ok := e.latencyBaselines[key]; !ok {
		e.latencyBaselines[key] = NewRollingPercentile(e.latencyBaselineWindow, 60*time.Second)
	}

	e.latencyBaselines[key].Add(latencyMs)
}

// CheckLatencyAnomaly evaluates the current 5-minute p99 against the baseline.
func (e *Engine) CheckLatencyAnomaly(service, namespace string, currentP99 float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := namespace + "/" + service

	// Suppress during LEARNING period
	if firstSeen, ok := e.serviceFirstSeen[key]; ok {
		if time.Since(firstSeen) < e.latencyBaselineWindow {
			return // Still warming up
		}
	}

	baseline, ok := e.latencyBaselines[key]
	if !ok {
		return
	}

	baselineP99 := baseline.P99()
	if baselineP99 <= 0 {
		return
	}

	threshold := baselineP99 * e.latencyAnomalyFactor

	if currentP99 > threshold {
		e.latencyAnomalyWindows[key]++
		// Require 2 consecutive windows to avoid single-spike false positives
		if e.latencyAnomalyWindows[key] >= 2 {
			e.onAnomaly(DetectedAnomaly{
				Type:        AnomalyLatencySpike,
				Service:     service,
				Namespace:   namespace,
				Severity:    "critical",
				Description: "p99 latency has exceeded 3x the 1-hour baseline for 2+ consecutive measurement windows",
				DetectedAt:  time.Now(),
				Value:       currentP99,
				Baseline:    baselineP99,
				Threshold:   threshold,
			})
		}
	} else {
		e.latencyAnomalyWindows[key] = 0 // Reset counter
	}
}

// RecordErrorRate records an error rate observation for a service.
func (e *Engine) RecordErrorRate(service, namespace string, errorRatePercent float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := namespace + "/" + service

	if _, ok := e.errorBaselines[key]; !ok {
		e.errorBaselines[key] = NewRollingAverage(e.errorRateBaselineWindow)
	}

	e.errorBaselines[key].Add(errorRatePercent)

	baseline := e.errorBaselines[key].Average()

	// Alert on baseline + 15pp OR absolute 20%
	if errorRatePercent > baseline+e.errorRateThresholdPP || errorRatePercent > e.errorRateAbsoluteMax {
		e.onAnomaly(DetectedAnomaly{
			Type:        AnomalyErrorRate,
			Service:     service,
			Namespace:   namespace,
			Severity:    "critical",
			Description: "Error rate exceeds baseline + 15pp or absolute 20% threshold",
			DetectedAt:  time.Now(),
			Value:       errorRatePercent,
			Baseline:    baseline,
			Threshold:   math.Max(baseline+e.errorRateThresholdPP, e.errorRateAbsoluteMax),
		})
	}
}

// RecordSyscallRate records a pod's syscall rate on a given node.
func (e *Engine) RecordSyscallRate(node, pod, namespace string, rate float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.syscallBaselines[node]; !ok {
		e.syscallBaselines[node] = NewNodeSyscallStats()
	}

	stats := e.syscallBaselines[node]
	stats.Record(pod, rate)

	mean, stddev := stats.MeanAndStdDev()
	threshold := mean + (e.noisyNeighborSigma * stddev)

	podKey := node + "/" + pod
	if rate > threshold {
		e.noisyNeighborWindows[podKey]++
		// 30 consecutive seconds (6 observations at 5-second intervals)
		if e.noisyNeighborWindows[podKey] >= 6 {
			e.onAnomaly(DetectedAnomaly{
				Type:        AnomalyNoisyNeighbor,
				Service:     pod,
				Namespace:   namespace,
				Node:        node,
				Severity:    "warning",
				Description: "Syscall rate exceeds mean + 3σ for 30+ consecutive seconds",
				DetectedAt:  time.Now(),
				Value:       rate,
				Baseline:    mean,
				Threshold:   threshold,
			})
		}
	} else {
		e.noisyNeighborWindows[podKey] = 0
	}
}

// ============================================================
// Rolling Statistics Helpers
// ============================================================

// RollingPercentile tracks percentiles over a time window.
type RollingPercentile struct {
	window   time.Duration
	interval time.Duration
	buckets  [][]float64
	current  int
	lastTick time.Time
}

// NewRollingPercentile creates a rolling percentile tracker.
func NewRollingPercentile(window, interval time.Duration) *RollingPercentile {
	numBuckets := int(window / interval)
	if numBuckets < 1 {
		numBuckets = 1
	}
	buckets := make([][]float64, numBuckets)
	for i := range buckets {
		buckets[i] = make([]float64, 0, 1024)
	}
	return &RollingPercentile{
		window:   window,
		interval: interval,
		buckets:  buckets,
		lastTick: time.Now(),
	}
}

// Add records a value.
func (rp *RollingPercentile) Add(value float64) {
	rp.advance()
	rp.buckets[rp.current] = append(rp.buckets[rp.current], value)
}

// P99 returns the p99 value across all buckets.
func (rp *RollingPercentile) P99() float64 {
	rp.advance()
	var all []float64
	for _, bucket := range rp.buckets {
		all = append(all, bucket...)
	}
	if len(all) == 0 {
		return 0
	}

	// Simple p99: sort and take 99th percentile
	// Using a simple insertion sort for small datasets
	for i := 1; i < len(all); i++ {
		key := all[i]
		j := i - 1
		for j >= 0 && all[j] > key {
			all[j+1] = all[j]
			j--
		}
		all[j+1] = key
	}

	idx := int(float64(len(all)) * 0.99)
	if idx >= len(all) {
		idx = len(all) - 1
	}
	return all[idx]
}

func (rp *RollingPercentile) advance() {
	now := time.Now()
	elapsed := now.Sub(rp.lastTick)
	ticks := int(elapsed / rp.interval)

	for i := 0; i < ticks && i < len(rp.buckets); i++ {
		rp.current = (rp.current + 1) % len(rp.buckets)
		rp.buckets[rp.current] = rp.buckets[rp.current][:0]
	}

	if ticks > 0 {
		rp.lastTick = now
	}
}

// RollingAverage tracks the average value over a time window.
type RollingAverage struct {
	window time.Duration
	values []timestampedValue
}

type timestampedValue struct {
	time  time.Time
	value float64
}

// NewRollingAverage creates a rolling average tracker.
func NewRollingAverage(window time.Duration) *RollingAverage {
	return &RollingAverage{
		window: window,
		values: make([]timestampedValue, 0, 1024),
	}
}

// Add records a value.
func (ra *RollingAverage) Add(value float64) {
	ra.prune()
	ra.values = append(ra.values, timestampedValue{time: time.Now(), value: value})
}

// Average returns the average value over the window.
func (ra *RollingAverage) Average() float64 {
	ra.prune()
	if len(ra.values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range ra.values {
		sum += v.value
	}
	return sum / float64(len(ra.values))
}

func (ra *RollingAverage) prune() {
	cutoff := time.Now().Add(-ra.window)
	i := 0
	for i < len(ra.values) && ra.values[i].time.Before(cutoff) {
		i++
	}
	if i > 0 {
		ra.values = ra.values[i:]
	}
}

// NodeSyscallStats tracks per-pod syscall rates for a single node.
type NodeSyscallStats struct {
	rates map[string]float64 // pod → most recent rate
}

// NewNodeSyscallStats creates per-node syscall tracking.
func NewNodeSyscallStats() *NodeSyscallStats {
	return &NodeSyscallStats{
		rates: make(map[string]float64),
	}
}

// Record updates a pod's syscall rate.
func (n *NodeSyscallStats) Record(pod string, rate float64) {
	n.rates[pod] = rate
}

// MeanAndStdDev computes the mean and standard deviation across all pods.
func (n *NodeSyscallStats) MeanAndStdDev() (float64, float64) {
	if len(n.rates) == 0 {
		return 0, 0
	}

	var sum float64
	for _, r := range n.rates {
		sum += r
	}
	mean := sum / float64(len(n.rates))

	var variance float64
	for _, r := range n.rates {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(n.rates))

	return mean, math.Sqrt(variance)
}
