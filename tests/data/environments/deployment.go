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

// DeploymentData is the test/data representation of a Deployment CR.
// Mirrors the Deployment API type used in the environments controller.
// Used to construct test fixtures without importing the full API type.
//
// APIVersion: environments.blanketops.dev/v1alpha1
type DeploymentData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec,omitempty"`
	Status DeploymentStatus `json:"status,omitempty"`
}

// DeploymentSpec is the desired state of a Deployment.
// Mirrors environments.blanketops.dev/v1alpha1 DeploymentSpec.
type DeploymentSpec struct {
	// Contract is the intent declaration for this deployment.
	// All deployment configuration lives under contract — mirrors
	// the Environment CR pattern.
	Contract DeploymentContract `json:"contract"`
}

// DeploymentContract is the core intent declaration for a Deployment.
type DeploymentContract struct {
	// Names of the ServiceUnit CRs that make up this deployment.
	// Must exist in the same namespace before reconciliation begins.
	// e.g. for-kaniko-app-api, for-kaniko-app-worker
	ServiceUnits []string `json:"serviceUnits"`

	// Target runtime backend for workload execution.
	// e.g. kubernetes.io/container-runtime
	// Knative, ECS, Azure Container planned.
	Runtime DeploymentRuntimeValue `json:"runtime"`

	// Rollout strategy for workload updates.
	// Rolling is the default — Canary and BlueGreen require traffic-splitting ingress.
	Strategy DeploymentStrategyValue `json:"strategy"`

	// When true, the controller watches for new image digests from Build outputs
	// and triggers a rolling update automatically.
	// Set to false for GitOps-only workflows where image updates come via PR.
	ImageAutomation bool `json:"imageAutomation"`

	// How the controller applies manifests from manifestsRepo.
	// kustomize is the default and current supported strategy.
	// helm planned.
	ReconciliationStrategy ReconciliationStrategyValue `json:"reconciliationStrategy"`

	// GitHub org or user that owns the manifests repository.
	// Used for GitOps audit trail and webhook registration.
	// e.g. ntlaletsi70
	GitOwner string `json:"gitOwner"`

	// GitOps repository containing the rendered deployment manifests.
	// Controller reconciles cluster state from this repo on each commit.
	ManifestsRepo ManifestsRepo `json:"manifestsRepo"`
}

// ManifestsRepo is the GitOps repository containing deployment manifests.
type ManifestsRepo struct {
	// SSH or HTTPS URL of the manifests repository.
	// SSH format: ssh://git@github.com/<owner>/<repo>.git
	//             git@github.com:<owner>/<repo>.git
	URL string `json:"url"`

	// Name of the Kubernetes Secret containing SSH private key.
	// Sourced from the environment SecretStore via ExternalSecret.
	// Must have read access to the manifests repository.
	CloneSecret string `json:"cloneSecret"`

	// Manifest rendering strategy within this repo.
	// e.g. kustomization, helm (helm planned)
	Strategy string `json:"strategy"`

	// Path within the repository to the environment overlay.
	// e.g. ./overlays/dev, ./bases/kustomization.yaml
	Path string `json:"path"`
}

// DeploymentRuntimeValue is the target runtime backend for workload execution.
type DeploymentRuntimeValue string

const (
	// DeploymentRuntimeKubernetes — standard Kubernetes workloads (Deployment, Service).
	// Default runtime — currently the only fully supported option.
	DeploymentRuntimeKubernetes DeploymentRuntimeValue = "kubernetes.io/container-runtime"

	// DeploymentRuntimeKnative — Knative Serving.
	// Requires Knative Serving + Kourier installed on the cluster.
	DeploymentRuntimeKnative DeploymentRuntimeValue = "knative.io/serving"

	// DeploymentRuntimeECS — AWS Elastic Container Service. Planned.
	DeploymentRuntimeECS DeploymentRuntimeValue = "aws.amazon.com/ecs"

	// DeploymentRuntimeAzureContainer — Azure Container Apps. Planned.
	DeploymentRuntimeAzureContainer DeploymentRuntimeValue = "azure.microsoft.com/container-apps"
)

// DeploymentStrategyValue is the rollout strategy for workload updates.
type DeploymentStrategyValue string

const (
	// DeploymentStrategyRolling — gradually replaces old pods with new ones.
	// Zero-downtime by default with readiness gates.
	DeploymentStrategyRolling DeploymentStrategyValue = "Rolling"

	// DeploymentStrategyCanary — shifts a percentage of traffic to the new version first.
	// Requires Knative or a traffic-splitting ingress.
	DeploymentStrategyCanary DeploymentStrategyValue = "Canary"

	// DeploymentStrategyBlueGreen — routes all traffic to the new version at once.
	// Old version kept alive until explicitly decommissioned.
	DeploymentStrategyBlueGreen DeploymentStrategyValue = "BlueGreen"
)

// ReconciliationStrategyValue is how the controller applies manifests.
type ReconciliationStrategyValue string

const (
	// ReconciliationStrategyKustomize — renders overlays via kustomize.
	// Controller applies via kubectl apply -k. Default.
	ReconciliationStrategyKustomize ReconciliationStrategyValue = "kustomize"

	// ReconciliationStrategyHelm — renders charts via helm. Planned.
	ReconciliationStrategyHelm ReconciliationStrategyValue = "helm"
)

// DeploymentStatus is the observed state of a Deployment.
type DeploymentStatus struct {
	// Current lifecycle phase.
	Phase DeploymentPhaseValue `json:"phase,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// Runtime backend actually used for this deployment.
	Runtime DeploymentRuntimeValue `json:"runtime,omitempty"`

	// Rollout strategy currently in effect.
	Strategy DeploymentStrategyValue `json:"strategy,omitempty"`

	// Per-ServiceUnit observed status.
	// One entry per name in spec.contract.serviceUnits.
	ServiceUnitStatuses []ServiceUnitDeploymentStatus `json:"serviceUnitStatuses,omitempty"`

	// When the controller last successfully reconciled this Deployment.
	LastUpdatedAt *metav1.Time `json:"lastUpdatedAt,omitempty"`
}

// ServiceUnitDeploymentStatus is the observed state of a single ServiceUnit
// within a Deployment.
type ServiceUnitDeploymentStatus struct {
	// Name of the ServiceUnit CR this status belongs to.
	Name string `json:"name"`

	// Current phase of this ServiceUnit within the deployment.
	Phase ServiceUnitPhaseValue `json:"phase,omitempty"`

	// Fully qualified image reference deployed for this ServiceUnit.
	// Includes digest once the image has been pulled and verified.
	Image string `json:"image,omitempty"`

	// Runtime backend used for this ServiceUnit.
	Runtime DeploymentRuntimeValue `json:"runtime,omitempty"`

	// Human-readable status message for this ServiceUnit.
	Message string `json:"message,omitempty"`

	// Detailed error message if phase is Failed.
	Error string `json:"error,omitempty"`

	// When this ServiceUnit last transitioned to its current phase.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// DeploymentPhaseValue is the lifecycle phase of a Deployment.
type DeploymentPhaseValue string

const (
	DeploymentPhasePending   DeploymentPhaseValue = "Pending"
	DeploymentPhaseDeploying DeploymentPhaseValue = "Deploying"
	DeploymentPhaseReady     DeploymentPhaseValue = "Ready"
	DeploymentPhaseFailed    DeploymentPhaseValue = "Failed"
)

// ServiceUnitPhaseValue is the lifecycle phase of a ServiceUnit within a Deployment.
type ServiceUnitPhaseValue string

const (
	ServiceUnitPhasePending   ServiceUnitPhaseValue = "Pending"
	ServiceUnitPhaseDeploying ServiceUnitPhaseValue = "Deploying"
	ServiceUnitPhaseReady     ServiceUnitPhaseValue = "Ready"
	ServiceUnitPhaseFailed    ServiceUnitPhaseValue = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// NewDeploymentData returns a DeploymentData matching the canonical
// for-kaniko-app dev deployment — single ServiceUnit, Rolling, kustomize.
// Matches: environments.blanketops.dev/v1alpha1 Deployment for-kaniko-app
func NewDeploymentData(name, namespace, envName string) *DeploymentData {
	return &DeploymentData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1alpha1",
			Kind:       "Deployment",
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
		Spec: DeploymentSpec{
			Contract: DeploymentContract{
				ServiceUnits:           []string{name + "-api"},
				Runtime:                DeploymentRuntimeKubernetes,
				Strategy:               DeploymentStrategyRolling,
				ImageAutomation:        false,
				ReconciliationStrategy: ReconciliationStrategyKustomize,
				GitOwner:               "ntlaletsi70",
				ManifestsRepo: ManifestsRepo{
					URL:         "ssh://git@github.com/ntlaletsi70/" + name + "-manifests.git",
					CloneSecret: "git-ssh-credentials",
					Strategy:    "kustomization",
					Path:        "./overlays/dev",
				},
			},
		},
	}
}

// NewMultiUnitDeploymentData returns a DeploymentData with multiple ServiceUnits.
// Matches the commented prod example — api + worker, imageAutomation enabled.
func NewMultiUnitDeploymentData(name, namespace, envName string) *DeploymentData {
	d := NewDeploymentData(name, namespace, envName)
	d.ObjectMeta.Labels["environments.blanketops.dev/type"] = "production"
	d.Spec.Contract.ServiceUnits = []string{name + "-api", name + "-worker"}
	d.Spec.Contract.ImageAutomation = true
	d.Spec.Contract.ManifestsRepo = ManifestsRepo{
		URL:         "git@github.com:ntlaletsi70/" + name + "-manifests.git",
		CloneSecret: "git-ssh-credentials",
		Strategy:    "kustomization",
		Path:        "./bases/kustomization.yaml",
	}
	return d
}

// NewKnativeDeploymentData returns a DeploymentData targeting the Knative runtime.
// Requires Knative Serving + Kourier installed on the cluster.
func NewKnativeDeploymentData(name, namespace, envName string) *DeploymentData {
	d := NewDeploymentData(name, namespace, envName)
	d.Spec.Contract.Runtime = DeploymentRuntimeKnative
	d.Spec.Contract.Strategy = DeploymentStrategyCanary
	return d
}
