/*
Copyright 2026 The BlanketOps Authors.
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

// BuildData is the test/data representation of a Build CR.
// Mirrors the Build API type used in the environments controller.
// Used to construct test fixtures without importing the full API type.
type BuildData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec,omitempty"`
	Status BuildStatus `json:"status,omitempty"`
}

// BuildSpec is the desired state of a Build.
// Mirrors environments.blanketops.dev/v1alpha1 BuildSpec.
type BuildSpec struct {
	// Fully qualified target image — registry/repo/app:tag.
	// e.g. docker.io/nkanyezisolutions/for-kaniko-app:main
	Image string `json:"image"`

	// Shipwright BuildStrategy reference.
	Strategy Strategy `json:"strategy"`

	// Git source to clone and build from.
	Source Source `json:"source"`

	// Kubernetes ServiceAccount used by the build pod.
	ServiceAccount ServiceAccount `json:"serviceAccount"`

	// Trigger and retry policy for this build.
	Policy BuildPolicy `json:"policy,omitempty"`

	// Additional parameters passed to the BuildStrategy.
	Params []Param `json:"params,omitempty"`
}

// Strategy references a Shipwright ClusterBuildStrategy or namespaced BuildStrategy.
type Strategy struct {
	// Name of the Shipwright BuildStrategy.
	// Known values: kaniko | buildah-shipwright-managed-push | buildpacks-v3
	Name string `json:"name"`

	// Kind — ClusterBuildStrategy or BuildStrategy.
	Kind StrategyKind `json:"kind,omitempty"`
}

// StrategyKind is the scope of the BuildStrategy resource.
type StrategyKind string

const (
	// ClusterBuildStrategy — available cluster-wide. Standard for BlanketOps.
	StrategyKindCluster StrategyKind = "ClusterBuildStrategy"

	// BuildStrategy — scoped to a specific namespace.
	StrategyKindNamespaced StrategyKind = "BuildStrategy"
)

// Source is the Git source configuration for a Build.
type Source struct {
	// SSH or HTTPS URL of the Git repository.
	// SSH format: git@github.com:<owner>/<repo>.git
	URL string `json:"url"`

	// Branch, tag, or commit SHA to build from.
	// e.g. main, refs/heads/main, abc1234
	Revision string `json:"revision"`

	// Path within the repository to use as the build context.
	// Required for kaniko. Defaults to repository root if empty.
	ContextDir string `json:"contextDir,omitempty"`

	// Clone depth. 0 means full history.
	// Set to 1 for faster CI builds.
	Depth uint32 `json:"depth,omitempty"`

	// Whether to verify the Git server TLS certificate.
	// Set to false only for internal servers with self-signed certs.
	SSLVerify bool `json:"sslVerify,omitempty"`

	// Name of the Kubernetes Secret containing the SSH private key.
	// Sourced from the environment SecretStore via ExternalSecret.
	CloneSecret string `json:"cloneSecret"`
}

// ServiceAccount is the Kubernetes ServiceAccount used by the build pod.
type ServiceAccount struct {
	// Name of the Kubernetes ServiceAccount.
	Name string `json:"name"`

	// Name of the Kubernetes Secret containing registry credentials.
	// Must be of type kubernetes.io/dockerconfigjson.
	Secret string `json:"secret"`
}

// BuildPolicy defines trigger and retry behaviour for a Build.
type BuildPolicy struct {
	// Events permitted to trigger a BuildRun.
	AllowedTriggers []BuildTriggerPolicy `json:"allowedTriggers,omitempty"`

	// Retry behaviour on BuildRun failure.
	Retry RetryPolicy `json:"retry,omitempty"`
}

// BuildTriggerPolicy is a single permitted trigger type.
type BuildTriggerPolicy struct {
	// Event type that permits a BuildRun to be triggered.
	Type TriggerType `json:"type"`
}

// TriggerType is the event type that may trigger a Build.
type TriggerType string

const (
	TriggerTypeManual      TriggerType = "Manual"
	TriggerTypePush        TriggerType = "Push"
	TriggerTypePullRequest TriggerType = "PullRequest"
	TriggerTypeSchedule    TriggerType = "Schedule"
)

// RetryPolicy defines retry behaviour on BuildRun failure.
type RetryPolicy struct {
	// Whether to automatically retry a failed BuildRun.
	OnFailure bool `json:"onFailure"`

	// Maximum number of retry attempts before marking the Build failed.
	MaxAttempts uint32 `json:"maxAttempts"`
}

// Param is a key-value parameter passed to the BuildStrategy.
type Param struct {
	// Parameter name — must match a param declared in the BuildStrategy.
	Name string `json:"name"`

	// Parameter value — overrides strategy defaults.
	Value string `json:"value"`
}

// BuildStatus is the observed state of a Build.
type BuildStatus struct {
	// Current lifecycle phase.
	Phase BuildPhase `json:"phase,omitempty"`

	// Fully qualified image reference including digest, once pushed.
	Image string `json:"image,omitempty"`

	// Name of the Shipwright BuildRun that last executed this Build.
	ExecutionRef string `json:"executionRef,omitempty"`

	// Human-readable status message. Populated on failure.
	Message string `json:"message,omitempty"`

	// When the most recent BuildRun started.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// When the most recent BuildRun completed (success or failure).
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// BuildPhase is the lifecycle phase of a Build.
type BuildPhase string

const (
	BuildPhasePending   BuildPhase = "Pending"
	BuildPhaseRunning   BuildPhase = "Running"
	BuildPhaseSucceeded BuildPhase = "Succeeded"
	BuildPhaseFailed    BuildPhase = "Failed"
	BuildPhaseCancelled BuildPhase = "Cancelled"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewBuildData returns a BuildData with sane defaults for use in tests.
// Override fields as needed for specific test cases.
func NewBuildData(name, namespace string) *BuildData {
	return &BuildData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1alpha1",
			Kind:       "Build",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":                         "blanketops-environments",
				"app.kubernetes.io/part-of":                      "blanketops",
				"app.kubernetes.io/managed-by":                   "kustomize",
				"app.kubernetes.io/version":                      "v0.0.1",
				"environments.blanketops.dev/name":               envName,
				"environments.blanketops.dev/type":               envType,
				"environments.blanketops.dev/api-version":        "v0.1.4",
				"environments.blanketops.dev/contract-version":   "v0.1.7",
				"environments.blanketops.dev/controller-version": "v0.2.3",
				"environments.blanketops.dev/operator-version":   "v0.2.1",
			},
		},
		Spec: BuildSpec{
			Image: "docker.io/nkanyezisolutions/for-kaniko-app:main",
			Strategy: Strategy{
				Name: "kaniko",
				Kind: StrategyKindCluster,
			},
			Source: Source{
				URL:         "git@github.com:ntlaletsi70/blanketops-app.git",
				Revision:    "main",
				ContextDir:  ".",
				SSLVerify:   true,
				CloneSecret: "github-ssh-credentials",
			},
			ServiceAccount: ServiceAccount{
				Name:   "build-bot",
				Secret: "docker-registry-credentials",
			},
			Policy: BuildPolicy{
				AllowedTriggers: []BuildTriggerPolicy{
					{Type: TriggerTypePush},
					{Type: TriggerTypePullRequest},
				},
				Retry: RetryPolicy{
					OnFailure:   true,
					MaxAttempts: 3,
				},
			},
		},
	}
}

// NewBuildahBuildData returns a BuildData using the buildah-shipwright-managed-push strategy.
func NewBuildahBuildData(name, namespace string) *BuildData {
	b := NewBuildData(name, namespace)
	b.Spec.Strategy = Strategy{
		Name: "buildah-shipwright-managed-push",
		Kind: StrategyKindCluster,
	}
	b.Spec.Source.ContextDir = ""
	return b
}

// NewBuildpacksBuildData returns a BuildData using the buildpacks-v3 strategy.
// No contextDir or dockerfile required — stack is auto-detected.
func NewBuildpacksBuildData(name, namespace string) *BuildData {
	b := NewBuildData(name, namespace)
	b.Spec.Strategy = Strategy{
		Name: "buildpacks-v3",
		Kind: StrategyKindCluster,
	}
	b.Spec.Source.ContextDir = ""
	return b
}
