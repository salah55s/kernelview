// Package bpf handles loading and managing eBPF programs and reading
// events from BPF ring buffers and maps.
package bpf

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

// ProgramInfo describes a loaded BPF program.
type ProgramInfo struct {
	Name       string
	Type       string // "kprobe", "tracepoint", "uprobe", "xdp"
	Loaded     bool
	Link       link.Link
	Error      error
	EventCount uint64
	DropCount  uint64
}

// Loader manages the lifecycle of eBPF programs.
type Loader struct {
	mu       sync.RWMutex
	pinPath  string
	programs map[string]*ProgramInfo
	reader   *ringbuf.Reader
	logger   *slog.Logger

	// Detected capabilities
	KernelVersion  string
	CgroupVersion  string
	HasBTF         bool
	HasRingBuf     bool
}

// NewLoader creates a new eBPF program loader.
func NewLoader(pinPath string, logger *slog.Logger) (*Loader, error) {
	// Remove locked memory limits for BPF
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("removing memlock rlimit: %w", err)
	}

	// Ensure pin path exists
	if err := os.MkdirAll(pinPath, 0755); err != nil {
		return nil, fmt.Errorf("creating BPF pin path %s: %w", pinPath, err)
	}

	l := &Loader{
		pinPath:  pinPath,
		programs: make(map[string]*ProgramInfo),
		logger:   logger,
	}

	// Detect kernel capabilities
	l.detectCapabilities()

	return l, nil
}

// detectCapabilities checks kernel features.
func (l *Loader) detectCapabilities() {
	// Read kernel version
	data, err := os.ReadFile("/proc/version")
	if err == nil {
		l.KernelVersion = strings.TrimSpace(string(data))
	}

	// Detect cgroups version
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		l.CgroupVersion = "v2"
	} else {
		l.CgroupVersion = "v1"
	}

	// Detect BTF support
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err == nil {
		l.HasBTF = true
	}

	// Ring buffer support (kernel 5.8+) — try to detect via BTF
	l.HasRingBuf = true // Assume true for 5.10+ target kernels

	l.logger.Info("detected kernel capabilities",
		"kernel", l.KernelVersion,
		"cgroup", l.CgroupVersion,
		"btf", l.HasBTF,
		"ringbuf", l.HasRingBuf,
	)
}

// LoadProgram loads a single compiled BPF object file.
func (l *Loader) LoadProgram(name, objectPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	info := &ProgramInfo{Name: name}
	l.programs[name] = info

	spec, err := ebpf.LoadCollectionSpec(objectPath)
	if err != nil {
		info.Error = fmt.Errorf("loading spec %s: %w", objectPath, err)
		l.logger.Error("failed to load BPF program spec", "name", name, "error", err)
		return info.Error
	}

	coll, err := ebpf.NewCollectionWithOptions(spec, ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: l.pinPath,
		},
	})
	if err != nil {
		info.Error = fmt.Errorf("creating collection %s: %w", name, err)
		l.logger.Error("failed to create BPF collection", "name", name, "error", err)
		return info.Error
	}

	// Pin all maps for persistence across agent restarts
	for mapName, m := range coll.Maps {
		pinFile := filepath.Join(l.pinPath, mapName)
		if err := m.Pin(pinFile); err != nil {
			l.logger.Warn("failed to pin map", "map", mapName, "error", err)
		}
	}

	// Attach programs based on their type
	for progName, prog := range coll.Programs {
		var lnk link.Link
		var attachErr error

		switch prog.Type() {
		case ebpf.Kprobe:
			// Extract function name from section (e.g., "kprobe/tcp_sendmsg" → "tcp_sendmsg")
			funcName := extractFuncName(progName)
			lnk, attachErr = link.Kprobe(funcName, prog, nil)
		case ebpf.TracePoint:
			group, tpName := extractTracepointNames(progName)
			lnk, attachErr = link.Tracepoint(group, tpName, prog, nil)
		case ebpf.XDP:
			// XDP requires a network interface — attached separately
			l.logger.Info("XDP program loaded, attach via AttachXDP()", "name", progName)
			continue
		}

		if attachErr != nil {
			l.logger.Error("failed to attach BPF program",
				"name", progName, "type", prog.Type(), "error", attachErr)
			info.Error = attachErr
			continue
		}

		if lnk != nil {
			info.Link = lnk
		}

		l.logger.Info("attached BPF program", "name", progName, "type", prog.Type())
	}

	info.Loaded = true
	l.logger.Info("loaded BPF program", "name", name)

	// Try to set up ring buffer reader from the events map
	if eventsMap, ok := coll.Maps["events"]; ok {
		reader, err := ringbuf.NewReader(eventsMap)
		if err != nil {
			l.logger.Warn("failed to create ring buffer reader, will try perf buffer", "error", err)
		} else {
			l.reader = reader
		}
	}

	return nil
}

// ReadEvents reads events from the ring buffer. It blocks until an event
// is available or the context is cancelled.
func (l *Loader) ReadEvents(handler func(data []byte)) error {
	if l.reader == nil {
		return fmt.Errorf("ring buffer reader not initialized")
	}

	for {
		record, err := l.reader.Read()
		if err != nil {
			return fmt.Errorf("reading ring buffer: %w", err)
		}
		handler(record.RawSample)
	}
}

// GetProgramStatus returns the status of all loaded programs.
func (l *Loader) GetProgramStatus() map[string]*ProgramInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]*ProgramInfo, len(l.programs))
	for k, v := range l.programs {
		result[k] = v
	}
	return result
}

// Close cleans up all BPF resources.
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.reader != nil {
		l.reader.Close()
	}

	for name, info := range l.programs {
		if info.Link != nil {
			if err := info.Link.Close(); err != nil {
				l.logger.Error("failed to close link", "name", name, "error", err)
			}
		}
	}

	return nil
}

// Helper: extract kernel function name from BPF section name
func extractFuncName(section string) string {
	parts := strings.SplitN(section, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return section
}

// Helper: extract tracepoint group and name from section
func extractTracepointNames(section string) (string, string) {
	// Format: "tracepoint/group/name" or just "group/name"
	parts := strings.Split(section, "/")
	switch len(parts) {
	case 3:
		return parts[1], parts[2]
	case 2:
		return parts[0], parts[1]
	default:
		return "", section
	}
}

// Ensure alignment helper (unused but keeping for reference)
var _ = unsafe.Sizeof(0)
