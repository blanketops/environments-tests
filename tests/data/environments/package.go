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
// Its fields and JSON names follow environments-contract's PackageSpec
// (blanketops/environments/v1/package.proto); the engine parses it strictly
// and rejects unknown fields.
type PackageContract struct {
	// Whether this package is active and should be reconciled.
	// Set to false to pause reconciliation without deleting the CR.
	Enabled bool `json:"enabled"`

	// Logical name of the package — may differ from metadata.name.
	// e.g. for-kaniko-app
	Name string `json:"name"`

	// Version identifier of the package contents.
	// e.g. v1.2.3
	Version string `json:"version"`

	// Package maintainers, for provenance and contact.
	Maintainers []PackageMaintainer `json:"maintainers,omitempty"`

	// Human-readable description of the package contents.
	Description string `json:"description,omitempty"`

	// Source repository containing the package definitions.
	Repository PackageRepository `json:"repository"`

	// Enable diff preview (kapp) during reconciliation; the diff output is
	// recorded in status for audit.
	DiffEnabled bool `json:"diffEnabled"`

	// Repository where rendered/applied state is stored — the FluxCD
	// reconciliation source of truth.
	StateRepository StateRepository `json:"stateRepository"`
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
	// SSH or HTTPS URL of the package repository.
	// SSH format: git@github.com:<owner>/<repo>-packages.git
	URL string `json:"url"`

	// Name of the Kubernetes Secret containing repository credentials,
	// sourced from the environment SecretStore via ExternalSecret.
	// e.g. git-ssh-credentials-packages
	CredentialsSecret string `json:"credentialsSecret"`

	// Git ref to apply — branch, tag, or commit SHA. The repository is not
	// fetched without one. A branch is written with its remote, e.g. origin/main.
	Ref string `json:"ref,omitempty"`

	// Path inside the repository that holds the package definitions.
	// Empty means the repository root.
	Path string `json:"path,omitempty"`
}

// StateRepository is the GitOps state repository where rendered manifests
// are committed.
type StateRepository struct {
	// SSH or HTTPS URL of the state repository.
	// SSH format: git@github.com:<owner>/<repo>-state.git
	URL string `json:"url"`

	// Git ref to reconcile against — branch, tag, or commit SHA.
	// e.g. master, main
	Ref string `json:"ref"`

	// Name of the Kubernetes Secret used to authenticate the clone.
	// e.g. git-ssh-credentials-state
	CloneSecret string `json:"cloneSecret"`

	// Application strategy for the state contents.
	// e.g. kustomization
	Strategy string `json:"strategy"`

	// Path inside the repository to reconcile.
	// e.g. ./clusters/dev, ./overlays/dev
	Path string `json:"path"`
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
	// Populated when diffEnabled is true. Used for audit and debug.
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

// Test fixtures

// NewPackageData returns a PackageData matching the canonical
// for-kaniko-app dev package — kapp-diff enabled, kustomization strategy.
// Matches: environments.blanketops.dev/v1 Package for-kaniko-app
func NewPackageData(name, namespace, envName string) *PackageData {
	return &PackageData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1",
			Kind:       "Package",
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
				"environments.blanketops.dev/type":               "dev",
				"environments.blanketops.dev/api-version":        "v0.1.4",
				"environments.blanketops.dev/contract-version":   "v0.1.7",
				"environments.blanketops.dev/controller-version": "v0.2.3",
				"environments.blanketops.dev/operator-version":   "v0.2.1",
			},
		},
		Spec: PackageSpec{
			Contract: PackageContract{
				Enabled:     true,
				Name:        name,
				Version:     "v1.2.3",
				Description: "This package contains the API deployment manifests and runtime configuration.",
				Maintainers: []PackageMaintainer{
					{
						Name:  "Neo",
						Email: "neo@blanketops.online",
					},
				},
				Repository: PackageRepository{
					URL:               "git@github.com:ntlaletsi70/" + name + "-packages.git",
					CredentialsSecret: "git-ssh-credentials-packages",
					Ref:               "origin/main",
				},
				DiffEnabled: true,
				StateRepository: StateRepository{
					URL:         "git@github.com:ntlaletsi70/" + name + "-state.git",
					Ref:         "master",
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
func NewDisabledPackageData(name, namespace, envName string) *PackageData {
	p := NewPackageData(name, namespace, envName)
	p.Spec.Contract.Enabled = false
	return p
}

// NewNoDiffPackageData returns a PackageData with diffEnabled=false.
// Simulates a package that applies without a diff preview step.
func NewNoDiffPackageData(name, namespace, envName string) *PackageData {
	p := NewPackageData(name, namespace, envName)
	p.Spec.Contract.DiffEnabled = false
	return p
}

// NewStagingPackageData returns a PackageData targeting a staging state repository.
// Uses a staging branch and overlay path.
func NewStagingPackageData(name, namespace, envName string) *PackageData {
	p := NewPackageData(name, namespace, envName)
	p.Labels["environments.blanketops.dev/type"] = "staging"
	p.Spec.Contract.StateRepository.Ref = "main"
	p.Spec.Contract.StateRepository.Path = "./clusters/staging"
	return p
}
