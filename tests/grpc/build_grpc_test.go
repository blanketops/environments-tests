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

// BuildService gRPC tests
// APIVersion: environments.blanketops.dev/v1alpha1
// Proto:      blanketops/environments/v1alpha1/build.proto
// Service:    BuildService
//
// Each test calls the real BuildService over a real gRPC connection
// (in-process, see TestMain in suite_test.go) — no fakes, no mocks.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/types"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	buildpb "github.com/blanketops/environments-contract/blanketops/environments/v1alpha1"
)

func newBuildSpec() *buildpb.BuildSpec {
	return &buildpb.BuildSpec{
		Image: "docker.io/nkanyezisolutions/for-kaniko-app:main",
		Strategy: &buildpb.BuildStrategy{
			Name: "kaniko",
			Kind: &commonv1.BuildStrategyKind{Kind: commonv1.BuildStrategyKind_BUILD_STRATEGY_KIND_CLUSTER},
		},
		Source: &buildpb.GitSource{
			Url:         "git@github.com:ntlaletsi70/blanketops-app.git",
			Revision:    "main",
			ContextDir:  ".",
			SslVerify:   true,
			CloneSecret: "github-ssh-credentials",
		},
		ServiceAccount: &buildpb.ServiceAccount{
			Name:   "build-bot",
			Secret: "docker-registry-credentials",
		},
		Policy: &buildpb.BuildPolicy{
			AllowedTriggers: []*buildpb.BuildTriggerPolicy{
				{Type: &commonv1.BuildTriggerType{Type: commonv1.BuildTriggerType_BUILD_TRIGGER_TYPE_COMMIT}},
			},
			Retry: &buildpb.RetryPolicy{OnFailure: true, MaxAttempts: 3},
		},
	}
}

// createTestBuild creates a Build via the real gRPC service and registers
// its cleanup, returning the created proto Build.
func createTestBuild(t *testing.T, c buildpb.BuildServiceClient) *buildpb.Build {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreateBuild(ctx, &buildpb.CreateBuildRequest{Spec: newBuildSpec()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetBuild())

	name := resp.GetBuild().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteBuild(context.Background(), &buildpb.DeleteBuildRequest{Name: name})
	})

	return resp.GetBuild()
}

func TestBuildService_CreateBuild(t *testing.T) {
	c := buildClient(t)

	build := createTestBuild(t, c)

	assert.NotEmpty(t, build.GetMetadata().GetName())
	assert.NotEmpty(t, build.GetMetadata().GetId())
	assert.Equal(t, "docker.io/nkanyezisolutions/for-kaniko-app:main", build.GetSpec().GetImage())
	assert.Equal(t, "kaniko", build.GetSpec().GetStrategy().GetName())
}

func TestBuildService_GetBuild(t *testing.T) {
	c := buildClient(t)
	created := createTestBuild(t, c)

	resp, err := c.GetBuild(context.Background(), &buildpb.GetBuildRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	require.NotNil(t, resp.GetBuild())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetBuild().GetMetadata().GetName())
	assert.Equal(t, created.GetSpec().GetImage(), resp.GetBuild().GetSpec().GetImage())
}

func TestBuildService_UpdateBuild(t *testing.T) {
	c := buildClient(t)
	ctx := context.Background()
	created := createTestBuild(t, c)

	newSpec := newBuildSpec()
	newSpec.Image = "docker.io/nkanyezisolutions/for-kaniko-app:v2"

	resp, err := c.UpdateBuild(ctx, &buildpb.UpdateBuildRequest{
		Build: &buildpb.Build{
			Metadata: &commonv1.Metadata{Name: created.GetMetadata().GetName()},
			Spec:     newSpec,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "docker.io/nkanyezisolutions/for-kaniko-app:v2", resp.GetBuild().GetSpec().GetImage())

	getResp, err := c.GetBuild(ctx, &buildpb.GetBuildRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	assert.Equal(t, "docker.io/nkanyezisolutions/for-kaniko-app:v2", getResp.GetBuild().GetSpec().GetImage())
}

func TestBuildService_PatchBuild(t *testing.T) {
	c := buildClient(t)
	ctx := context.Background()
	created := createTestBuild(t, c)

	patch := `{"spec":{"policy":{"retry":{"max_attempts":5}}}}`
	resp, err := c.PatchBuild(ctx, &buildpb.PatchBuildRequest{Name: created.GetMetadata().GetName(), Patch: patch})
	require.NoError(t, err)
	require.NotNil(t, resp.GetBuild())

	assert.Equal(t, uint32(5), resp.GetBuild().GetSpec().GetPolicy().GetRetry().GetMaxAttempts())
	// Fields outside the patch survive untouched.
	assert.Equal(t, "docker.io/nkanyezisolutions/for-kaniko-app:main", resp.GetBuild().GetSpec().GetImage())
	assert.Equal(t, "kaniko", resp.GetBuild().GetSpec().GetStrategy().GetName())
}

func TestBuildService_ListBuilds(t *testing.T) {
	c := buildClient(t)
	ctx := context.Background()
	created := createTestBuild(t, c)

	resp, err := c.ListBuilds(ctx, &buildpb.ListBuildsRequest{})
	require.NoError(t, err)

	var found bool
	for _, b := range resp.GetBuilds() {
		if b.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListBuilds to include the created build")

	strategy := "kaniko"
	filtered, err := c.ListBuilds(ctx, &buildpb.ListBuildsRequest{Strategy: &strategy})
	require.NoError(t, err)
	for _, b := range filtered.GetBuilds() {
		assert.Equal(t, "kaniko", b.GetSpec().GetStrategy().GetName())
	}
}

func TestBuildService_ListBuilds_Pagination(t *testing.T) {
	c := buildClient(t)

	for range 3 {
		createTestBuild(t, c)
	}

	pageSize := int32(1)
	page1, err := c.ListBuilds(context.Background(), &buildpb.ListBuildsRequest{PageSize: &pageSize})
	require.NoError(t, err)
	assert.Len(t, page1.GetBuilds(), 1)
	require.NotNil(t, page1.NextPageToken)
	assert.NotEmpty(t, page1.GetNextPageToken())

	page2, err := c.ListBuilds(context.Background(), &buildpb.ListBuildsRequest{
		PageSize:  &pageSize,
		PageToken: page1.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, page2.GetBuilds(), 1)
	assert.NotEqual(t, page1.GetBuilds()[0].GetMetadata().GetName(), page2.GetBuilds()[0].GetMetadata().GetName())
}

func TestBuildService_DeleteBuild(t *testing.T) {
	c := buildClient(t)
	ctx := context.Background()

	resp, err := c.CreateBuild(ctx, &buildpb.CreateBuildRequest{Spec: newBuildSpec()})
	require.NoError(t, err)
	name := resp.GetBuild().GetMetadata().GetName()

	delResp, err := c.DeleteBuild(ctx, &buildpb.DeleteBuildRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetBuild(ctx, &buildpb.GetBuildRequest{Name: name})
	assert.Error(t, err, "expected the deleted build to be gone")
}

func TestBuildService_TriggerBuild(t *testing.T) {
	c := buildClient(t)
	ctx := context.Background()
	created := createTestBuild(t, c)

	resp, err := c.TriggerBuild(ctx, &buildpb.TriggerBuildRequest{
		Name:   created.GetMetadata().GetName(),
		Params: []*buildpb.Param{{Name: "commit-sha", Value: "abc1234"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetExecutionRef())
	gotPhase := resp.GetBuild().GetStatus().GetPhase().GetPhase()
	assert.Equal(t, commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_RUNNING, gotPhase)

	getResp, err := c.GetBuild(ctx, &buildpb.GetBuildRequest{Name: created.GetMetadata().GetName()})
	require.NoError(t, err)
	assert.Equal(t, resp.GetExecutionRef(), getResp.GetBuild().GetStatus().GetExecutionRef())

	var foundOverride bool
	for _, p := range getResp.GetBuild().GetSpec().GetParams() {
		if p.GetName() == "commit-sha" && p.GetValue() == "abc1234" {
			foundOverride = true
		}
	}
	assert.True(t, foundOverride, "expected TriggerBuild's param override to persist on spec.params")
}

func TestBuildService_WatchBuild(t *testing.T) {
	c := buildClient(t)
	created := createTestBuild(t, c)
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchBuild(ctx, &buildpb.WatchBuildRequest{Name: name})
	require.NoError(t, err)

	// Nothing in this environment drives real phase transitions (no
	// Shipwright BuildRun controller is installed) — drive one directly
	// against the cluster, the same way tests/fake manually calls
	// Reconcile() in the absence of the full stack, and assert the real
	// k8s watch behind WatchBuild delivers it.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cr := &environmentsv1alpha1.Build{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := buildStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Phase = &commonv1.BuildStatusPhase{Phase: commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_SUCCEEDED}
		raw, err := buildStatusToRaw(st)
		if err != nil {
			return
		}
		cr.Status.Contract = raw
		_ = k8sClient.Status().Update(ctx, cr)
	}()

	var sawSucceeded bool
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		if event.GetBuild().GetStatus().GetPhase().GetPhase() == commonv1.BuildStatusPhase_BUILD_STATUS_PHASE_SUCCEEDED {
			sawSucceeded = true
			break
		}
	}
	assert.True(t, sawSucceeded, "expected WatchBuild to deliver the Succeeded transition")
}
