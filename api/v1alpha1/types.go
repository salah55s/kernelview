// Package v1alpha1 contains the API types for the KernelView Remediation CRD.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Trigger",type=string,JSONPath=`.spec.trigger`
// +kubebuilder:printcolumn:name="Action",type=string,JSONPath=`.spec.action`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetPod`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RemediationAction is the Schema for the remediationactions API.
type RemediationAction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemediationActionSpec   `json:"spec,omitempty"`
	Status RemediationActionStatus `json:"status,omitempty"`
}

// RemediationActionSpec defines the desired state of RemediationAction.
type RemediationActionSpec struct {
	// Trigger describes what anomaly triggered this action.
	// +kubebuilder:validation:Enum=NOISY_NEIGHBOR;OOM_KILL;LATENCY_SPIKE;ERROR_RATE;CONNECTION_FAILURE
	Trigger string `json:"trigger"`

	// TargetPod is the name of the pod to remediate.
	TargetPod string `json:"targetPod"`

	// TargetNamespace is the namespace of the target pod.
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// Action is the type of remediation to perform.
	// +kubebuilder:validation:Enum=THROTTLE_CPU;RESTART_POD;ISOLATE_POD;ADJUST_MEMORY;SCALE_DOWN
	Action string `json:"action"`

	// Parameters contains action-specific configuration.
	Parameters ActionParameters `json:"parameters,omitempty"`

	// ApprovalRequired indicates if human approval is needed.
	ApprovalRequired bool `json:"approvalRequired,omitempty"`

	// CorrelationID links this action to the AI Correlator's analysis.
	CorrelationID string `json:"correlationID,omitempty"`

	// DryRun if true, logs the action but doesn't execute it.
	DryRun bool `json:"dryRun,omitempty"`
}

// ActionParameters holds action-specific configuration.
type ActionParameters struct {
	// NewCPULimitMillicores is the new CPU limit for THROTTLE_CPU actions.
	NewCPULimitMillicores int32 `json:"newCPULimitMillicores,omitempty"`

	// NewMemoryLimitBytes is the new memory limit for ADJUST_MEMORY actions.
	NewMemoryLimitBytes int64 `json:"newMemoryLimitBytes,omitempty"`

	// NewReplicaCount is the new replica count for SCALE_DOWN actions.
	NewReplicaCount int32 `json:"newReplicaCount,omitempty"`

	// DurationSeconds is how long the remediation lasts before auto-revert.
	// 0 means permanent (no auto-revert).
	DurationSeconds int32 `json:"durationSeconds,omitempty"`
}

// RemediationActionStatus defines the observed state of RemediationAction.
type RemediationActionStatus struct {
	// Phase is the current lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Approved;Executing;Completed;Reverted;Failed;Rejected
	Phase string `json:"phase,omitempty"`

	// ExecutedAt is when the action was executed.
	ExecutedAt *metav1.Time `json:"executedAt,omitempty"`

	// RevertedAt is when the action was reverted.
	RevertedAt *metav1.Time `json:"revertedAt,omitempty"`

	// Message is a human-readable description of the current state.
	Message string `json:"message,omitempty"`

	// SafetyCheckResults contains the results of all safety rule evaluations.
	SafetyCheckResults []SafetyCheckResult `json:"safetyCheckResults,omitempty"`

	// OriginalState stores the pre-remediation state for revert.
	OriginalState *OriginalState `json:"originalState,omitempty"`
}

// SafetyCheckResult is the result of a single safety rule evaluation.
type SafetyCheckResult struct {
	RuleName string `json:"ruleName"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason,omitempty"`
}

// OriginalState stores pre-remediation values for reverting.
type OriginalState struct {
	CPULimitMillicores int32 `json:"cpuLimitMillicores,omitempty"`
	MemoryLimitBytes   int64 `json:"memoryLimitBytes,omitempty"`
	ReplicaCount       int32 `json:"replicaCount,omitempty"`
}

// +kubebuilder:object:root=true

// RemediationActionList contains a list of RemediationAction.
type RemediationActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemediationAction `json:"items"`
}
