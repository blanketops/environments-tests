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

// GitHubEventService gRPC tests
// APIVersion: events.blanketops.dev/v1alpha1
// Proto:      blanketops/events/v1alpha1/githubevent.proto
// Service:    GitHubEventService
//
// GitHubEvent CRs live in the argo-events namespace.
// Controller reconciles EventSource + EventBus + Sensor as owned children.

func TestGitHubEventService_CreateGitHubEvent(t *testing.T) {
	t.Skip("wire: GitHubEventService.CreateGitHubEvent — real gRPC server required")
	// d := testdata.NewGitHubEventData("grpc-ge-push", "for-kaniko-app")
	// resp, err := client.CreateGitHubEvent(ctx, &v1alpha1.CreateGitHubEventRequest{Spec: toProtoSpec(d)})
	// require.NoError(t, err)
	// assert.Equal(t, "push", string(resp.GitHubEvent.Spec.Contract.EventType))
	// assert.Equal(t, "github-webhook-secret", resp.GitHubEvent.Spec.Contract.Webhook.SecretRef.Name)
}

func TestGitHubEventService_GetGitHubEvent(t *testing.T) {
	t.Skip("wire: GitHubEventService.GetGitHubEvent — real gRPC server required")
}

func TestGitHubEventService_ListGitHubEvents(t *testing.T) {
	t.Skip("wire: GitHubEventService.ListGitHubEvents — filter by eventType and repository")
}

func TestGitHubEventService_DeleteGitHubEvent(t *testing.T) {
	t.Skip("wire: GitHubEventService.DeleteGitHubEvent — cascades to EventSource + Sensor")
}

func TestGitHubEventService_WatchGitHubEvent(t *testing.T) {
	t.Skip("wire: GitHubEventService.WatchGitHubEvent — streaming phase transitions")
	// Pending → Ready when EventSource + Sensor reconciled
}

func TestGitHubEventService_PullRequest(t *testing.T) {
	t.Skip("wire: GitHubEventService.CreateGitHubEvent — pull_request event type")
	// d := testdata.NewPullRequestGitHubEventData("grpc-ge-pr", "for-kaniko-app")
}
