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

// ServiceUnitService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1alpha1/serviceunit.proto
// Service:    ServiceUnitService

func TestServiceUnitService_CreateServiceUnit_Static(t *testing.T) {
	t.Skip("wire: ServiceUnitService.CreateServiceUnit — type=static, image provided directly")
	// d := testdata.NewStaticServiceUnitData("grpc-su-web", "default", "for-kaniko-app")
	// resp, err := client.CreateServiceUnit(ctx, &v1alpha1.CreateServiceUnitRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "static", string(resp.ServiceUnit.Spec.Contract.Type))
	// assert.NotEmpty(t, resp.ServiceUnit.Spec.Contract.Image)
	// assert.Nil(t, resp.ServiceUnit.Spec.Contract.BuildRef)
}

func TestServiceUnitService_CreateServiceUnit_Build(t *testing.T) {
	t.Skip("wire: ServiceUnitService.CreateServiceUnit — type=build, image from BuildRun output")
	// d := testdata.NewBuildServiceUnitData("grpc-su-worker", "default", "for-kaniko-app")
	// resp, err := client.CreateServiceUnit(ctx, &v1alpha1.CreateServiceUnitRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "build", string(resp.ServiceUnit.Spec.Contract.Type))
	// assert.NotNil(t, resp.ServiceUnit.Spec.Contract.BuildRef)
	// assert.Empty(t, resp.ServiceUnit.Spec.Contract.Image)
}

func TestServiceUnitService_GetServiceUnit(t *testing.T) {
	t.Skip("wire: ServiceUnitService.GetServiceUnit — real gRPC server required")
}

func TestServiceUnitService_UpdateServiceUnit(t *testing.T) {
	t.Skip("wire: ServiceUnitService.UpdateServiceUnit — full spec replace")
}

func TestServiceUnitService_PatchServiceUnit(t *testing.T) {
	t.Skip("wire: ServiceUnitService.PatchServiceUnit — JSON merge patch RFC 7396")
	// patch := `{"spec":{"contract":{"size":3}}}`
}

func TestServiceUnitService_ListServiceUnits(t *testing.T) {
	t.Skip("wire: ServiceUnitService.ListServiceUnits — filter by type and appType")
}

func TestServiceUnitService_ListServiceUnits_Pagination(t *testing.T) {
	t.Skip("wire: ServiceUnitService.ListServiceUnits — pagination")
}

func TestServiceUnitService_DeleteServiceUnit(t *testing.T) {
	t.Skip("wire: ServiceUnitService.DeleteServiceUnit — verify workload torn down")
}

func TestServiceUnitService_WatchServiceUnit(t *testing.T) {
	t.Skip("wire: ServiceUnitService.WatchServiceUnit — streaming phase transitions")
}
