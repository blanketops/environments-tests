/*
Copyright 2026 BlanketOps.

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
	"encoding/json"
	"testing"

	networksv1alpha1 "github.com/blanketops/environments-api/api/networks/v1alpha1"
	domainresolve "github.com/blanketops/environments/resolution/domain/resolve"
	routeresolve "github.com/blanketops/environments/resolution/route/resolve"
	"k8s.io/apimachinery/pkg/runtime"
)

// Every fixture builder must produce a contract the engine's own resolvers
// accept; see tests/data/environments/conformance_test.go.

func contractOf(t *testing.T, v any) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	return runtime.RawExtension{Raw: raw}
}

func TestFixturesResolveWithTheEngine(t *testing.T) {
	const ns, env, host = "dev", "for-kaniko-app", "api.dev.blanketops.online"

	routes := map[string]*RouteData{
		"NewRouteData":             NewRouteData("app-route", ns, host, env),
		"NewCustomDomainRouteData": NewCustomDomainRouteData("app-route", ns, host, env),
		"NewDisabledRouteData":     NewDisabledRouteData("app-route", ns, host, env),
		"NewHTTPRouteData":         NewHTTPRouteData("app-route", ns, host, env),
		"NewKnativeRouteData":      NewKnativeRouteData("app-route", ns, host, env),
	}
	for name, d := range routes {
		t.Run(name, func(t *testing.T) {
			// The Route fixture's whole spec is the contract, as the
			// controller suite sends it (routeFromData).
			cr := &networksv1alpha1.Route{ObjectMeta: d.ObjectMeta,
				Spec: networksv1alpha1.RouteSpec{Contract: contractOf(t, d.Spec)}}
			if _, err := routeresolve.ResolveRoute(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}

	domains := map[string]*DomainData{
		"NewPlatformDomainData": NewPlatformDomainData("app-domain", ns, host, "app-route", env),
		"NewCustomDomainData":   NewCustomDomainData("app-domain", ns, host, "app-route", env),
		"NewNoMTLSDomainData":   NewNoMTLSDomainData("app-domain", ns, host, "app-route", env),
	}
	for name, d := range domains {
		t.Run(name, func(t *testing.T) {
			cr := &networksv1alpha1.Domain{ObjectMeta: d.ObjectMeta,
				Spec: networksv1alpha1.DomainSpec{Contract: contractOf(t, d.Spec.Contract)}}
			if _, err := domainresolve.ResolveDomain(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}
}
