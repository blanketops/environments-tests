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

// BuildService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1alpha1/build.proto
// Service:    BuildService
//
// Wire these tests when the gRPC server is running.
// Each test calls the real BuildService over a gRPC connection —
// no fakes, no mocks. Server must be reachable at grpcAddr().

// TODO: func grpcAddr() string { return os.Getenv("BLANKETOPS_GRPC_ADDR") }
// TODO: func buildClient(t *testing.T) environmentsv1alpha1.BuildServiceClient

func TestBuildService_CreateBuild(t *testing.T) {
	t.Skip("wire: BuildService.CreateBuild — real gRPC server required")
	// d := testdata.NewBuildData("grpc-test-build", "default")
	// resp, err := client.CreateBuild(ctx, &v1alpha1.CreateBuildRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "grpc-test-build", resp.Build.Metadata.Name)
}

func TestBuildService_GetBuild(t *testing.T) {
	t.Skip("wire: BuildService.GetBuild — real gRPC server required")
	// resp, err := client.GetBuild(ctx, &v1alpha1.GetBuildRequest{Name: "grpc-test-build"})
	// require.NoError(t, err)
	// assert.NotNil(t, resp.Build)
}

func TestBuildService_UpdateBuild(t *testing.T) {
	t.Skip("wire: BuildService.UpdateBuild — real gRPC server required")
	// full replace of spec — verify controller reconciles updated spec
}

func TestBuildService_PatchBuild(t *testing.T) {
	t.Skip("wire: BuildService.PatchBuild — JSON merge patch RFC 7396")
	// patch := `{"spec":{"policy":{"retry":{"max_attempts":5}}}}`
	// resp, err := client.PatchBuild(ctx, &v1alpha1.PatchBuildRequest{Name: name, Patch: patch})
	// require.NoError(t, err)
	// assert.Equal(t, uint32(5), resp.Build.Spec.Policy.Retry.MaxAttempts)
}

func TestBuildService_ListBuilds(t *testing.T) {
	t.Skip("wire: BuildService.ListBuilds — filter by phase and strategy")
	// resp, err := client.ListBuilds(ctx, &v1alpha1.ListBuildsRequest{Strategy: proto.String("kaniko")})
	// require.NoError(t, err)
	// assert.NotEmpty(t, resp.Builds)
}

func TestBuildService_ListBuilds_Pagination(t *testing.T) {
	t.Skip("wire: BuildService.ListBuilds — pagination via page_size + page_token")
	// page1, err := client.ListBuilds(ctx, &v1alpha1.ListBuildsRequest{PageSize: proto.Int32(2)})
	// require.NoError(t, err)
	// assert.NotEmpty(t, page1.NextPageToken)
	// page2, err := client.ListBuilds(ctx, &v1alpha1.ListBuildsRequest{PageToken: page1.NextPageToken})
	// require.NoError(t, err)
	// assert.NotEmpty(t, page2.Builds)
}

func TestBuildService_DeleteBuild(t *testing.T) {
	t.Skip("wire: BuildService.DeleteBuild — verify Shipwright Build + BuildRuns are GC'd")
	// resp, err := client.DeleteBuild(ctx, &v1alpha1.DeleteBuildRequest{Name: name})
	// require.NoError(t, err)
	// assert.True(t, resp.Success)
}

func TestBuildService_TriggerBuild(t *testing.T) {
	t.Skip("wire: BuildService.TriggerBuild — verify Shipwright BuildRun emitted")
	// resp, err := client.TriggerBuild(ctx, &v1alpha1.TriggerBuildRequest{Name: name})
	// require.NoError(t, err)
	// assert.NotEmpty(t, resp.ExecutionRef)
}

func TestBuildService_WatchBuild(t *testing.T) {
	t.Skip("wire: BuildService.WatchBuild — streaming, verify phase transitions delivered")
	// stream, err := client.WatchBuild(ctx, &v1alpha1.WatchBuildRequest{Name: name})
	// require.NoError(t, err)
	// event, err := stream.Recv()
	// require.NoError(t, err)
	// assert.NotNil(t, event.Build)
}
