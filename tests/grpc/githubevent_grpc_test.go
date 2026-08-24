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

// GitHubEventService gRPC tests
// APIVersion: events.blanketops.dev/v1alpha1
// Proto:      blanketops/events/v1alpha1/githubevent.proto
// Service:    GitHubEventService
//
// Each test calls the real GitHubEventService over a real gRPC
// connection (in-process, see TestMain in suite_test.go) — no fakes,
// no mocks.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	commonv1 "github.com/blanketops/environments-contract/blanketops/common/v1"
	githubeventpb "github.com/blanketops/environments-contract/blanketops/events/v1alpha1"
)

func newPushGitHubEventSpec() *githubeventpb.GitHubEventSpec {
	return &githubeventpb.GitHubEventSpec{
		Repository:       "ntlaletsi70/for-kaniko-app",
		EventType:        &commonv1.GitHubEventType{Type: commonv1.GitHubEventType_GIT_HUB_EVENT_TYPE_PUSH},
		Ref:              "refs/heads/main",
		CommitSha:        "abc1234",
		Actor:            "ntlaletsi70",
		EventId:          "delivery-001",
		WebhookSecretRef: "github-webhook-secret",
	}
}

func newPullRequestGitHubEventSpec() *githubeventpb.GitHubEventSpec {
	spec := newPushGitHubEventSpec()
	spec.EventType = &commonv1.GitHubEventType{Type: commonv1.GitHubEventType_GIT_HUB_EVENT_TYPE_PULL_REQUEST}
	spec.Ref = "refs/pull/42/head"
	spec.EventId = "delivery-002"
	return spec
}

// createTestGitHubEvent creates a GitHubEvent via the real gRPC service
// and registers its cleanup, returning the created proto GitHubEvent.
func createTestGitHubEvent(
	t *testing.T, c githubeventpb.GitHubEventServiceClient, spec *githubeventpb.GitHubEventSpec,
) *githubeventpb.GitHubEvent {
	t.Helper()
	ctx := context.Background()

	resp, err := c.CreateGitHubEvent(ctx, &githubeventpb.CreateGitHubEventRequest{Spec: spec})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGithubEvent())

	name := resp.GetGithubEvent().GetMetadata().GetName()
	t.Cleanup(func() {
		_, _ = c.DeleteGitHubEvent(context.Background(), &githubeventpb.DeleteGitHubEventRequest{Name: name})
	})

	return resp.GetGithubEvent()
}

func TestGitHubEventService_CreateGitHubEvent(t *testing.T) {
	c := githubEventClient(t)

	event := createTestGitHubEvent(t, c, newPushGitHubEventSpec())

	assert.Equal(t, commonv1.GitHubEventType_GIT_HUB_EVENT_TYPE_PUSH, event.GetSpec().GetEventType().GetType())
	assert.Equal(t, "github-webhook-secret", event.GetSpec().GetWebhookSecretRef())
	assert.Equal(t, "ntlaletsi70/for-kaniko-app", event.GetSpec().GetRepository())
}

func TestGitHubEventService_GetGitHubEvent(t *testing.T) {
	c := githubEventClient(t)
	created := createTestGitHubEvent(t, c, newPushGitHubEventSpec())

	resp, err := c.GetGitHubEvent(context.Background(), &githubeventpb.GetGitHubEventRequest{
		Name: created.GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetGithubEvent())
	assert.Equal(t, created.GetMetadata().GetName(), resp.GetGithubEvent().GetMetadata().GetName())
	assert.Equal(t, created.GetSpec().GetCommitSha(), resp.GetGithubEvent().GetSpec().GetCommitSha())
}

func TestGitHubEventService_ListGitHubEvents(t *testing.T) {
	c := githubEventClient(t)
	ctx := context.Background()
	created := createTestGitHubEvent(t, c, newPushGitHubEventSpec())

	resp, err := c.ListGitHubEvents(ctx, &githubeventpb.ListGitHubEventsRequest{})
	require.NoError(t, err)

	var found bool
	for _, e := range resp.GetGithubEvents() {
		if e.GetMetadata().GetName() == created.GetMetadata().GetName() {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ListGitHubEvents to include the created event")

	repo := "ntlaletsi70/for-kaniko-app"
	filtered, err := c.ListGitHubEvents(ctx, &githubeventpb.ListGitHubEventsRequest{Repository: &repo})
	require.NoError(t, err)
	for _, e := range filtered.GetGithubEvents() {
		assert.Equal(t, repo, e.GetSpec().GetRepository())
	}
}

func TestGitHubEventService_DeleteGitHubEvent(t *testing.T) {
	c := githubEventClient(t)
	ctx := context.Background()

	resp, err := c.CreateGitHubEvent(ctx, &githubeventpb.CreateGitHubEventRequest{Spec: newPushGitHubEventSpec()})
	require.NoError(t, err)
	name := resp.GetGithubEvent().GetMetadata().GetName()

	delResp, err := c.DeleteGitHubEvent(ctx, &githubeventpb.DeleteGitHubEventRequest{Name: name})
	require.NoError(t, err)
	assert.True(t, delResp.GetSuccess())

	_, err = c.GetGitHubEvent(ctx, &githubeventpb.GetGitHubEventRequest{Name: name})
	assert.Error(t, err, "expected the deleted event to be gone")
}

func TestGitHubEventService_WatchGitHubEvent(t *testing.T) {
	c := githubEventClient(t)
	created := createTestGitHubEvent(t, c, newPushGitHubEventSpec())
	name := created.GetMetadata().GetName()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := c.WatchGitHubEvent(ctx, &githubeventpb.WatchGitHubEventRequest{Name: name})
	require.NoError(t, err)

	// GitHubEventService has no Update/Patch RPC (events are immutable) —
	// drive the accepted/triggered status transition directly against
	// the cluster, same as the other watch tests, since there's no RPC
	// path to do it and no controller running to do it for us.
	go func() {
		time.Sleep(200 * time.Millisecond)

		cr := &eventsv1alpha1.GitHubEvent{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: testNamespace}, cr); err != nil {
			return
		}
		st, err := githubEventStatusFromRaw(cr.Status.Contract)
		if err != nil {
			return
		}
		st.Accepted = true
		st.Triggered = true
		raw, err := protojson.Marshal(st)
		if err != nil {
			return
		}
		cr.Status.Contract = runtime.RawExtension{Raw: raw}
		_ = k8sClient.Status().Update(ctx, cr)
	}()

	var sawTriggered bool
	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}
		if event.GetGithubEvent().GetStatus().GetTriggered() {
			sawTriggered = true
			cancel()
			break
		}
	}
	assert.True(t, sawTriggered, "expected WatchGitHubEvent to deliver the triggered transition")
}

func TestGitHubEventService_PullRequest(t *testing.T) {
	c := githubEventClient(t)

	event := createTestGitHubEvent(t, c, newPullRequestGitHubEventSpec())

	assert.Equal(t, commonv1.GitHubEventType_GIT_HUB_EVENT_TYPE_PULL_REQUEST, event.GetSpec().GetEventType().GetType())
	assert.Equal(t, "refs/pull/42/head", event.GetSpec().GetRef())
}
