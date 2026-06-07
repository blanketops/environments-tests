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

// PackageService gRPC tests
// APIVersion: environments.blanketops.dev/v1
// Proto:      blanketops/environments/v1/package.proto
// Service:    PackageService

func TestPackageService_CreatePackage(t *testing.T) {
	t.Skip("wire: PackageService.CreatePackage — real gRPC server required")
	// d := testdata.NewPackageData("grpc-test-package", "default")
	// resp, err := client.CreatePackage(ctx, &v1.CreatePackageRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "Neo", resp.Package.Spec.Contract.PackageMaintainers[0].Name)
}

func TestPackageService_GetPackage(t *testing.T) {
	t.Skip("wire: PackageService.GetPackage — real gRPC server required")
}

func TestPackageService_UpdatePackage(t *testing.T) {
	t.Skip("wire: PackageService.UpdatePackage — full spec replace")
}

func TestPackageService_PatchPackage(t *testing.T) {
	t.Skip("wire: PackageService.PatchPackage — JSON merge patch RFC 7396")
	// patch := `{"spec":{"contract":{"version":"v1.3.0","package_kapp_diff":true}}}`
}

func TestPackageService_ListPackages(t *testing.T) {
	t.Skip("wire: PackageService.ListPackages — filter by phase and diff_enabled")
}

func TestPackageService_ListPackages_Pagination(t *testing.T) {
	t.Skip("wire: PackageService.ListPackages — pagination")
}

func TestPackageService_DeletePackage(t *testing.T) {
	t.Skip("wire: PackageService.DeletePackage — CR deleted, state_repository contents retained")
	// resp.Success must be true
	// state repository contents are NOT deleted — CR-only deletion
}

func TestPackageService_WatchPackage(t *testing.T) {
	t.Skip("wire: PackageService.WatchPackage — streaming phase transitions")
	// PENDING → READY transition delivered
	// diff_output populated when package_kapp_diff=true
}

func TestPackageService_DisabledPackage(t *testing.T) {
	t.Skip("wire: PackageService.CreatePackage — enabled=false, reconciliation suspended")
	// d := testdata.NewDisabledPackageData("grpc-disabled-pkg", "default")
	// controller suspends reconciliation — domain handles enabled=false gate
}
