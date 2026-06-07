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

// DeploymentService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1/deployment.proto
// Service:    DeploymentService

func TestDeploymentService_CreateDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.CreateDeployment — real gRPC server required")
	// d := testdata.NewDeploymentData("grpc-test-deployment", "default")
	// resp, err := client.CreateDeployment(ctx, &v1.CreateDeploymentRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "grpc-test-deployment", resp.Deployment.Metadata.Name)
}

func TestDeploymentService_GetDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.GetDeployment — real gRPC server required")
}

func TestDeploymentService_UpdateDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.UpdateDeployment — full spec replace")
}

func TestDeploymentService_PatchDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.PatchDeployment — JSON merge patch RFC 7396")
	// patch := `{"spec":{"contract":{"strategy":"DEPLOYMENT_STRATEGY_CANARY"}}}`
}

func TestDeploymentService_ListDeployments(t *testing.T) {
	t.Skip("wire: DeploymentService.ListDeployments — filter by phase and runtime")
}

func TestDeploymentService_ListDeployments_Pagination(t *testing.T) {
	t.Skip("wire: DeploymentService.ListDeployments — pagination")
}

func TestDeploymentService_DeleteDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.DeleteDeployment — verify workloads torn down")
	// resp.Success must be true
	// controller tears down managed workloads on deletion
}

func TestDeploymentService_WatchDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.WatchDeployment — streaming phase transitions")
	// PENDING → DEPLOYING → READY transitions delivered in order
}

func TestDeploymentService_MultiUnitDeployment(t *testing.T) {
	t.Skip("wire: DeploymentService.CreateDeployment — multi-unit, imageAutomation=true")
	// d := testdata.NewMultiUnitDeploymentData("grpc-multi-unit", "default")
	// two ServiceUnits, imageAutomation enabled
}
