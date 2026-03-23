package escalation

import "fmt"

// OwnershipRouter resolves who gets paged for an incident.
// Priority order from spec §5.3.1:
//  1. Service annotation: kernelview.io/owner
//  2. Namespace annotation: kernelview.io/team → PagerDuty on-call lookup
//  3. P0 or >3 affected services → platform team on-call
//  4. Fallback: global platform on-call (never silently drop)
type OwnershipRouter struct {
	// K8s metadata lookups (injected)
	getServiceAnnotation   func(service, namespace, key string) string
	getNamespaceAnnotation func(namespace, key string) string

	// PagerDuty integration
	pagerdutyLookup func(teamName string) string

	// Fallback
	platformOnCall string
}

// NewOwnershipRouter creates a new ownership router.
func NewOwnershipRouter(platformOnCall string) *OwnershipRouter {
	return &OwnershipRouter{
		platformOnCall: platformOnCall,
	}
}

// SetK8sLookups sets the Kubernetes annotation lookup functions.
func (r *OwnershipRouter) SetK8sLookups(
	svcLookup func(service, namespace, key string) string,
	nsLookup func(namespace, key string) string,
) {
	r.getServiceAnnotation = svcLookup
	r.getNamespaceAnnotation = nsLookup
}

// SetPagerDutyLookup sets the PagerDuty on-call lookup function.
func (r *OwnershipRouter) SetPagerDutyLookup(lookup func(teamName string) string) {
	r.pagerdutyLookup = lookup
}

// ResolveOwner returns who should be paged for an incident.
// IMPORTANT from spec: Never silently drop an incident because ownership
// resolution failed. An unrouted P1 is worse than a misrouted P1.
func (r *OwnershipRouter) ResolveOwner(incident *ManagedIncident) (owner string, source string) {
	// Rule 1: Service-level annotation
	if r.getServiceAnnotation != nil {
		owner := r.getServiceAnnotation(incident.ServiceName, incident.Namespace, "kernelview.io/owner")
		if owner != "" {
			return owner, "service_annotation"
		}
	}

	// Rule 2: Namespace-level annotation → PagerDuty team lookup
	if r.getNamespaceAnnotation != nil {
		teamName := r.getNamespaceAnnotation(incident.Namespace, "kernelview.io/team")
		if teamName != "" && r.pagerdutyLookup != nil {
			onCall := r.pagerdutyLookup(teamName)
			if onCall != "" {
				return onCall, fmt.Sprintf("pagerduty_oncall(%s)", teamName)
			}
		}
	}

	// Rule 3: P0 or multi-service → platform team regardless
	if incident.Severity == SevP0 {
		return r.platformOnCall, "platform_oncall_p0"
	}

	// Rule 4: Fallback — global platform on-call
	// "Never silently drop an incident because ownership resolution failed"
	return r.platformOnCall, "platform_oncall_fallback"
}
