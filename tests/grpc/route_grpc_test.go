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

package grpc_test

import (
	"testing"
)

// RouteService gRPC tests
// APIVersion: networking.environments.blanketops.dev/v1alpha1
// Proto:      blanketops/networks/v1alpha1/route.proto
// Service:    RouteService
//
// Route is the thinnest CR — host, path, enabled, tlsEnabled, runtime.
// Controller reconciles a Domain CR as an owned child on creation.

func TestRouteService_CreateRoute_Platform(t *testing.T) {
	t.Skip("wire: RouteService.CreateRoute — platform domain, DNS01 wildcard")
	// d := testdata.NewRouteData("grpc-route-platform", "dev", "api.dev.domain.co.za")
	// resp, err := client.CreateRoute(ctx, &v1alpha1.CreateRouteRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.True(t, resp.Route.Spec.TLSEnabled)
	// assert.Equal(t, "kubernetes.io/container-runtime", string(resp.Route.Spec.Runtime))
}

func TestRouteService_CreateRoute_Custom(t *testing.T) {
	t.Skip("wire: RouteService.CreateRoute — custom domain, HTTP01 per-tenant Issuer")
	// d := testdata.NewCustomDomainRouteData("grpc-route-custom", "dev", "app.client-a.co.za")
}

func TestRouteService_GetRoute(t *testing.T) {
	t.Skip("wire: RouteService.GetRoute — real gRPC server required")
}

func TestRouteService_UpdateRoute(t *testing.T) {
	t.Skip("wire: RouteService.UpdateRoute — full spec replace")
}

func TestRouteService_PatchRoute(t *testing.T) {
	t.Skip("wire: RouteService.PatchRoute — JSON merge patch RFC 7396")
	// patch := `{"spec":{"enabled":false}}`
}

func TestRouteService_ListRoutes(t *testing.T) {
	t.Skip("wire: RouteService.ListRoutes — filter by host and runtime")
}

func TestRouteService_ListRoutes_Pagination(t *testing.T) {
	t.Skip("wire: RouteService.ListRoutes — pagination")
}

func TestRouteService_DeleteRoute(t *testing.T) {
	t.Skip("wire: RouteService.DeleteRoute — cascades to owned Domain CR")
}

func TestRouteService_WatchRoute(t *testing.T) {
	t.Skip("wire: RouteService.WatchRoute — streaming phase transitions")
	// Pending → Ready when owned Domain CR reconciled
}

func TestRouteService_DisableRoute(t *testing.T) {
	t.Skip("wire: RouteService.PatchRoute — enabled=false, route disabled without deletion")
	// d := testdata.NewDisabledRouteData("grpc-route-disabled", "dev", "api.dev.domain.co.za")
}

func TestRouteService_KnativeRuntime(t *testing.T) {
	t.Skip("wire: RouteService.CreateRoute — knative runtime, scale-to-zero")
	// d := testdata.NewKnativeRouteData("grpc-route-knative", "dev", "api.dev.domain.co.za")
}
