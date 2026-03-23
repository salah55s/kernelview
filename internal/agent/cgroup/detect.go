// Package cgroup provides cgroup detection and throttling capabilities.
// Supports both cgroups v1 and v2.
package cgroup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Version represents the cgroup version in use.
type Version string

const (
	V1 Version = "v1"
	V2 Version = "v2"
)

// Detector detects and manages cgroup operations.
type Detector struct {
	version Version
	logger  *slog.Logger
}

// NewDetector creates a Detector and auto-detects the cgroup version.
func NewDetector(logger *slog.Logger) *Detector {
	d := &Detector{logger: logger}
	d.version = d.detectVersion()
	logger.Info("detected cgroup version", "version", d.version)
	return d
}

// Version returns the detected cgroup version.
func (d *Detector) Version() Version {
	return d.version
}

// detectVersion checks whether the system uses cgroups v1 or v2.
func (d *Detector) detectVersion() Version {
	// cgroups v2 has a unified hierarchy with cgroup.controllers
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return V2
	}
	return V1
}

// GetContainerCgroupPath resolves the cgroup path for a container
// given a host PID.
func (d *Detector) GetContainerCgroupPath(hostPID int) (string, error) {
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", hostPID)
	data, err := os.ReadFile(cgroupFile)
	if err != nil {
		return "", fmt.Errorf("reading cgroup for PID %d: %w", hostPID, err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		cgroupPath := parts[2]

		if d.version == V2 {
			// cgroups v2: hierarchy ID is 0
			if parts[0] == "0" {
				return d.buildCgroupPath(cgroupPath), nil
			}
		} else {
			// cgroups v1: look for the cpu controller
			if strings.Contains(parts[1], "cpu") {
				return d.buildCgroupPath(cgroupPath), nil
			}
		}
	}

	return "", fmt.Errorf("no cgroup path found for PID %d", hostPID)
}

// ExtractContainerID extracts the container ID from a cgroup path.
// Handles Docker, containerd, and CRI-O formats.
func (d *Detector) ExtractContainerID(cgroupPath string) string {
	// Docker: /docker/<container-id>
	// containerd: /system.slice/containerd.service/kubepods-.../<container-id>
	// CRI-O: /kubepods.slice/kubepods-...-<container-id>.scope

	parts := strings.Split(cgroupPath, "/")
	if len(parts) == 0 {
		return ""
	}

	last := parts[len(parts)-1]

	// Docker format
	if strings.HasPrefix(last, "docker-") {
		return strings.TrimSuffix(strings.TrimPrefix(last, "docker-"), ".scope")
	}

	// CRI-O format
	if strings.HasPrefix(last, "crio-") {
		return strings.TrimSuffix(strings.TrimPrefix(last, "crio-"), ".scope")
	}

	// containerd format — usually a 64-char hex ID
	if len(last) == 64 {
		return last
	}

	// Generic: try to extract from .scope suffix
	if strings.HasSuffix(last, ".scope") {
		trimmed := strings.TrimSuffix(last, ".scope")
		idx := strings.LastIndex(trimmed, "-")
		if idx >= 0 {
			return trimmed[idx+1:]
		}
	}

	return last
}

// ThrottleCPU writes a tighter CPU quota to a container's cgroup.
func (d *Detector) ThrottleCPU(cgroupPath string, quotaMs int) error {
	if d.version == V2 {
		return d.throttleCPUv2(cgroupPath, quotaMs)
	}
	return d.throttleCPUv1(cgroupPath, quotaMs)
}

// throttleCPUv2 writes to cpu.max (cgroups v2).
// Format: "quota period" in microseconds.
func (d *Detector) throttleCPUv2(cgroupPath string, quotaMs int) error {
	cpuMaxPath := filepath.Join(cgroupPath, "cpu.max")
	// quota in microseconds, 100ms period
	value := fmt.Sprintf("%d 100000", quotaMs*1000)

	d.logger.Info("throttling CPU (v2)", "path", cpuMaxPath, "value", value)
	return os.WriteFile(cpuMaxPath, []byte(value), 0644)
}

// throttleCPUv1 writes to cpu.cfs_quota_us (cgroups v1).
func (d *Detector) throttleCPUv1(cgroupPath string, quotaMs int) error {
	quotaPath := filepath.Join(cgroupPath, "cpu.cfs_quota_us")
	value := fmt.Sprintf("%d", quotaMs*1000) // Convert ms to μs

	d.logger.Info("throttling CPU (v1)", "path", quotaPath, "value", value)
	return os.WriteFile(quotaPath, []byte(value), 0644)
}

// ReadCPULimit reads the current CPU limit from the container's cgroup.
func (d *Detector) ReadCPULimit(cgroupPath string) (quotaUs int, periodUs int, err error) {
	if d.version == V2 {
		data, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.max"))
		if err != nil {
			return 0, 0, err
		}
		_, err = fmt.Sscanf(strings.TrimSpace(string(data)), "%d %d", &quotaUs, &periodUs)
		return quotaUs, periodUs, err
	}

	// v1
	quotaData, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.cfs_quota_us"))
	if err != nil {
		return 0, 0, err
	}
	fmt.Sscanf(strings.TrimSpace(string(quotaData)), "%d", &quotaUs)

	periodData, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.cfs_period_us"))
	if err != nil {
		return quotaUs, 100000, nil // default period
	}
	fmt.Sscanf(strings.TrimSpace(string(periodData)), "%d", &periodUs)

	return quotaUs, periodUs, nil
}

// ReadMemoryLimit reads the current memory limit.
func (d *Detector) ReadMemoryLimit(cgroupPath string) (int64, error) {
	var limitPath string
	if d.version == V2 {
		limitPath = filepath.Join(cgroupPath, "memory.max")
	} else {
		limitPath = filepath.Join(cgroupPath, "memory.limit_in_bytes")
	}

	data, err := os.ReadFile(limitPath)
	if err != nil {
		return 0, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "max" || trimmed == "-1" {
		return -1, nil // no limit
	}

	var limit int64
	fmt.Sscanf(trimmed, "%d", &limit)
	return limit, nil
}

func (d *Detector) buildCgroupPath(relative string) string {
	if d.version == V2 {
		return filepath.Join("/sys/fs/cgroup", relative)
	}
	return filepath.Join("/sys/fs/cgroup/cpu", relative)
}
