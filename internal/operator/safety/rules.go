// Package safety implements the non-negotiable safety rules for the
// Remediation Operator. These rules CANNOT be bypassed regardless of
// what the AI Correlator recommends.
package safety

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CheckResult is the result of a single safety check.
type CheckResult struct {
	RuleName string
	Passed   bool
	Reason   string
}

// ActionContext contains all information needed to evaluate safety rules.
type ActionContext struct {
	TargetPod         string
	TargetNamespace   string
	TargetDeployment  string
	ActionType        string // THROTTLE_CPU, RESTART_POD, ISOLATE_POD, etc.
	NewCPULimit       int32  // millicores (for THROTTLE_CPU)
	DeclaredCPULimit  int32  // millicores (current declared limit)
	ReplicaCount      int32
	HasPDB            bool   // PodDisruptionBudget exists
	PDBMinAvailable   int32
	IsStatefulSet     bool
	HasHPA            bool
	CordonnedNodes    int
	TotalNodes        int
}

// Engine evaluates all safety rules for a proposed remediation action.
type Engine struct {
	mu                  sync.RWMutex
	logger              *slog.Logger
	protectedNamespaces map[string]bool
	maxActionsPerHour   int
	minCPUPercent       float64
	maxCordonnedPercent float64

	// Action history for rate limiting
	actionHistory map[string][]time.Time // pod → timestamps of recent actions
}

// NewEngine creates a safety rule engine.
func NewEngine(logger *slog.Logger, protectedNamespaces []string, maxActionsPerHour int, minCPUPercent, maxCordonnedPercent float64) *Engine {
	ns := make(map[string]bool)
	for _, n := range protectedNamespaces {
		ns[n] = true
	}

	return &Engine{
		logger:              logger,
		protectedNamespaces: ns,
		maxActionsPerHour:   maxActionsPerHour,
		minCPUPercent:       minCPUPercent,
		maxCordonnedPercent: maxCordonnedPercent,
		actionHistory:       make(map[string][]time.Time),
	}
}

// Evaluate runs ALL safety rules and returns results.
// Returns false if ANY rule fails.
func (e *Engine) Evaluate(ctx context.Context, action ActionContext) (bool, []CheckResult) {
	results := []CheckResult{
		e.checkProtectedNamespace(action),
		e.checkSingleReplicaRestart(action),
		e.checkMinCPUThrottle(action),
		e.checkActionRateLimit(action),
		e.checkIsolateRequiresApproval(action),
		e.checkPDBCompliance(action),
		e.checkStatefulSetRestart(action),
		e.checkClusterUpgradeState(action),
	}

	allPassed := true
	for _, r := range results {
		if !r.Passed {
			allPassed = false
			e.logger.Warn("safety rule FAILED",
				"rule", r.RuleName,
				"reason", r.Reason,
				"pod", action.TargetPod,
				"namespace", action.TargetNamespace,
				"action", action.ActionType,
			)
		}
	}

	return allPassed, results
}

// RecordAction records that an action was taken on a pod (for rate limiting).
func (e *Engine) RecordAction(pod string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.actionHistory[pod] = append(e.actionHistory[pod], time.Now())

	// Prune old entries
	cutoff := time.Now().Add(-1 * time.Hour)
	history := e.actionHistory[pod]
	i := 0
	for i < len(history) && history[i].Before(cutoff) {
		i++
	}
	e.actionHistory[pod] = history[i:]
}

// ============================================================
// Rule 1: Never action on protected namespaces (kube-system)
// ============================================================

func (e *Engine) checkProtectedNamespace(action ActionContext) CheckResult {
	if e.protectedNamespaces[action.TargetNamespace] {
		return CheckResult{
			RuleName: "protected_namespace",
			Passed:   false,
			Reason:   fmt.Sprintf("namespace %q is protected — automated remediation is not allowed", action.TargetNamespace),
		}
	}
	return CheckResult{RuleName: "protected_namespace", Passed: true, Reason: "namespace is not protected"}
}

// ============================================================
// Rule 2: Never restart single-replica pods without PDB
// ============================================================

func (e *Engine) checkSingleReplicaRestart(action ActionContext) CheckResult {
	if action.ActionType != "RESTART_POD" {
		return CheckResult{RuleName: "single_replica_restart", Passed: true, Reason: "not a restart action"}
	}
	if action.ReplicaCount <= 1 && !action.HasPDB {
		return CheckResult{
			RuleName: "single_replica_restart",
			Passed:   false,
			Reason:   "cannot restart a pod with only 1 replica and no PodDisruptionBudget — this would cause complete downtime",
		}
	}
	return CheckResult{RuleName: "single_replica_restart", Passed: true, Reason: "sufficient replicas or PDB present"}
}

// ============================================================
// Rule 3: Never throttle below 10% of declared CPU
// ============================================================

func (e *Engine) checkMinCPUThrottle(action ActionContext) CheckResult {
	if action.ActionType != "THROTTLE_CPU" {
		return CheckResult{RuleName: "min_cpu_throttle", Passed: true, Reason: "not a throttle action"}
	}
	if action.DeclaredCPULimit <= 0 {
		return CheckResult{RuleName: "min_cpu_throttle", Passed: true, Reason: "no declared CPU limit"}
	}

	minLimit := float64(action.DeclaredCPULimit) * (e.minCPUPercent / 100.0)
	if float64(action.NewCPULimit) < minLimit {
		return CheckResult{
			RuleName: "min_cpu_throttle",
			Passed:   false,
			Reason:   fmt.Sprintf("new CPU limit %dm is below %.0f%% of declared limit %dm", action.NewCPULimit, e.minCPUPercent, action.DeclaredCPULimit),
		}
	}
	return CheckResult{RuleName: "min_cpu_throttle", Passed: true, Reason: "CPU limit is above minimum threshold"}
}

// ============================================================
// Rule 4: Max 3 automated actions per pod per hour
// ============================================================

func (e *Engine) checkActionRateLimit(action ActionContext) CheckResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history := e.actionHistory[action.TargetPod]
	cutoff := time.Now().Add(-1 * time.Hour)

	count := 0
	for _, t := range history {
		if t.After(cutoff) {
			count++
		}
	}

	if count >= e.maxActionsPerHour {
		return CheckResult{
			RuleName: "action_rate_limit",
			Passed:   false,
			Reason:   fmt.Sprintf("%d actions taken on this pod in the last hour (max %d) — the root cause is likely not being addressed", count, e.maxActionsPerHour),
		}
	}
	return CheckResult{RuleName: "action_rate_limit", Passed: true, Reason: fmt.Sprintf("%d/%d actions in the last hour", count, e.maxActionsPerHour)}
}

// ============================================================
// Rule 5: ISOLATE always requires human approval
// ============================================================

func (e *Engine) checkIsolateRequiresApproval(action ActionContext) CheckResult {
	if action.ActionType == "ISOLATE_POD" {
		return CheckResult{
			RuleName: "isolate_requires_approval",
			Passed:   false,
			Reason:   "ISOLATE actions always require human approval — network isolation can cause cascading failures",
		}
	}
	return CheckResult{RuleName: "isolate_requires_approval", Passed: true, Reason: "not an isolate action"}
}

// ============================================================
// Rule 6: Respect PodDisruptionBudgets
// ============================================================

func (e *Engine) checkPDBCompliance(action ActionContext) CheckResult {
	if action.ActionType != "RESTART_POD" && action.ActionType != "SCALE_DOWN" {
		return CheckResult{RuleName: "pdb_compliance", Passed: true, Reason: "action type does not affect availability"}
	}
	if action.HasPDB && action.ReplicaCount <= action.PDBMinAvailable {
		return CheckResult{
			RuleName: "pdb_compliance",
			Passed:   false,
			Reason:   fmt.Sprintf("action would violate PodDisruptionBudget — current replicas %d, PDB minAvailable %d", action.ReplicaCount, action.PDBMinAvailable),
		}
	}
	return CheckResult{RuleName: "pdb_compliance", Passed: true, Reason: "PDB constraints satisfied"}
}

// ============================================================
// Additional Rule: StatefulSet pods require human approval
// ============================================================

func (e *Engine) checkStatefulSetRestart(action ActionContext) CheckResult {
	if action.ActionType == "RESTART_POD" && action.IsStatefulSet {
		return CheckResult{
			RuleName: "statefulset_restart",
			Passed:   false,
			Reason:   "StatefulSet pods have different restart semantics (ordered restart, PV rebinding) — human approval required",
		}
	}
	return CheckResult{RuleName: "statefulset_restart", Passed: true, Reason: "not a StatefulSet restart"}
}

// ============================================================
// Additional Rule: Check cluster upgrade state
// ============================================================

func (e *Engine) checkClusterUpgradeState(action ActionContext) CheckResult {
	if action.TotalNodes == 0 {
		return CheckResult{RuleName: "cluster_upgrade_state", Passed: true, Reason: "no node info available"}
	}

	cordonnedPercent := float64(action.CordonnedNodes) / float64(action.TotalNodes) * 100
	if cordonnedPercent > e.maxCordonnedPercent {
		return CheckResult{
			RuleName: "cluster_upgrade_state",
			Passed:   false,
			Reason:   fmt.Sprintf("%.0f%% of nodes are cordoned (max %.0f%%) — cluster may be upgrading, aborting remediation", cordonnedPercent, e.maxCordonnedPercent),
		}
	}
	return CheckResult{RuleName: "cluster_upgrade_state", Passed: true, Reason: fmt.Sprintf("%.0f%% nodes cordoned (within limit)", cordonnedPercent)}
}
