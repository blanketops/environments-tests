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
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	envctrl "github.com/blanketops/environments-controller/pkg/controller/environments"
	envruntime "github.com/blanketops/environments-controller/pkg/runtime"
	"github.com/blanketops/environments/core/command"
	"github.com/blanketops/environments/core/engine"
	"github.com/blanketops/environments/core/registry"
)

// -----------------------------------------------------------------------------
// Fake Environment Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeEnvironmentDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeEnvironmentDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeEnvironmentDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeEnvironmentDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeEnvironmentDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeEnvironmentDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Environment")
}

// Note: testLogger is declared in build.go — same package, no redeclaration.

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newEnvironmentTestReconciler(domain *FakeEnvironmentDomain) *envctrl.EnvironmentReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Environment"),
		domain,
	)

	eng := engine.NewEngine(reg, testLogger)

	return &envctrl.EnvironmentReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      testLogger,
		Recorder: events.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

// environmentContract mirrors the raw JSON shape resolution/environment/resolve
// actually parses: applicationName, branch, gitOwner, environmentType, and
// version are required; CR references (build, deployment, route, etc.) and
// serviceUnits are optional and omitted here — this fixture only needs to
// resolve, not compose a full environment.
type environmentContract struct {
	ApplicationName string `json:"applicationName"`
	Branch          string `json:"branch"`
	GitOwner        string `json:"gitOwner"`
	EnvironmentType string `json:"environmentType"`
	Version         string `json:"version"`
}

// validEnvironment builds an Environment CR with a minimal, resolver-valid
// contract. Environment.Spec is an opaque contract (Spec.Contract
// runtime.RawExtension) — the resolver parses it as raw JSON, not typed Go
// struct fields directly on EnvironmentSpec.
func validEnvironment(name string, environmentType string) *environmentsv1alpha1.Environment {
	raw, err := json.Marshal(environmentContract{
		ApplicationName: name,
		Branch:          "main",
		GitOwner:        "example-org",
		EnvironmentType: environmentType,
		Version:         "v1",
	})
	if err != nil {
		panic(fmt.Sprintf("marshal environment contract: %v", err))
	}
	return &environmentsv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.EnvironmentSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Environment Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Environment Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-environment-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "default",
		}

		BeforeEach(func() {
			Environment := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, key, Environment); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validEnvironment(name, "development")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Environment := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, key, Environment); err == nil {
				_ = k8sClient.Delete(ctx, Environment)
			}
		})

		It("routes the Environment update through the engine to the Environment domain", func() {
			domain := &FakeEnvironmentDomain{}
			reconciler := newEnvironmentTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Environment"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Environment multiple times", func() {
			domain := &FakeEnvironmentDomain{}
			reconciler := newEnvironmentTestReconciler(domain)

			for i := 0; i < 2; i++ {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the Environment domain fails", func() {
			domain := &FakeEnvironmentDomain{
				Err: fmt.Errorf("Environment domain failure"),
			}
			reconciler := newEnvironmentTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// -------------------------------------------------------------------------
	// STRATEGY ROUTING
	// -------------------------------------------------------------------------

	Context("Environment type routing", func() {
		type testCase struct {
			name            string
			environmentType string
		}

		DescribeTable(
			"routes Environment with the correct environmentType",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				environment := validEnvironment(tc.name, tc.environmentType)
				Expect(k8sClient.Create(ctx, environment)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, environment)
				})

				domain := &FakeEnvironmentDomain{}
				reconciler := newEnvironmentTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Environment"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				environmentObj, ok := cmd.Obj.(*environmentsv1alpha1.Environment)
				Expect(ok).To(BeTrue())

				var contract environmentContract
				Expect(json.Unmarshal(environmentObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(contract.EnvironmentType).To(Equal(tc.environmentType))
			},

			Entry("development", testCase{
				name:            "environment-development",
				environmentType: "development",
			}),
			Entry("staging", testCase{
				name:            "environment-staging",
				environmentType: "staging",
			}),
			Entry("production", testCase{
				name:            "environment-production",
				environmentType: "production",
			}),
		)
	})
})
