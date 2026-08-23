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

package sources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=sources.blanketops.dev

// GitRepositoryData is the test/data representation of a GitRepository CR.
// Mirrors the GitRepository API type used in the sources controller.
// Used to construct test fixtures without importing the full API type.
//
// Architectural role:
//
//	GitRepository is the source registration CR. The controller reconciles
//	a Crossplane GitHub webhook provider resource as an owned child —
//	registering the webhook on GitHub via the API. On receipt of a webhook
//	payload the cloudflared tunnel routes it into the cluster where the
//	GitHubEvent controller processes it.
//
// APIVersion: sources.blanketops.dev/v1alpha1
type GitRepositoryData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitRepositorySpec   `json:"spec,omitempty"`
	Status GitRepositoryStatus `json:"status,omitempty"`
}

// GitRepositorySpec is the desired state of a GitRepository.
// Mirrors sources.blanketops.dev/v1alpha1 GitRepositorySpec.
type GitRepositorySpec struct {
	// Contract is the intent declaration for this source registration.
	Contract GitRepositoryContract `json:"contract"`
}

// GitRepositoryContract is the core intent declaration for a GitRepository.
type GitRepositoryContract struct {
	// Source control provider.
	// e.g. github — determines which Crossplane provider is used
	// for webhook registration.
	Provider string `json:"provider"`

	// Public URL where GitHub delivers webhook payloads.
	// In dev: cloudflared trycloudflare.com tunnel URL.
	// In production: the EventListener ingress URL.
	// e.g. https://elimination-propecia-meter-strips.trycloudflare.com/
	HookURL string `json:"hookUrl"`

	// Repository to register the webhook on.
	Repository GitRepository `json:"repository"`

	// Webhook event subscriptions to register on the repository.
	// Each entry declares one or more GitHub event types.
	Webhooks []WebhookSubscription `json:"webhooks"`
}

// GitRepository identifies the GitHub repository to register the webhook on.
type GitRepository struct {
	// Repository owner — GitHub org or user.
	// e.g. ntlaletsi70
	Owner string `json:"owner"`

	// Repository name — without the owner prefix.
	// e.g. for-kaniko-app
	Name string `json:"name"`
}

// WebhookSubscription declares a set of GitHub event types to subscribe to.
// Multiple subscriptions can be registered on a single repository.
type WebhookSubscription struct {
	// GitHub event types to subscribe to for this webhook.
	// e.g. push, pull_request
	// Maps to GitHub webhook event names — see GitHub docs for valid values.
	Events []WebhookEvent `json:"events"`
}

// WebhookEvent is a GitHub webhook event type.
type WebhookEvent string

const (
	// WebhookEventPush — fired on every push to the repository.
	WebhookEventPush WebhookEvent = "push"

	// WebhookEventPullRequest — fired when a pull request is opened,
	// synchronized, reopened, or closed.
	WebhookEventPullRequest WebhookEvent = "pull_request"

	// WebhookEventRelease — fired when a release is published. Future use.
	WebhookEventRelease WebhookEvent = "release"

	// WebhookEventWorkflowRun — fired when a GitHub Actions workflow run
	// is completed. Future use.
	WebhookEventWorkflowRun WebhookEvent = "workflow_run"
)

// GitRepositoryStatus is the observed state of a GitRepository.
type GitRepositoryStatus struct {
	// Current lifecycle phase.
	Phase GitRepositoryPhase `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// Whether the webhook has been successfully registered on GitHub.
	WebhookRegistered bool `json:"webhookRegistered,omitempty"`

	// Whether the hookUrl is reachable from GitHub.
	// Verified by the Crossplane provider on registration.
	HookURLReachable bool `json:"hookUrlReachable,omitempty"`

	// GitHub-assigned webhook ID for this registration.
	// Used by the controller to manage webhook lifecycle (update, delete).
	WebhookID string `json:"webhookId,omitempty"`

	// When the controller last successfully reconciled this GitRepository.
	LastReconciledAt *metav1.Time `json:"lastReconciledAt,omitempty"`
}

// GitRepositoryPhase is the lifecycle phase of a GitRepository.
type GitRepositoryPhase string

const (
	// GitRepositoryPhasePending — waiting for Crossplane provider or hookUrl.
	GitRepositoryPhasePending GitRepositoryPhase = "Pending"

	// GitRepositoryPhaseReady — webhook registered and hookUrl reachable.
	GitRepositoryPhaseReady GitRepositoryPhase = "Ready"

	// GitRepositoryPhaseFailed — webhook registration failed.
	// See status.message for the Crossplane provider error.
	GitRepositoryPhaseFailed GitRepositoryPhase = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewGitRepositoryData returns a GitRepositoryData matching the canonical
// for-kaniko-app source registration — push + pull_request events via GitHub.
// Matches: sources.blanketops.dev/v1alpha1 GitRepository for-kaniko-app
func NewGitRepositoryData(name, envName string) *GitRepositoryData {
	return &GitRepositoryData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sources.blanketops.dev/v1alpha1",
			Kind:       "GitRepository",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":                         "blanketops-environments",
				"app.kubernetes.io/part-of":                      "blanketops",
				"app.kubernetes.io/managed-by":                   "kustomize",
				"app.kubernetes.io/version":                      "v0.0.1",
				"environments.blanketops.dev/name":               envName,
				"environments.blanketops.dev/type":               "dev",
				"environments.blanketops.dev/api-version":        "v0.1.4",
				"environments.blanketops.dev/contract-version":   "v0.1.7",
				"environments.blanketops.dev/controller-version": "v0.2.3",
				"environments.blanketops.dev/operator-version":   "v0.2.1",
			},
		},
		Spec: GitRepositorySpec{
			Contract: GitRepositoryContract{
				Provider: "github",
				HookURL:  "https://elimination-propecia-meter-strips.trycloudflare.com/",
				Repository: GitRepository{
					Owner: "ntlaletsi70",
					Name:  name,
				},
				Webhooks: []WebhookSubscription{
					{
						Events: []WebhookEvent{
							WebhookEventPush,
							WebhookEventPullRequest,
						},
					},
				},
			},
		},
	}
}

// NewPushOnlyGitRepositoryData returns a GitRepositoryData subscribed to
// push events only. Useful for testing push-gated pipelines.
func NewPushOnlyGitRepositoryData(name, envName string) *GitRepositoryData {
	r := NewGitRepositoryData(name, envName)
	r.Spec.Contract.Webhooks = []WebhookSubscription{
		{Events: []WebhookEvent{WebhookEventPush}},
	}
	return r
}

// NewGitRepositoryDataWithHookURL returns a GitRepositoryData with a custom
// hookUrl. Useful for testing production ingress URLs vs dev tunnel URLs.
func NewGitRepositoryDataWithHookURL(name, hookURL, envName string) *GitRepositoryData {
	r := NewGitRepositoryData(name, envName)
	r.Spec.Contract.HookURL = hookURL
	return r
}
