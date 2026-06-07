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

// +groupName=networking.environments.blanketops.dev

// RouteData is the test/data representation of a Route CR.
// Mirrors the Route API type used in the networking controller.
// Used to construct test fixtures without importing the full API type.
//
// Architectural role:
//   Route is intentionally the thinnest CR in the BlanketOps stack.
//   It declares the host, path, TLS intent, and runtime binding for a workload.
//   The controller reconciles a Domain CR as an owned child — the Domain owns
//   the full cert/mapping chain (DomainClaim, DomainMapping, Issuer, Certificate).
//   Deleting a Route cascades to its owned Domain CR.
//
// TLS split:
//   *.dev.domain.co.za  → platform (DNS01 wildcard ClusterIssuer)
//   client-a.co.za      → custom  (HTTP01 namespaced Issuer per tenant)
//
// APIVersion: networking.environments.blanketops.dev/v1alpha1
type RouteData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

// RouteSpec is the desired state of a Route.
// Mirrors networking.environments.blanketops.dev/v1alpha1 RouteSpec.
// Intentionally flat — Route owns no complex nested structs.
type RouteSpec struct {
	// Fully qualified domain name this route serves.
	// Platform domains: *.dev.domain.co.za — covered by wildcard cert automatically.
	// Custom domains:   client-a.co.za     — triggers HTTP01 per-tenant Issuer.
	Host string `json:"host"`

	// URL path prefix for this route.
	// e.g. / for root, /api for API routes.
	// Defaults to / if not specified.
	Path string `json:"path,omitempty"`

	// Whether this route is active and should be reconciled.
	// Set to false to temporarily disable routing without deleting the CR.
	Enabled bool `json:"enabled"`

	// Whether TLS should be provisioned for this route.
	// When true — controller reconciles cert-manager resources via the owned Domain CR.
	// When false — route serves HTTP only.
	TLSEnabled bool `json:"tlsEnabled"`

	// Runtime backend that serves traffic for this route.
	// Determines which ingress controller handles the route.
	// e.g. kubernetes.io/container-runtime (Kourier for Knative, nginx for standard)
	Runtime RouteRuntime `json:"runtime"`
}

// RouteRuntime is the serving runtime backend for a Route.
type RouteRuntime string

const (
	// RouteRuntimeKubernetes — standard Kubernetes container runtime.
	// Traffic served via Kourier (Knative) or nginx ingress.
	// Default and currently supported runtime.
	RouteRuntimeKubernetes RouteRuntime = "kubernetes.io/container-runtime"

	// RouteRuntimeKnative — Knative Serving runtime.
	// Requires Knative Serving + Kourier installed on the cluster.
	// Enables scale-to-zero and traffic splitting via DomainMapping.
	RouteRuntimeKnative RouteRuntime = "knative.io/serving"
)

// RouteStatus is the observed state of a Route.
type RouteStatus struct {
	// Current lifecycle phase.
	Phase RoutePhase `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// Whether the owned Domain CR has been reconciled successfully.
	// When false — TLS cert or DomainMapping is still provisioning.
	DomainReady bool `json:"domainReady,omitempty"`

	// Name of the owned Domain CR reconciled by this Route.
	DomainRef string `json:"domainRef,omitempty"`

	// When the controller last successfully reconciled this Route.
	LastReconciledAt *metav1.Time `json:"lastReconciledAt,omitempty"`
}

// RoutePhase is the lifecycle phase of a Route.
type RoutePhase string

const (
	// RoutePhasePending — waiting for the owned Domain CR to become ready.
	RoutePhasePending RoutePhase = "Pending"

	// RoutePhaseReady — route is active and serving traffic.
	// TLS cert provisioned (if tlsEnabled) and DomainMapping active.
	RoutePhaseReady RoutePhase = "Ready"

	// RoutePhaseFailed — route failed to reconcile.
	// See status.message for the root cause.
	RoutePhaseFailed RoutePhase = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewRouteData returns a RouteData for a platform domain route.
// Platform domain: *.dev.domain.co.za — covered by wildcard DNS01 cert.
// Matches: networking.environments.blanketops.dev/v1alpha1 Route
func NewRouteData(name, namespace, host string) *RouteData {
	return &RouteData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.environments.blanketops.dev/v1alpha1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
			    "app.kubernetes.io/name":                       "blanketops-environments",
		        "app.kubernetes.io/part-of":                    "blanketops",
		        "app.kubernetes.io/managed-by":                 "kustomize",
		        "app.kubernetes.io/version":                    "v0.0.1",
	            "environments.blanketops.dev/name":             envName,
		        "environments.blanketops.dev/type":             envType,
		        "environments.blanketops.dev/api-version":      "v0.1.4",
		        "environments.blanketops.dev/contract-version": "v0.1.7",
		        "environments.blanketops.dev/controller-version": "v0.2.3",
		        "environments.blanketops.dev/operator-version": "v0.2.1",
			},
		},
		Spec: RouteSpec{
			Host:       host,
			Path:       "/",
			Enabled:    true,
			TLSEnabled: true,
			Runtime:    RouteRuntimeKubernetes,
		},
	}
}

// NewCustomDomainRouteData returns a RouteData for a custom tenant domain.
// Custom domain: client-a.co.za — triggers HTTP01 Issuer per tenant namespace.
func NewCustomDomainRouteData(name, namespace, host string) *RouteData {
	r := NewRouteData(name, namespace, host)
	r.ObjectMeta.Labels["environments.blanketops.dev/type"] = "production"
	return r
}

// NewDisabledRouteData returns a RouteData with enabled=false.
// Simulates a route that is temporarily disabled without deletion.
func NewDisabledRouteData(name, namespace, host string) *RouteData {
	r := NewRouteData(name, namespace, host)
	r.Spec.Enabled = false
	return r
}

// NewHTTPRouteData returns a RouteData with TLS disabled — HTTP only.
// Useful for testing non-TLS ingress paths.
func NewHTTPRouteData(name, namespace, host string) *RouteData {
	r := NewRouteData(name, namespace, host)
	r.Spec.TLSEnabled = false
	return r
}

// NewKnativeRouteData returns a RouteData targeting the Knative serving runtime.
// Requires Knative Serving + Kourier installed on the cluster.
func NewKnativeRouteData(name, namespace, host string) *RouteData {
	r := NewRouteData(name, namespace, host)
	r.Spec.Runtime = RouteRuntimeKnative
	return r
}