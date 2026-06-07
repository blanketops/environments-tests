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

// PackageData is the test/data representation of a Package CR.
// Mirrors the Package API type used in the environments controller.
// Used to construct test fixtures without importing the full API type.
//
// APIVersion: environments.blanketops.dev/v1
type PackageData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
	Status PackageStatus `json:"status,omitempty"`
}

// PackageSpec is the desired state of a Package.
// Mirrors environments.blanketops.dev/v1 PackageSpec.
type PackageSpec struct {
	// Contract is the intent declaration for this package.
	// All package configuration lives under contract — mirrors
	// the Environment and Deployment CR pattern.
	Contract PackageContract `json:"contract"`
}

// PackageContract is the core intent declaration for a Package.
type PackageContract struct {
	// Logical name of the package.
	// Typically matches the application name.
	// e.g. for-kaniko-app
	Name string `json:"name"`

	// Semantic version identifier for this package release.
	// e.g. v1.0.0
	Version string `json:"version"`

	// Whether this package is active and should be reconciled.
	// Set to false to pause reconciliation without deleting the CR.
	Enabled bool `json:"enabled"`

	// Package artifact name — used in state repository commit messages
	// and audit trail. Typically matches Name.
	PackageName string `json:"packageName"`

	// Package artifact version — tracks the rendered artifact version
	// independently from the logical version above.
	// e.g. v1.2.3
	PackageVersion string `json:"packageVersion"`

	// Human-readable description of what this package contains.
	// e.g. "This package contains the API deployment manifests and runtime configuration."
	PackageDescription string `json:"packageDescription,omitempty"`

	// Individuals or teams responsible for this package.
	// Recorded in the package manifest and audit trail.
	PackageMaintainers []PackageMaintainer `json:"packageMaintainers,omitempty"`

	// Source repository containing the package definitions —
	// kustomize overlays, helm charts, or raw manifests.
	// Credentials sourced from the environment SecretStore.
	PackageRepository PackageRepository `json:"packageRepository"`

	// Whether to run kapp-diff before committing to the state repository.
	// When true — controller previews changes and records the diff in status.
	// Prevents unexpected state mutations on large packages.
	PackageKappDiff bool `json:"packageKappDiff"`

	// GitOps state repository where rendered manifests are committed.
	// Represents the observed truth — what is actually applied to the cluster.
	// Separate from packageRepository which holds declared intent.
	StateRepo StateRepo `json:"stateRepo"`
}

// PackageMaintainer is an individual or team responsible for this package.
type PackageMaintainer struct {
	// Full name of the maintainer.
	// e.g. Neo
	Name string `json:"name"`

	// Contact email address for this maintainer.
	// e.g. neo@blanketops.online
	Email string `json:"email"`
}

// PackageRepository is the source repository containing package definitions.
type PackageRepository struct {
	// SSH URL of the package source repository.
	// SSH format: git@github.com:<owner>/<repo>-packages.git
	URL string `json:"url"`

	// Name of the Kubernetes Secret containing repository credentials.
	// Sourced from the environment SecretStore via ExternalSecret.
	// Must have read access to the package source repository.
	// e.g. git-ssh-credentials-packages
	CredentialsSecret string `json:"credentialsSecret"`
}

// StateRepo is the GitOps state repository where rendered manifests are committed.
type StateRepo struct {
	// SSH URL of the GitOps state repository.
	// SSH format: git@github.com:<owner>/<repo>-state.git
	// Controller commits rendered manifests here after each reconciliation.
	URL string `json:"url"`

	// Git reference configuration for the state repository.
	Ref StateRepoRef `json:"ref"`

	// Name of the Kubernetes Secret containing SSH private key.
	// Sourced from the environment SecretStore via ExternalSecret.
	// Must have write access to the state repository.
	// e.g. git-ssh-credentials-state
	CloneSecret string `json:"cloneSecret"`

	// Manifest rendering strategy for this state repository.
	// kustomization is the default — helm planned.
	Strategy string `json:"strategy"`

	// Path within the state repository to write rendered manifests.
	// e.g. ./clusters/dev, ./overlays/dev
	Path string `json:"path"`
}

// StateRepoRef is the Git reference configuration for the state repository.
type StateRepoRef struct {
	// Branch to commit rendered state to.
	// e.g. master, main
	Branch string `json:"branch"`
}

// PackageStatus is the observed state of a Package.
type PackageStatus struct {
	// Current lifecycle phase.
	Phase PackagePhaseValue `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// When the controller last attempted to reconcile this Package.
	LastReconciledAt *metav1.Time `json:"lastReconciledAt,omitempty"`

	// Whether the last reconciliation resulted in a diff being applied
	// to the state repository. False if state was already up to date.
	DiffApplied bool `json:"diffApplied,omitempty"`

	// Raw kapp-diff output from the last reconciliation.
	// Populated when packageKappDiff is true. Used for audit and debug.
	// Cleared on each new reconciliation attempt.
	DiffOutput string `json:"diffOutput,omitempty"`

	// The source-of-truth state as last committed to the state repository.
	State PackageState `json:"state,omitempty"`
}

// PackageState is the applied state as committed to the state repository.
type PackageState struct {
	// URL of the state repository where rendered manifests were committed.
	Repository string `json:"repository,omitempty"`

	// Git SHA of the last commit written to the state repository.
	// Use this to correlate state repository history with Package reconciliations.
	Revision string `json:"revision,omitempty"`
}

// PackagePhaseValue is the lifecycle phase of a Package.
type PackagePhaseValue string

const (
	// PackagePhasePending — waiting for the source repository or upstream Build.
	PackagePhasePending PackagePhaseValue = "Pending"

	// PackagePhaseReady — manifests rendered and committed to state repository.
	PackagePhaseReady PackagePhaseValue = "Ready"

	// PackagePhaseFailed — reconciliation failed. See status.message.
	PackagePhaseFailed PackagePhaseValue = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewPackageData returns a PackageData matching the canonical
// for-kaniko-app dev package — kapp-diff enabled, kustomization strategy.
// Matches: environments.blanketops.dev/v1 Package for-kaniko-app
func NewPackageData(name, namespace string) *PackageData {
	return &PackageData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1",
			Kind:       "Package",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"environments.blanketops.dev/name":             name,
				"environments.blanketops.dev/type":             "dev",
				"environments.blanketops.dev/api-version":      "v0.1.4",
				"environments.blanketops.dev/contract-version": "v0.1.7",
			},
		},
		Spec: PackageSpec{
			Contract: PackageContract{
				Name:               name,
				Version:            "v1.0.0",
				Enabled:            true,
				PackageName:        name,
				PackageVersion:     "v1.2.3",
				PackageDescription: "This package contains the API deployment manifests and runtime configuration.",
				PackageMaintainers: []PackageMaintainer{
					{
						Name:  "Neo",
						Email: "neo@blanketops.online",
					},
				},
				PackageRepository: PackageRepository{
					URL:               "git@github.com:ntlaletsi70/" + name + "-packages.git",
					CredentialsSecret: "git-ssh-credentials-packages",
				},
				PackageKappDiff: true,
				StateRepo: StateRepo{
					URL: "git@github.com:ntlaletsi70/" + name + "-state.git",
					Ref: StateRepoRef{
						Branch: "master",
					},
					CloneSecret: "git-ssh-credentials-state",
					Strategy:    "kustomization",
					Path:        "./clusters/dev",
				},
			},
		},
	}
}

// NewDisabledPackageData returns a PackageData with enabled=false.
// Simulates a paused package — reconciliation suspended without deletion.
func NewDisabledPackageData(name, namespace string) *PackageData {
	p := NewPackageData(name, namespace)
	p.Spec.Contract.Enabled = false
	return p
}

// NewNoDiffPackageData returns a PackageData with packageKappDiff=false.
// Simulates a package that applies without a diff preview step.
func NewNoDiffPackageData(name, namespace string) *PackageData {
	p := NewPackageData(name, namespace)
	p.Spec.Contract.PackageKappDiff = false
	return p
}

// NewStagingPackageData returns a PackageData targeting a staging state repository.
// Uses a staging branch and overlay path.
func NewStagingPackageData(name, namespace string) *PackageData {
	p := NewPackageData(name, namespace)
	p.ObjectMeta.Labels["environments.blanketops.dev/type"] = "staging"
	p.Spec.Contract.StateRepo.Ref = StateRepoRef{Branch: "main"}
	p.Spec.Contract.StateRepo.Path = "./clusters/staging"
	return p
}