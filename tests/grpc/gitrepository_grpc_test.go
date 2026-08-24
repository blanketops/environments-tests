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

// GitRepositoryService gRPC tests
// APIVersion: sources.blanketops.dev/v1alpha1
// Proto:      blanketops/sources/v1alpha1/gitrepository.proto
// Service:    GitRepositoryService
//
// Each test calls the real GitRepositoryService over a real gRPC
// connection (in-process, see TestMain in suite_test.go) — no fakes,
// no mocks.
//
// UpdateGitRepository/PatchGitRepository/SyncGitRepository weren't
// among the originally stubbed test names (only 7 of this service's 8
// RPCs had one) even though the proto has them — added here alongside
// the original 7 so the full real surface implemented in
// gitrepository_server_test.go is actually exercised.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/types"

	sourcesv1alpha1 "github.com/blanketops/environments-api/api/sources/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	gitrepositorypb "github.com/blanketops/environments-contract/blanketops/sources/v1alpha1"
)

func newGitRepositorySpec() *gitrepositorypb.GitRepositorySpec {
	return &gitrepositorypb.GitRepositorySpec{
		Provider: &commonv1.GitProvider{Provider: commonv1.GitProvider_GIT_PROVIDER_GITHUB},
		Repository: &gitrepositorypb.GitRepositoryRef{
			Owner: "ntlaletsi70",
			Name:  "for-kaniko-app",
		},
		Webhooks: []*gitrepositorypb.GitRepositoryWebhook{
			{
				Events: []*commonv1.GitEventType{
					{Type: commonv1.GitEventType_GIT_EVENT_TYPE_PUSH},
					{Type: commonv1.GitEventType_GIT_EVENT_TYPE_PULL_REQUEST},
				},
			},
		},
		CredentialsSecret: "github-app-credentials",
	}
}

// createTestGitRepository creates a GitRepository via the real gRPC
// service and registers its cleanup, returning the created proto
// GitRepository.
func createTestGitRepository(
	t *testing.T, c gitrepositorypb.GitRepositoryServiceClient, spec *gitrepositorypb.GitRepositorySpec,
) *gitrepositorypb.GitRepository {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreateGitRepository(ctx, &gitrepositorypb.CreateGitRepositoryRequest{Spec: spec})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGitRepository())

	name := resp.GetGitRepository().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteGitRepository(context.Background(), &gitrepositorypb.DeleteGitRepositoryRequest{Name: name})
	})

	return resp.GetGitRepository()
}

func TestGitRepositoryService_CreateGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)

	repo := createTestGitRepository(t, c, newGitRepositorySpec())

	assert.Equal(t, "ntlaletsi70", repo.GetSpec().GetRepository().GetOwner())
	assert.Equal(t, commonv1.GitProvider_GIT_PROVIDER_GITHUB, repo.GetSpec().GetProvider().GetProvider())
}

func TestGitRepositoryService_GetGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	created := createTestGitRepository(t, c, newGitRepositorySpec())

	resp, err := c.GetGitRepository(context.Background(), &gitrepositorypb.GetGitRepositoryRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGitRepository())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetGitRepository().GetMetadata().GetName())
	wantRepoName := created.GetSpec().GetRepository().GetName()
	assert.Equal(t, wantRepoName, resp.GetGitRepository().GetSpec().GetRepository().GetName())
}

func TestGitRepositoryService_UpdateGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	ctx := context.Background()
	created := createTestGitRepository(t, c, newGitRepositorySpec())

	newSpec := newGitRepositorySpec()
	newSpec.CredentialsSecret = "rotated-github-app-credentials"

	resp, err := c.UpdateGitRepository(ctx, &gitrepositorypb.UpdateGitRepositoryRequest{
		GitRepository: &gitrepositorypb.GitRepository{
			Metadata: &commonv1.Metadata{Name: created.GetMetadata().GetName()},
			Spec:     newSpec,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "rotated-github-app-credentials", resp.GetGitRepository().GetSpec().GetCredentialsSecret())

	getResp, err := c.GetGitRepository(ctx, &gitrepositorypb.GetGitRepositoryRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	assert.Equal(t, "rotated-github-app-credentials", getResp.GetGitRepository().GetSpec().GetCredentialsSecret())
}

func TestGitRepositoryService_PatchGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	ctx := context.Background()
	created := createTestGitRepository(t, c, newGitRepositorySpec())

	// The proto's own doc comment on PatchGitRepositoryRequest shows this
	// patch as {"spec":{"webhooks":[{"events":["GIT_EVENT_TYPE_PUSH"]}]}}
	// — but events is `repeated GitEventType`, a wrapper message
	// (`{type: ...}`), not a bare enum, so that example doesn't actually
	// match the schema it's documenting. Using the real shape here.
	patch := `{"spec":{"webhooks":[{"events":[{"type":"GIT_EVENT_TYPE_PUSH"}]}]}}`
	resp, err := c.PatchGitRepository(ctx, &gitrepositorypb.PatchGitRepositoryRequest{
		Name:  created.GetMetadata().GetName(),
		Patch: patch,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGitRepository())

	require.Len(t, resp.GetGitRepository().GetSpec().GetWebhooks(), 1)
	require.Len(t, resp.GetGitRepository().GetSpec().GetWebhooks()[0].GetEvents(), 1)
	assert.Equal(t,
		commonv1.GitEventType_GIT_EVENT_TYPE_PUSH,
		resp.GetGitRepository().GetSpec().GetWebhooks()[0].GetEvents()[0].GetType(),
	)
	// Fields outside the patch survive untouched.
	assert.Equal(t, "ntlaletsi70", resp.GetGitRepository().GetSpec().GetRepository().GetOwner())
}

func TestGitRepositoryService_ListGitRepositories(t *testing.T) {
	c := gitRepositoryClient(t)
	ctx := context.Background()
	created := createTestGitRepository(t, c, newGitRepositorySpec())

	resp, err := c.ListGitRepositories(ctx, &gitrepositorypb.ListGitRepositoriesRequest{})
	require.NoError(t, err)

	var found bool
	for _, r := range resp.GetGitRepositories() {
		if r.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListGitRepositories to include the created repository")

	provider := &commonv1.GitProvider{Provider: commonv1.GitProvider_GIT_PROVIDER_GITHUB}
	filtered, err := c.ListGitRepositories(ctx, &gitrepositorypb.ListGitRepositoriesRequest{Provider: provider})
	require.NoError(t, err)
	for _, r := range filtered.GetGitRepositories() {
		assert.Equal(t, commonv1.GitProvider_GIT_PROVIDER_GITHUB, r.GetSpec().GetProvider().GetProvider())
	}
}

func TestGitRepositoryService_DeleteGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	ctx := context.Background()

	resp, err := c.CreateGitRepository(ctx, &gitrepositorypb.CreateGitRepositoryRequest{Spec: newGitRepositorySpec()})
	require.NoError(t, err)
	name := resp.GetGitRepository().GetMetadata().GetName()

	delResp, err := c.DeleteGitRepository(ctx, &gitrepositorypb.DeleteGitRepositoryRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetGitRepository(ctx, &gitrepositorypb.GetGitRepositoryRequest{Name: name})
	assert.Error(t, err, "expected the deleted repository to be gone")
}

func TestGitRepositoryService_SyncGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	created := createTestGitRepository(t, c, newGitRepositorySpec())

	resp, err := c.SyncGitRepository(context.Background(), &gitrepositorypb.SyncGitRepositoryRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGitRepository())
	assert.NotNil(t, resp.GetGitRepository().GetStatus().GetLastUpdatedAt())
}

func TestGitRepositoryService_WatchGitRepository(t *testing.T) {
	c := gitRepositoryClient(t)
	created := createTestGitRepository(t, c, newGitRepositorySpec())
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchGitRepository(ctx, &gitrepositorypb.WatchGitRepositoryRequest{Name: name})
	require.NoError(t, err)

	// Drive Pending -> Ready directly against the cluster, same as the
	// other watch tests — PatchGitRepository (like PatchBuild and every
	// other Patch RPC in this suite) only ever writes .spec back, so it
	// can't be used to change status here.
	go func() {
		time.Sleep(200 * time.Millisecond)

		cr := &sourcesv1alpha1.GitRepository{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := gitRepositoryStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Ready = true
		raw, err := gitRepositoryStatusToRaw(st)
		if err != nil {
			return
		}
		cr.Status.Contract = raw
		_ = k8sClient.Status().Update(ctx, cr)
	}()

	var sawReady bool
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		if event.GetGitRepository().GetStatus().GetReady() {
			sawReady = true
			break
		}
	}
	assert.True(t, sawReady, "expected WatchGitRepository to deliver the ready transition")
}

func TestGitRepositoryService_PushOnly(t *testing.T) {
	c := gitRepositoryClient(t)

	spec := newGitRepositorySpec()
	spec.Webhooks = []*gitrepositorypb.GitRepositoryWebhook{
		{Events: []*commonv1.GitEventType{{Type: commonv1.GitEventType_GIT_EVENT_TYPE_PUSH}}},
	}

	repo := createTestGitRepository(t, c, spec)

	require.Len(t, repo.GetSpec().GetWebhooks(), 1)
	require.Len(t, repo.GetSpec().GetWebhooks()[0].GetEvents(), 1)
	assert.Equal(t, commonv1.GitEventType_GIT_EVENT_TYPE_PUSH, repo.GetSpec().GetWebhooks()[0].GetEvents()[0].GetType())
}

func TestGitRepositoryService_CustomHookURL(t *testing.T) {
	c := gitRepositoryClient(t)

	// The proto has no explicit hookUrl field on GitRepositorySpec — a
	// custom ingress hook URL is a controller-side reconciliation detail
	// (webhook registration target), not part of the intent surface this
	// RPC accepts. This exercises the same creation path against a
	// non-default credentials secret, standing in for what the original
	// stub's "production ingress hookUrl" scenario meant to cover.
	spec := newGitRepositorySpec()
	spec.CredentialsSecret = "production-github-app-credentials"

	repo := createTestGitRepository(t, c, spec)

	assert.Equal(t, "production-github-app-credentials", repo.GetSpec().GetCredentialsSecret())
}
