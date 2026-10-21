/*
Copyright 2026 BlanketOps.

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
	"encoding/json"
	"testing"

	environmentv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	buildresolve "github.com/blanketops/environments/resolution/build/resolve"
	deploymentresolve "github.com/blanketops/environments/resolution/deployment/resolve"
	packageresolve "github.com/blanketops/environments/resolution/packages/resolve"
	serviceunitresolve "github.com/blanketops/environments/resolution/serviceunit/resolve"
	"k8s.io/apimachinery/pkg/runtime"
)

// The fixtures here are hand-written JSON shapes. The engine's own resolvers
// (environments/resolution) are what decide whether a contract is valid, so
// every fixture builder must produce a contract they accept. A field renamed
// or retyped in the engine fails here, without a cluster, instead of showing
// up as an unrelated controller or gRPC failure.

func contractOf(t *testing.T, v any) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	return runtime.RawExtension{Raw: raw}
}

func TestFixturesResolveWithTheEngine(t *testing.T) {
	const ns, env = "default", "for-kaniko-app"

	builds := map[string]*BuildData{
		"NewBuildData":           NewBuildData("app-build", ns, env),
		"NewBuildahBuildData":    NewBuildahBuildData("app-build", ns, env),
		"NewBuildpacksBuildData": NewBuildpacksBuildData("app-build", ns, env),
	}
	for name, d := range builds {
		t.Run(name, func(t *testing.T) {
			cr := &environmentv1alpha1.Build{ObjectMeta: d.ObjectMeta,
				Spec: environmentv1alpha1.BuildSpec{Contract: contractOf(t, d.Spec)}}
			if _, err := buildresolve.ResolveBuild(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}

	deployments := map[string]*DeploymentData{
		"NewDeploymentData":          NewDeploymentData("app-deployment", ns, env),
		"NewMultiUnitDeploymentData": NewMultiUnitDeploymentData("app-deployment", ns, env),
		"NewKnativeDeploymentData":   NewKnativeDeploymentData("app-deployment", ns, env),
	}
	for name, d := range deployments {
		t.Run(name, func(t *testing.T) {
			cr := &environmentv1alpha1.Deployment{ObjectMeta: d.ObjectMeta,
				Spec: environmentv1alpha1.DeploymentSpec{Contract: contractOf(t, d.Spec.Contract)}}
			if _, err := deploymentresolve.ResolveDeployment(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}

	packages := map[string]*PackageData{
		"NewPackageData":         NewPackageData("app-package", ns, env),
		"NewDisabledPackageData": NewDisabledPackageData("app-package", ns, env),
		"NewNoDiffPackageData":   NewNoDiffPackageData("app-package", ns, env),
		"NewStagingPackageData":  NewStagingPackageData("app-package", ns, env),
	}
	for name, d := range packages {
		t.Run(name, func(t *testing.T) {
			cr := &environmentv1alpha1.Package{ObjectMeta: d.ObjectMeta,
				Spec: environmentv1alpha1.PackageSpec{Contract: contractOf(t, d.Spec.Contract)}}
			if _, err := packageresolve.ResolvePackage(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}

	units := map[string]*ServiceUnitData{
		"NewStaticServiceUnitData": NewStaticServiceUnitData("app-unit", ns, env),
		"NewBuildServiceUnitData":  NewBuildServiceUnitData("app-unit", ns, env),
		"NewAPIServiceUnitData":    NewAPIServiceUnitData("app-unit", ns, env),
	}
	for name, d := range units {
		t.Run(name, func(t *testing.T) {
			cr := &environmentv1alpha1.ServiceUnit{ObjectMeta: d.ObjectMeta,
				Spec: environmentv1alpha1.ServiceUnitSpec{Contract: contractOf(t, d.Spec.Contract)}}
			if _, err := serviceunitresolve.ResolveServiceUnit(cr); err != nil {
				t.Fatalf("engine rejects the %s contract: %v", name, err)
			}
		})
	}
}
