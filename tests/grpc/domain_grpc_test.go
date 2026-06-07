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

// DomainService gRPC tests
// APIVersion: networks.blanketops.dev/v1alpha1
// Proto:      blanketops/networks/v1alpha1/domain.proto
// Service:    DomainService
//
// TLS strategy determines which cert chain the controller provisions:
//   platform: DomainClaim + DomainMapping only (wildcard pre-issued)
//   custom:   Issuer + DomainClaim + DomainMapping + Certificate (HTTP01)

func TestDomainService_CreateDomain_Platform(t *testing.T) {
	t.Skip("wire: DomainService.CreateDomain — platform TLS, DNS01 wildcard, mTLS enforced")
	// d := testdata.NewPlatformDomainData("grpc-domain-platform", "dev",
	//   "api.dev.domain.co.za", "shopping-app-route", "environment-sample")
	// resp, err := client.CreateDomain(ctx, &v1alpha1.CreateDomainRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "platform", string(resp.Domain.Spec.Contract.TLSStrategy))
	// assert.True(t, resp.Domain.Spec.Contract.Mtls.Enforced)
}

func TestDomainService_CreateDomain_Custom(t *testing.T) {
	t.Skip("wire: DomainService.CreateDomain — custom TLS, HTTP01 per-tenant, mTLS enforced")
	// d := testdata.NewCustomDomainData("grpc-domain-custom", "dev",
	//   "app.client-a.co.za", "client-a-route", "environment-sample")
	// assert.Equal(t, "custom", string(resp.Domain.Spec.Contract.TLSStrategy))
	// controller must emit: Issuer + DomainClaim + DomainMapping + Certificate
}

func TestDomainService_GetDomain(t *testing.T) {
	t.Skip("wire: DomainService.GetDomain — real gRPC server required")
}

func TestDomainService_UpdateDomain(t *testing.T) {
	t.Skip("wire: DomainService.UpdateDomain — full spec replace")
}

func TestDomainService_PatchDomain(t *testing.T) {
	t.Skip("wire: DomainService.PatchDomain — JSON merge patch RFC 7396")
	// patch := `{"spec":{"contract":{"renew_before":"360h"}}}`
}

func TestDomainService_ListDomains(t *testing.T) {
	t.Skip("wire: DomainService.ListDomains — filter by tlsStrategy and mtlsEnforced")
}

func TestDomainService_ListDomains_Pagination(t *testing.T) {
	t.Skip("wire: DomainService.ListDomains — pagination")
}

func TestDomainService_DeleteDomain(t *testing.T) {
	t.Skip("wire: DomainService.DeleteDomain — DomainClaim + DomainMapping GC'd on delete")
}

func TestDomainService_WatchDomain(t *testing.T) {
	t.Skip("wire: DomainService.WatchDomain — streaming phase transitions")
	// Pending → Ready when cert issued and DomainMapping active
}

func TestDomainService_NoMTLS(t *testing.T) {
	t.Skip("wire: DomainService.CreateDomain — mTLS not enforced, blanketops-proxy sidecar not injected")
	// d := testdata.NewNoMTLSDomainData("grpc-domain-no-mtls", "dev",
	//   "api.dev.domain.co.za", "shopping-app-route", "environment-sample")
}
