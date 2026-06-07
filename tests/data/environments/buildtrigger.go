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

package environments

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=environments.blanketops.dev

// BuildTriggerData is the test/data representation of a BuildTrigger CR.
// Mirrors the BuildTrigger API type used in the environments controller.
// Used to construct test fixtures without importing the full API type.
//
// BuildTrigger is immutable once created — it is an event record.
// No Update or Patch fixtures are provided.
type BuildTriggerData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildTriggerSpec   `json:"spec,omitempty"`
	Status BuildTriggerStatus `json:"status,omitempty"`
}

// BuildTriggerSpec is the desired state of a BuildTrigger.
// Mirrors environments.blanketops.dev/v1 BuildTriggerSpec.
type BuildTriggerSpec struct {
	// Where the trigger event originated.
	// Determines which payload format and auth to expect.
	Source BuildTriggerSource `json:"source"`

	// The type of Git event that fired this trigger.
	EventType BuildTriggerEventType `json:"eventType"`

	// Repository that produced the event.
	Repository Repository `json:"repository"`

	// Git ref that the event targets.
	// e.g. refs/heads/main, refs/pull/42/merge
	Ref string `json:"ref"`

	// Full commit SHA associated with this event.
	// Used to pin the exact source revision for the BuildRun.
	SHA string `json:"sha,omitempty"`

	// GitHub username or bot name that caused the event.
	// Recorded for audit purposes.
	Actor string `json:"actor,omitempty"`

	// Unique identifier assigned by the source platform (e.g. GitHub delivery ID).
	// Used for idempotency — duplicate EventIDs are rejected.
	EventID string `json:"eventId,omitempty"`

	// Reference to the Build CR this trigger targets.
	BuildRef BuildRef `json:"buildRef"`

	// Controls whether the trigger is allowed to dispatch a BuildRun.
	PayloadPolicy PayloadPolicy `json:"payloadPolicy,omitempty"`
}

// Repository is the source repository that produced the trigger event.
type Repository struct {
	// Repository owner — GitHub org or user.
	// e.g. ntlaletsi70
	Owner string `json:"owner"`

	// Repository name — without the owner prefix.
	// e.g. blanketops-app
	Name string `json:"name"`
}

// BuildRef references the Build CR that this trigger targets.
type BuildRef struct {
	// Name of the Build CR in the same namespace.
	// Must exist before the trigger is processed.
	Name string `json:"name"`
}

// PayloadPolicy controls whether the trigger is permitted to dispatch a BuildRun.
type PayloadPolicy struct {
	// When true — trigger is accepted and a BuildRun is dispatched.
	// When false — trigger is received but gated; no BuildRun is created.
	// Useful for dry-run or approval-gated pipelines.
	Allow bool `json:"allow"`
}

// BuildTriggerSource is where the trigger event originated.
type BuildTriggerSource string

const (
	// BuildTriggerSourceGitHub — GitHub webhook payload.
	// Validated via HMAC secret from the environment SecretStore.
	BuildTriggerSourceGitHub BuildTriggerSource = "GitHub"

	// BuildTriggerSourceGitLab — GitLab webhook payload. Future support.
	BuildTriggerSourceGitLab BuildTriggerSource = "GitLab"

	// BuildTriggerSourceManual — fired via TriggerBuild RPC or CLI.
	// No payload validation required.
	BuildTriggerSourceManual BuildTriggerSource = "Manual"
)

// BuildTriggerEventType is the Git event type that fired the trigger.
type BuildTriggerEventType string

const (
	// BuildTriggerEventTypePush — push to a branch or tag.
	BuildTriggerEventTypePush BuildTriggerEventType = "Push"

	// BuildTriggerEventTypePullRequest — PR opened, synchronized, or reopened.
	BuildTriggerEventTypePullRequest BuildTriggerEventType = "PullRequest"

	// BuildTriggerEventTypeManual — manually triggered.
	// Always accepted regardless of Build policy.
	BuildTriggerEventTypeManual BuildTriggerEventType = "Manual"
)

// BuildTriggerStatus is the observed state of a BuildTrigger.
type BuildTriggerStatus struct {
	// Current lifecycle phase.
	Phase BuildTriggerPhase `json:"phase,omitempty"`

	// Human-readable reason for the current phase.
	// Populated on Rejected — describes which policy condition failed.
	Reason string `json:"reason,omitempty"`

	// Name of the controller that accepted and processed the trigger.
	AcceptedBy string `json:"acceptedBy,omitempty"`

	// Name of the Shipwright BuildRun emitted as a result of this trigger.
	// Empty if phase is Pending or Rejected.
	ExecutionRef string `json:"executionRef,omitempty"`

	// Human-readable status message. Populated on failure or rejection.
	Message string `json:"message,omitempty"`

	// When the controller last processed this trigger.
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
}

// BuildTriggerPhase is the lifecycle phase of a BuildTrigger.
type BuildTriggerPhase string

const (
	// BuildTriggerPhasePending — received, policy evaluation in progress.
	BuildTriggerPhasePending BuildTriggerPhase = "Pending"

	// BuildTriggerPhaseAccepted — policy matched, BuildRun dispatched.
	BuildTriggerPhaseAccepted BuildTriggerPhase = "Accepted"

	// BuildTriggerPhaseRejected — policy did not match, no BuildRun dispatched.
	BuildTriggerPhaseRejected BuildTriggerPhase = "Rejected"

	// BuildTriggerPhaseConsumed — BuildRun completed. Terminal phase.
	BuildTriggerPhaseConsumed BuildTriggerPhase = "Consumed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewBuildTriggerData returns a BuildTriggerData with sane defaults for use in tests.
// Represents a push event from GitHub targeting build-for-kaniko-sample.
func NewBuildTriggerData(name, namespace string) *BuildTriggerData {
	return &BuildTriggerData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1",
			Kind:       "BuildTrigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":                       "blanketops-environments",
		        "app.kubernetes.io/part-of":                    "blanketops",
		        "app.kubernetes.io/managed-by":                 "kustomize",
		        "app.kubernetes.io/version":                    "v0.0.1",
	            "environments.blanketops.dev/name":             envName,
		        "environments.blanketops.dev/type":             envType,
		        "environments.blanketops.dev/api-version":      "v0.1.4",
		        "environments.blanketops.dev/contract-version": "v0.1.7",
		        "environments.blanketops.dev/controller-version": "v0.2.3",
		        "environments.blanketops.dev/operator-version": "v0.2.1",
			},
		},
		Spec: BuildTriggerSpec{
			Source:    BuildTriggerSourceGitHub,
			EventType: BuildTriggerEventTypePush,
			Repository: Repository{
				Owner: "ntlaletsi70",
				Name:  "blanketops-app",
			},
			Ref:     "refs/heads/main",
			SHA:     "abc1234567890def",
			Actor:   "ntlaletsi70",
			EventID: "github-delivery-abc123",
			BuildRef: BuildRef{
				Name: "build-for-kaniko-sample",
			},
			PayloadPolicy: PayloadPolicy{
				Allow: true,
			},
		},
	}
}

// NewPullRequestBuildTriggerData returns a BuildTriggerData for a pull_request event.
func NewPullRequestBuildTriggerData(name, namespace string) *BuildTriggerData {
	bt := NewBuildTriggerData(name, namespace)
	bt.Spec.EventType = BuildTriggerEventTypePullRequest
	bt.Spec.Ref = "refs/pull/42/merge"
	bt.Spec.SHA = ""
	bt.Spec.EventID = "github-delivery-pr-42"
	return bt
}

// NewRejectedBuildTriggerData returns a BuildTriggerData with a gated PayloadPolicy.
// Simulates a trigger received but not allowed to dispatch a BuildRun.
func NewRejectedBuildTriggerData(name, namespace string) *BuildTriggerData {
	bt := NewBuildTriggerData(name, namespace)
	bt.Spec.PayloadPolicy = PayloadPolicy{Allow: false}
	bt.Status = BuildTriggerStatus{
		Phase:   BuildTriggerPhaseRejected,
		Reason:  "payload policy allow=false — no BuildRun dispatched",
		Message: "trigger received but gated by payload policy",
	}
	return bt
}

// NewManualBuildTriggerData returns a BuildTriggerData for a manual trigger.
// Always accepted regardless of Build policy.
func NewManualBuildTriggerData(name, namespace string) *BuildTriggerData {
	bt := NewBuildTriggerData(name, namespace)
	bt.Spec.Source = BuildTriggerSourceManual
	bt.Spec.EventType = BuildTriggerEventTypeManual
	bt.Spec.SHA = ""
	bt.Spec.EventID = ""
	bt.Spec.Actor = "platform-operator"
	return bt
}