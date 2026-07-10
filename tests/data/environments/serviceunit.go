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

// ServiceUnitData is the test/data representation of a ServiceUnit CR.
// Mirrors the ServiceUnit API type used in the environments controller.
// Used to construct test fixtures without importing the full API type.
//
// Two flavours:
//   - static: image provided directly — no Build CR dependency
//   - build:  image resolved from a Build CR's BuildRun output
//
// APIVersion: environments.blanketops.dev/v1alpha1
type ServiceUnitData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceUnitSpec   `json:"spec,omitempty"`
	Status ServiceUnitStatus `json:"status,omitempty"`
}

// ServiceUnitSpec is the desired state of a ServiceUnit.
// Mirrors environments.blanketops.dev/v1alpha1 ServiceUnitSpec.
type ServiceUnitSpec struct {
	// Contract is the intent declaration for this service unit.
	// All configuration lives under contract — mirrors the
	// Environment, Deployment, and Package CR pattern.
	Contract ServiceUnitContract `json:"contract"`
}

// ServiceUnitContract is the core intent declaration for a ServiceUnit.
type ServiceUnitContract struct {
	// Flavour of this ServiceUnit — determines how the image is sourced.
	//   static: image field required, no Build CR dependency
	//   build:  buildRef required, image resolved from BuildRun output
	Type ServiceUnitType `json:"type"`

	// Fully qualified image reference — required when type is static.
	// e.g. docker.io/nkanyezisolutions/for-kaniko-app:master
	// Ignored when type is build.
	Image string `json:"image,omitempty"`

	// Reference to the Build CR — required when type is build.
	// Controller resolves the image from the Build's latest BuildRun output.
	// Ignored when type is static.
	BuildRef *ServiceUnitBuildRef `json:"buildRef,omitempty"`

	// Container port exposed by this ServiceUnit.
	// e.g. 8080 for web/api, 9000 for worker
	ContainerPort int32 `json:"containerPort"`

	// Number of replicas to run.
	// web/api default: 2 — worker default: 1
	Size int32 `json:"size"`

	// Workload classification — web, worker, or api.
	// Used by the controller to apply appropriate defaults
	// (resource limits, HPA config, ingress wiring).
	AppType ServiceUnitAppType `json:"appType"`

	// Application runtime stack.
	// Used for observability labelling and stack-specific configuration.
	StackType ServiceUnitStackType `json:"stackType"`
}

// ServiceUnitBuildRef references the Build CR that provides the image
// for a build-type ServiceUnit.
type ServiceUnitBuildRef struct {
	// Name of the Build CR in the same namespace.
	// Controller waits for the Build to reach Succeeded phase
	// before resolving the image and proceeding with deployment.
	Name string `json:"name"`
}

// ServiceUnitType is the flavour of a ServiceUnit.
type ServiceUnitType string

const (
	// ServiceUnitTypeStatic — image provided directly.
	// No Build CR dependency. Image field required.
	ServiceUnitTypeStatic ServiceUnitType = "static"

	// ServiceUnitTypeBuild — image resolved from a Build CR.
	// BuildRef field required. Image field ignored.
	ServiceUnitTypeBuild ServiceUnitType = "build"
)

// ServiceUnitAppType is the workload classification of a ServiceUnit.
type ServiceUnitAppType string

const (
	// ServiceUnitAppTypeWeb — HTTP-serving frontend workload.
	// Default: 2 replicas, port 8080, ingress wired.
	ServiceUnitAppTypeWeb ServiceUnitAppType = "web"

	// ServiceUnitAppTypeWorker — background processing workload.
	// Default: 1 replica, no ingress.
	ServiceUnitAppTypeWorker ServiceUnitAppType = "worker"

	// ServiceUnitAppTypeAPI — HTTP-serving API workload.
	// Default: 2 replicas, port 8080, ingress wired.
	ServiceUnitAppTypeAPI ServiceUnitAppType = "api"
)

// ServiceUnitStackType is the application runtime stack.
type ServiceUnitStackType string

const (
	ServiceUnitStackTypeNodeJS ServiceUnitStackType = "nodejs"
	ServiceUnitStackTypePython ServiceUnitStackType = "python"
	ServiceUnitStackTypeGo     ServiceUnitStackType = "go"
	ServiceUnitStackTypeJava   ServiceUnitStackType = "java"
	ServiceUnitStackTypeRuby   ServiceUnitStackType = "ruby"
	ServiceUnitStackTypeRust   ServiceUnitStackType = "rust"
)

// ServiceUnitStatus is the observed state of a ServiceUnit.
type ServiceUnitStatus struct {
	// Current lifecycle phase.
	Phase ServiceUnitStatusPhase `json:"phase,omitempty"`

	// Fully qualified image reference that was resolved and deployed.
	// For build type — includes the digest from the BuildRun output.
	Image string `json:"image,omitempty"`

	// Human-readable summary of the current state.
	// Populated on failure with the root cause.
	Message string `json:"message,omitempty"`

	// When this ServiceUnit last transitioned to its current phase.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ServiceUnitStatusPhase is the lifecycle phase of a ServiceUnit.
type ServiceUnitStatusPhase string

const (
	// ServiceUnitStatusPhasePending — waiting for Package or Build to be ready.
	ServiceUnitStatusPhasePending ServiceUnitStatusPhase = "Pending"

	// ServiceUnitStatusPhaseReady — workload running and serving traffic.
	ServiceUnitStatusPhaseReady ServiceUnitStatusPhase = "Ready"

	// ServiceUnitStatusPhaseFailed — workload failed to start or crashed.
	ServiceUnitStatusPhaseFailed ServiceUnitStatusPhase = "Failed"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// newServiceUnitBase returns a base ServiceUnitData with shared labels.
func newServiceUnitBase(name, namespace, envName, appLabel string) *ServiceUnitData {
	return &ServiceUnitData{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "environments.blanketops.dev/v1alpha1",
			Kind:       "ServiceUnit",
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
		        "environments.blanketops.dev/type":             "dev",
		        "environments.blanketops.dev/api-version":      "v0.1.4",
		        "environments.blanketops.dev/contract-version": "v0.1.7",
		        "environments.blanketops.dev/controller-version": "v0.2.3",
		        "environments.blanketops.dev/operator-version": "v0.2.1",
			},
		},
	}
}

// NewStaticServiceUnitData returns a ServiceUnitData matching the canonical
// for-kaniko-app-api static web unit.
// Matches: environments.blanketops.dev/v1alpha1 ServiceUnit for-kaniko-app-api
func NewStaticServiceUnitData(name, namespace, envName string) *ServiceUnitData {
	su := newServiceUnitBase(name, namespace, envName, "api")
	su.Spec = ServiceUnitSpec{
		Contract: ServiceUnitContract{
			Type:          ServiceUnitTypeStatic,
			Image:         "docker.io/nkanyezisolutions/" + envName + ":master",
			ContainerPort: 8080,
			Size:          2,
			AppType:       ServiceUnitAppTypeWeb,
			StackType:     ServiceUnitStackTypeNodeJS,
		},
	}
	return su
}

// NewBuildServiceUnitData returns a ServiceUnitData matching the canonical
// for-kaniko-app-worker build unit — image resolved from a Build CR.
// Matches: environments.blanketops.dev/v1alpha1 ServiceUnit for-kaniko-app-worker
func NewBuildServiceUnitData(name, namespace, envName string) *ServiceUnitData {
	su := newServiceUnitBase(name, namespace, envName, "worker")
	su.Spec = ServiceUnitSpec{
		Contract: ServiceUnitContract{
			Type: ServiceUnitTypeBuild,
			BuildRef: &ServiceUnitBuildRef{
				Name: envName,
			},
			ContainerPort: 9000,
			Size:          1,
			AppType:       ServiceUnitAppTypeWorker,
			StackType:     ServiceUnitStackTypePython,
		},
	}
	return su
}

// NewAPIServiceUnitData returns a ServiceUnitData for a Go API unit.
// Useful for testing multi-stack environments.
func NewAPIServiceUnitData(name, namespace, envName string) *ServiceUnitData {
	su := newServiceUnitBase(name, namespace, envName, "api")
	su.Spec = ServiceUnitSpec{
		Contract: ServiceUnitContract{
			Type:          ServiceUnitTypeStatic,
			Image:         "docker.io/nkanyezisolutions/" + envName + "-api:main",
			ContainerPort: 8080,
			Size:          2,
			AppType:       ServiceUnitAppTypeAPI,
			StackType:     ServiceUnitStackTypeGo,
		},
	}
	return su
}