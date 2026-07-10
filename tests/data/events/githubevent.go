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

package events

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=events.blanketops.dev

// GitHubEventData is the test/data representation of a GitHubEvent CR.
// Mirrors the GitHubEvent API type used in the events controller.
// Used to construct test fixtures without importing the full API type.
//
// Architectural role:
//   GitHubEvent declares which GitHub webhook events should wake up the pipeline.
//   The controller reconciles child Argo Events resources (EventSource, EventBus,
//   Sensor) as owned resources — cascade deleted when the GitHubEvent is deleted.
//   On a matching event the Sensor fires and the controller creates a BuildTrigger CR.
//
// APIVersion: events.blanketops.dev/v1alpha1
// Namespace:  argo-events (Argo Events controller watches this namespace)
type GitHubEventData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHubEventSpec   `json:"spec,omitempty"`
	Status GitHubEventStatus `json:"status,omitempty"`
}

// GitHubEventSpec is the desired state of a GitHubEvent.
// Mirrors events.blanketops.dev/v1alpha1 GitHubEventSpec.
type GitHubEventSpec struct {
	// Contract is the intent declaration for this event filter.
	Contract GitHubEventContract `json:"contract"`
}

// GitHubEventContract is the core intent declaration for a GitHubEvent.
type GitHubEventContract struct {
	// Repository in owner/name format.
	// Must match exactly — the controller uses this to scope the EventSource.
	// e.g. ntlaletsi70/for-kaniko-app
	Repository string `json:"repository"`

	// Git event type to listen for.
	// Maps to the Argo Events EventSource github.events field.
	EventType GitHubEventType `json:"eventType"`

	// Git ref filter — only events targeting this ref are processed.
	// e.g. refs/heads/main, refs/heads/master, refs/tags/v1.0.0
	Ref string `json:"ref"`

	// Webhook HMAC secret configuration.
	// Used to verify GitHub payload signatures before processing.
	Webhook Webhook `json:"webhook"`
}

// Webhook holds the HMAC secret reference for payload signature verification.
type Webhook struct {
	// Reference to the Kubernetes Secret containing the HMAC webhook secret.
	// Sourced from the environment SecretStore via ExternalSecret.
	SecretRef SecretRef `json:"secretRef"`
}

// SecretRef is a reference to a Kubernetes Secret and a key within it.
type SecretRef struct {
	// Name of the Kubernetes Secret.
	// e.g. github-webhook-secret
	Name string `json:"name"`

	// Key within the Secret that holds the HMAC value.
	// e.g. secret
	Key string `json:"key"`
}

// GitHubEventType is the GitHub webhook event type to listen for.
type GitHubEventType string

const (
	// GitHubEventTypePush — push to a branch or tag.
	// Fires on every commit pushed to the target ref.
	GitHubEventTypePush GitHubEventType = "push"

	// GitHubEventTypePullRequest — pull request opened, synchronized, or reopened.
	// Fires when a PR targets the configured ref.
	GitHubEventTypePullRequest GitHubEventType = "pull_request"
)

// GitHubEventStatus is the observed state of a GitHubEvent.
type GitHubEventStatus struct {
	// Current lifecycle phase.
	Phase GitHubEventPhase `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// Whether the Argo Events EventSource has been reconciled and is active.
	EventSourceReady bool `json:"eventSourceReady,omitempty"`

	// Whether the Argo Events Sensor has been reconciled and is active.
	SensorReady bool `json:"sensorReady,omitempty"`

	// When the controller last successfully reconciled this GitHubEvent.
	LastReconciledAt *metav1.Time `json:"lastReconciledAt,omitempty"`
}

// GitHubEventPhase is the lifecycle phase of a GitHubEvent.
type GitHubEventPhase string

const (
	// GitHubEventPhasePending — waiting for Argo Events resources to become ready.
	GitHubEventPhasePending GitHubEventPhase = "Pending"

	// GitHubEventPhaseReady — EventSource + Sensor active, webhook receiving events.
	GitHubEventPhaseReady GitHubEventPhase = "Ready"

	// GitHubEventPhaseFailed — one or more child resources failed to reconcile.
	GitHubEventPhaseFailed GitHubEventPhase = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewGitHubEventData returns a GitHubEventData matching the canonical
// push-main-001 event — push to refs/heads/main on for-kaniko-app.
// Matches: events.blanketops.dev/v1alpha1 GitHubEvent push-main-001
func NewGitHubEventData(name, envName string) *GitHubEventData {
	return &GitHubEventData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "events.blanketops.dev/v1alpha1",
			Kind:       "GitHubEvent",
		},
		ObjectMeta: metav1.ObjectMeta{
			// GitHubEvent CRs live in argo-events namespace —
			// Argo Events controller watches this namespace.
			Name:      name,
			Namespace: "argo-events",
			Labels: map[string]string{
				"app.kubernetes.io/name":                       "blanketops-environments",
		        "app.kubernetes.io/part-of":                    "blanketops",
		        "app.kubernetes.io/managed-by":                 "kustomize",
		        "app.kubernetes.io/version":                    "v0.0.1",
	            "environments.blanketops.dev/name":             envName,
		        "environments.blanketops.dev/type":             "dev",
		        "environments.blanketops.dev/api-version":      "v0.1.4",
		        "environments.blanketops.dev/contract-version": "v0.1.7",
		        "environments.blanketops.dev/controller-version": "v0.2.3",
		        "environments.blanketops.dev/operator-version": "v0.2.1",
			},
		},
		Spec: GitHubEventSpec{
			Contract: GitHubEventContract{
				Repository: "ntlaletsi70/" + envName,
				EventType:  GitHubEventTypePush,
				Ref:        "refs/heads/main",
				Webhook: Webhook{
					SecretRef: SecretRef{
						Name: "github-webhook-secret",
						Key:  "secret",
					},
				},
			},
		},
	}
}

// NewPullRequestGitHubEventData returns a GitHubEventData for pull_request events.
// Useful for testing PR-gated pipelines.
func NewPullRequestGitHubEventData(name, envName string) *GitHubEventData {
	e := NewGitHubEventData(name, envName)
	e.Spec.Contract.EventType = GitHubEventTypePullRequest
	e.Spec.Contract.Ref = "refs/heads/main"
	return e
}

// NewMasterGitHubEventData returns a GitHubEventData targeting refs/heads/master.
// Matches the BuildTrigger ref used in the for-kaniko-app pipeline.
func NewMasterGitHubEventData(name, envName string) *GitHubEventData {
	e := NewGitHubEventData(name, envName)
	e.Spec.Contract.Ref = "refs/heads/master"
	return e
}