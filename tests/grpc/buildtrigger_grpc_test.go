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

// BuildTriggerService gRPC tests
// APIVersion: environments.blanketops.dev/v1
// Proto:      blanketops/environments/v1/buildtrigger.proto
// Service:    BuildTriggerService
//
// BuildTrigger is immutable once created — no Update or Patch tests.

func TestBuildTriggerService_CreateBuildTrigger(t *testing.T) {
	t.Skip("wire: BuildTriggerService.CreateBuildTrigger — real gRPC server required")
	// d := testdata.NewBuildTriggerData("grpc-bt-push", "default")
	// resp, err := client.CreateBuildTrigger(ctx, &v1.CreateBuildTriggerRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "grpc-bt-push", resp.BuildTrigger.Metadata.Name)
}

func TestBuildTriggerService_GetBuildTrigger(t *testing.T) {
	t.Skip("wire: BuildTriggerService.GetBuildTrigger — real gRPC server required")
}

func TestBuildTriggerService_ListBuildTriggers(t *testing.T) {
	t.Skip("wire: BuildTriggerService.ListBuildTriggers — filter by phase, source, repository, build_name")
	// resp, err := client.ListBuildTriggers(ctx, &v1.ListBuildTriggersRequest{
	//   Source:    v1.BUILD_TRIGGER_SOURCE_GITHUB.Enum(),
	//   BuildName: proto.String("build-for-kaniko-sample"),
	// })
}

func TestBuildTriggerService_ListBuildTriggers_Pagination(t *testing.T) {
	t.Skip("wire: BuildTriggerService.ListBuildTriggers — pagination")
}

func TestBuildTriggerService_DeleteBuildTrigger(t *testing.T) {
	t.Skip("wire: BuildTriggerService.DeleteBuildTrigger — terminal triggers pruned by GC controller")
	// resp, err := client.DeleteBuildTrigger(ctx, &v1.DeleteBuildTriggerRequest{Name: name})
	// require.NoError(t, err)
	// assert.True(t, resp.Success)
}

func TestBuildTriggerService_WatchBuildTrigger(t *testing.T) {
	t.Skip("wire: BuildTriggerService.WatchBuildTrigger — closes on CONSUMED or REJECTED")
	// stream closes when trigger reaches terminal phase
}
