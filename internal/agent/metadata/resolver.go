// Package metadata resolves container PIDs to Kubernetes pod metadata.
package metadata

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// PodInfo contains Kubernetes pod metadata resolved from a host PID.
type PodInfo struct {
	PodName      string
	Namespace    string
	ContainerID  string
	CgroupPath   string
	HostPID      int
	HostNetwork  bool
}

// Resolver maps host PIDs and container IDs to Kubernetes pods.
type Resolver struct {
	mu    sync.RWMutex
	cache map[int]*PodInfo       // hostPID → PodInfo
	cgMap map[string]*PodInfo    // containerID → PodInfo
	logger *slog.Logger
}

// NewResolver creates a new metadata resolver.
func NewResolver(logger *slog.Logger) *Resolver {
	return &Resolver{
		cache:  make(map[int]*PodInfo),
		cgMap:  make(map[string]*PodInfo),
		logger: logger,
	}
}

// Resolve looks up pod metadata for a given host PID.
// Returns nil if the PID does not belong to a known pod.
func (r *Resolver) Resolve(hostPID int) *PodInfo {
	r.mu.RLock()
	if info, ok := r.cache[hostPID]; ok {
		r.mu.RUnlock()
		return info
	}
	r.mu.RUnlock()

	// Try to resolve from /proc
	info, err := r.resolveFromProc(hostPID)
	if err != nil {
		r.logger.Debug("failed to resolve PID", "pid", hostPID, "error", err)
		return nil
	}

	r.mu.Lock()
	r.cache[hostPID] = info
	if info.ContainerID != "" {
		r.cgMap[info.ContainerID] = info
	}
	r.mu.Unlock()

	return info
}

// ResolveByContainerID looks up pod metadata by container ID.
func (r *Resolver) ResolveByContainerID(containerID string) *PodInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cgMap[containerID]
}

// UpdateFromKubernetes updates the cache with pod info from the kubelet.
// This is called by the Kubernetes informer when pods change.
func (r *Resolver) UpdateFromKubernetes(podName, namespace, containerID string, hostPID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := &PodInfo{
		PodName:     podName,
		Namespace:   namespace,
		ContainerID: containerID,
		HostPID:     hostPID,
	}

	r.cache[hostPID] = info
	r.cgMap[containerID] = info
}

// Invalidate removes a PID from the cache (e.g., after pod deletion).
func (r *Resolver) Invalidate(hostPID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info, ok := r.cache[hostPID]; ok {
		delete(r.cgMap, info.ContainerID)
	}
	delete(r.cache, hostPID)
}

// resolveFromProc reads /proc/{pid}/cgroup to determine the container ID.
func (r *Resolver) resolveFromProc(pid int) (*PodInfo, error) {
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := os.ReadFile(cgroupFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", cgroupFile, err)
	}

	info := &PodInfo{HostPID: pid}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		cgroupPath := parts[2]
		info.CgroupPath = cgroupPath

		// Extract container ID from cgroup path
		containerID := extractContainerIDFromCgroup(cgroupPath)
		if containerID != "" {
			info.ContainerID = containerID
		}

		// Extract pod name and namespace from cgroup path
		podName, namespace := extractPodInfoFromCgroup(cgroupPath)
		if podName != "" {
			info.PodName = podName
			info.Namespace = namespace
		}
	}

	// Check if using host network
	netNS := fmt.Sprintf("/proc/%d/ns/net", pid)
	hostNetNS := "/proc/1/ns/net"
	if isSameNamespace(netNS, hostNetNS) {
		info.HostNetwork = true
	}

	return info, nil
}

// extractContainerIDFromCgroup parses Docker, containerd, and CRI-O formats.
func extractContainerIDFromCgroup(path string) string {
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return ""
	}

	last := segments[len(segments)-1]

	// Docker: docker-<id>.scope
	if strings.HasPrefix(last, "docker-") {
		return strings.TrimSuffix(strings.TrimPrefix(last, "docker-"), ".scope")
	}
	// CRI-O: crio-<id>.scope
	if strings.HasPrefix(last, "crio-") {
		return strings.TrimSuffix(strings.TrimPrefix(last, "crio-"), ".scope")
	}
	// containerd: plain 64-char hex
	if len(last) == 64 {
		return last
	}

	return ""
}

// extractPodInfoFromCgroup tries to get pod name/namespace from cgroup path.
func extractPodInfoFromCgroup(path string) (string, string) {
	// Kubernetes cgroup paths contain the pod UID
	// e.g., /kubepods/burstable/pod<uid>/<container-id>
	// We can't get the pod name directly from cgroup — that comes from K8s API
	return "", ""
}

// isSameNamespace checks if two PID namespace files point to the same namespace.
func isSameNamespace(ns1, ns2 string) bool {
	fi1, err := os.Stat(ns1)
	if err != nil {
		return false
	}
	fi2, err := os.Stat(ns2)
	if err != nil {
		return false
	}
	return os.SameFile(fi1, fi2)
}
