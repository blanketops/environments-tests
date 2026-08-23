/*
Copyright 2025 The BlanketOps Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package networks

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=networks.blanketops.dev

// DomainData is the test/data representation of a Domain CR.
// Mirrors the Domain API type used in the networks controller.
// Used to construct test fixtures without importing the full API type.
//
// Architectural role:
//
//	Domain owns the cert/mapping chain for a host.
//	Route owns the workload binding (runtime, path, enabled).
//	They are peers in the same namespace — Domain is reconciled as
//	an owned child of Route (cascade delete).
//
// TLS strategy:
//
//	platform: *.dev.domain.co.za wildcard cert — DNS01 ClusterIssuer already issued.
//	          Controller emits DomainClaim + DomainMapping only. No cert work needed.
//	custom:   client-owned zone (e.g. client-a.co.za).
//	          Controller emits Issuer + DomainClaim + DomainMapping + Certificate.
//	          HTTP01 challenge via nginx solver in the tenant namespace.
//
// mTLS:
//
//	Inter-service mTLS via blanketops-proxy + blanketK.
//	Declared here — platform wires the sidecar identity automatically.
//
// APIVersion: networks.blanketops.dev/v1alpha1
type DomainData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec,omitempty"`
	Status DomainStatus `json:"status,omitempty"`
}

// DomainSpec is the desired state of a Domain.
// Mirrors networks.blanketops.dev/v1alpha1 DomainSpec.
type DomainSpec struct {
	// Contract is the intent declaration for this domain assignment.
	Contract DomainContract `json:"contract"`
}

// DomainContract is the core intent declaration for a Domain.
type DomainContract struct {
	// FQDN this domain assignment covers.
	// platform: must match *.dev.domain.co.za wildcard pattern.
	// custom:   client-owned zone, any FQDN.
	// e.g. api.dev.domain.co.za | app.client-a.co.za
	Host string `json:"host"`

	// Reference to the Route in this namespace this domain is assigned to.
	// Route owns the workload binding (runtime, path, enabled).
	// Domain owns the cert/mapping chain for this host.
	// Must be in the same namespace as this Domain CR.
	RouteRef RouteRef `json:"routeRef"`

	// TLS provisioning strategy for this domain.
	//   platform: wildcard cert already covers the host — no cert work needed.
	//             Controller emits DomainClaim + DomainMapping only.
	//   custom:   HTTP01 challenge — controller emits Issuer + DomainClaim +
	//             DomainMapping + Certificate in the tenant namespace.
	TLSStrategy TLSStrategy `json:"tlsStrategy"`

	// Inter-service mTLS configuration via blanketops-proxy + blanketK.
	// When enforced — platform wires the sidecar identity automatically.
	MTLS MTLS `json:"mtls"`

	// Certificate renewal window before expiry.
	// Only applies when tlsStrategy is custom.
	// Ignored for platform — wildcard renewal is platform-managed.
	// e.g. 720h (30 days)
	RenewBefore string `json:"renewBefore,omitempty"`
}

// RouteRef references the Route CR this domain is assigned to.
type RouteRef struct {
	// Name of the Route CR in the same namespace.
	// Must exist before the Domain is reconciled.
	Name string `json:"name"`
}

// MTLS is the inter-service mTLS configuration.
type MTLS struct {
	// When true — blanketops-proxy injects mTLS identity sidecar
	// and blanketK issues the workload certificate.
	// Platform wires the sidecar automatically on enforcement.
	Enforced bool `json:"enforced"`
}

// TLSStrategy is the TLS provisioning strategy for a Domain.
type TLSStrategy string

const (
	// TLSStrategyPlatform — platform wildcard cert covers the host.
	// DNS01 ClusterIssuer already issued the wildcard cert at install time.
	// Controller emits DomainClaim + DomainMapping only — no cert work needed.
	// Use for: *.dev.domain.co.za pattern hosts.
	TLSStrategyPlatform TLSStrategy = "platform"

	// TLSStrategyCustom — client-owned zone, HTTP01 challenge per tenant.
	// Controller emits Issuer + DomainClaim + DomainMapping + Certificate.
	// HTTP01 challenge routed via nginx solver in the tenant namespace.
	// Use for: client-owned FQDNs (e.g. app.client-a.co.za).
	TLSStrategyCustom TLSStrategy = "custom"
)

// DomainStatus is the observed state of a Domain.
type DomainStatus struct {
	// Current lifecycle phase.
	Phase DomainPhase `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// Whether the DomainClaim has been reconciled in knative-serving.
	DomainClaimReady bool `json:"domainClaimReady,omitempty"`

	// Whether the DomainMapping is active and serving traffic.
	DomainMappingReady bool `json:"domainMappingReady,omitempty"`

	// Whether the TLS certificate has been issued and is valid.
	// Always true for platform strategy — wildcard cert is pre-issued.
	// For custom — false until HTTP01 challenge completes.
	CertificateReady bool `json:"certificateReady,omitempty"`

	// When the controller last successfully reconciled this Domain.
	LastReconciledAt *metav1.Time `json:"lastReconciledAt,omitempty"`
}

// DomainPhase is the lifecycle phase of a Domain.
type DomainPhase string

const (
	// DomainPhasePending — waiting for DomainClaim, DomainMapping, or Certificate.
	DomainPhasePending DomainPhase = "Pending"

	// DomainPhaseReady — all resources reconciled, domain serving with TLS.
	DomainPhaseReady DomainPhase = "Ready"

	// DomainPhaseFailed — one or more child resources failed to reconcile.
	// See status.message for the root cause.
	DomainPhaseFailed DomainPhase = "Failed"
)

// blanketopsLabels returns the full canonical BlanketOps label set
// as seen on the real Domain CR — all ten labels.
func blanketopsLabels(envName, envType string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":                         "blanketops-environments",
		"app.kubernetes.io/part-of":                      "blanketops",
		"app.kubernetes.io/managed-by":                   "kustomize",
		"app.kubernetes.io/version":                      "v0.0.1",
		"environments.blanketops.dev/name":               envName,
		"environments.blanketops.dev/type":               envType,
		"environments.blanketops.dev/api-version":        "v0.1.4",
		"environments.blanketops.dev/contract-version":   "v0.1.7",
		"environments.blanketops.dev/controller-version": "v0.2.3",
		"environments.blanketops.dev/operator-version":   "v0.2.1",
	}
}

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewPlatformDomainData returns a DomainData matching the canonical
// domain-for-kaniko-sample — platform TLS, mTLS enforced, 720h renewBefore.
// Matches: networks.blanketops.dev/v1alpha1 Domain domain-for-kaniko-sample
func NewPlatformDomainData(name, namespace, host, routeRefName, envName string) *DomainData {
	return &DomainData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networks.blanketops.dev/v1alpha1",
			Kind:       "Domain",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    blanketopsLabels(envName, "dev"),
		},
		Spec: DomainSpec{
			Contract: DomainContract{
				Host: host,
				RouteRef: RouteRef{
					Name: routeRefName,
				},
				TLSStrategy: TLSStrategyPlatform,
				MTLS: MTLS{
					Enforced: true,
				},
				RenewBefore: "720h",
			},
		},
	}
}

// NewCustomDomainData returns a DomainData for a client-owned zone.
// Triggers HTTP01 Issuer + Certificate in the tenant namespace.
// Use for: app.client-a.co.za pattern hosts.
func NewCustomDomainData(name, namespace, host, routeRefName, envName string) *DomainData {
	d := NewPlatformDomainData(name, namespace, host, routeRefName, envName)
	d.Spec.Contract.TLSStrategy = TLSStrategyCustom
	return d
}

// NewNoMTLSDomainData returns a DomainData with mTLS not enforced.
// Useful for testing non-mTLS ingress paths or legacy workloads.
func NewNoMTLSDomainData(name, namespace, host, routeRefName, envName string) *DomainData {
	d := NewPlatformDomainData(name, namespace, host, routeRefName, envName)
	d.Spec.Contract.MTLS = MTLS{Enforced: false}
	return d
}
