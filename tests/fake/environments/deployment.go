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

	// test data — apimachinery types we built
	testdata "github.com/blanketops/environments-tests/tests/data/environments"
)

// Fake Deployment Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.

type FakeDeploymentDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeDeploymentDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeDeploymentDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeDeploymentDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeDeploymentDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeDeploymentDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Deployment")
}

// Note: testLogger is declared in build_controller_test.go — same package, no redeclaration.

// Test helpers

func newDeploymentReconciler(domain *FakeDeploymentDomain) *envctrl.DeploymentReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Deployment"),
		domain,
	)

	eng := engine.NewEngine(reg, testLogger)

	return &envctrl.DeploymentReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      testLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// deploymentFromData converts a testdata.DeploymentData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// Deployment.Spec is an opaque contract (Spec.Contract runtime.RawExtension)
// — the resolver parses it as raw JSON, not typed Go struct fields.
// testdata's DeploymentContract already carries the correct json tags for
// that raw contract shape, so this just marshals it directly.
func deploymentFromData(d *testdata.DeploymentData) *environmentsv1alpha1.Deployment {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal deployment contract: %v", err))
	}
	return &environmentsv1alpha1.Deployment{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1alpha1.DeploymentSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// Deployment Controller Tests

var _ = Describe("Deployment Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// BASIC BEHAVIOUR

	Context("Basic reconciliation behaviour", func() {
		const name = "test-deployment-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			deployment := &environmentsv1alpha1.Deployment{}
			if err := k8sClient.Get(ctx, key, deployment); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, deploymentFromData(
						testdata.NewDeploymentData(name, testNamespace, "for-kaniko-app"),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			deployment := &environmentsv1alpha1.Deployment{}
			if err := k8sClient.Get(ctx, key, deployment); err == nil {
				_ = k8sClient.Delete(ctx, deployment)
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newDeploymentReconciler(&FakeDeploymentDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the Deployment update through the engine to the Deployment domain", func() {
			domain := &FakeDeploymentDomain{}
			reconciler := newDeploymentReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("Deployment"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Deployment multiple times", func() {
			domain := &FakeDeploymentDomain{}
			reconciler := newDeploymentReconciler(domain)

			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the Deployment domain fails", func() {
			domain := &FakeDeploymentDomain{
				Err: fmt.Errorf("deployment domain failure"),
			}
			reconciler := newDeploymentReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// ROLLOUT STRATEGY ROUTING
	// Deployment strategies are Rolling | Canary | BlueGreen —
	// not build strategies. These were copy-pasted from Build and fixed here.

	Context("Deployment rollout strategy routing", func() {
		type deploymentTestCase struct {
			name            string
			strategy        testdata.DeploymentStrategyValue
			imageAutomation bool
		}

		DescribeTable(
			"routes Deployment with correct rollout strategy to the domain",
			func(tc deploymentTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewDeploymentData(tc.name, testNamespace, "for-kaniko-app")
				d.Spec.Contract.Strategy = tc.strategy
				d.Spec.Contract.ImageAutomation = tc.imageAutomation

				Expect(k8sClient.Create(ctx, deploymentFromData(d))).To(Succeed())
				DeferCleanup(func() {
					deployment := &environmentsv1alpha1.Deployment{}
					if err := k8sClient.Get(ctx, key, deployment); err == nil {
						_ = k8sClient.Delete(ctx, deployment)
						_, _ = newDeploymentReconciler(&FakeDeploymentDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeDeploymentDomain{}
				reconciler := newDeploymentReconciler(domain)

				// First reconcile on a brand-new object only adds the
				// finalizer and returns — the domain isn't invoked until
				// the next pass.
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Deployment"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				deploymentObj, ok := cmd.Obj.(*environmentsv1alpha1.Deployment)
				Expect(ok).To(BeTrue())

				var contract testdata.DeploymentContract
				Expect(json.Unmarshal(deploymentObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(string(contract.Strategy)).To(Equal(string(tc.strategy)))
				Expect(contract.ImageAutomation).To(Equal(tc.imageAutomation))
				Expect(contract.GitOwner).To(Equal("ntlaletsi70"))
				Expect(contract.ManifestsRepo.Path).To(Equal("./overlays/dev"))
			},

			Entry("rolling — standard single unit", deploymentTestCase{
				name:            "deployment-rolling",
				strategy:        testdata.DeploymentStrategyRolling,
				imageAutomation: false,
			}),
			Entry("rolling — image automation enabled", deploymentTestCase{
				name:            "deployment-rolling-auto",
				strategy:        testdata.DeploymentStrategyRolling,
				imageAutomation: true,
			}),
			Entry("canary — traffic splitting", deploymentTestCase{
				name:            "deployment-canary",
				strategy:        testdata.DeploymentStrategyCanary,
				imageAutomation: false,
			}),
			Entry("blue-green — full traffic cutover", deploymentTestCase{
				name:            "deployment-blue-green",
				strategy:        testdata.DeploymentStrategyBlueGreen,
				imageAutomation: false,
			}),
		)
	})

	// RUNTIME ROUTING

	Context("Deployment runtime routing", func() {
		type runtimeTestCase struct {
			name    string
			runtime testdata.DeploymentRuntimeValue
		}

		DescribeTable(
			"routes Deployment with correct runtime to the domain",
			func(tc runtimeTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewDeploymentData(tc.name, testNamespace, "for-kaniko-app")
				d.Spec.Contract.Runtime = tc.runtime

				Expect(k8sClient.Create(ctx, deploymentFromData(d))).To(Succeed())
				DeferCleanup(func() {
					deployment := &environmentsv1alpha1.Deployment{}
					if err := k8sClient.Get(ctx, key, deployment); err == nil {
						_ = k8sClient.Delete(ctx, deployment)
						_, _ = newDeploymentReconciler(&FakeDeploymentDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeDeploymentDomain{}
				reconciler := newDeploymentReconciler(domain)

				// First reconcile on a brand-new object only adds the
				// finalizer and returns — the domain isn't invoked until
				// the next pass.
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				deploymentObj, ok := domain.Cmd.Obj.(*environmentsv1alpha1.Deployment)
				Expect(ok).To(BeTrue())

				var contract testdata.DeploymentContract
				Expect(json.Unmarshal(deploymentObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(string(contract.Runtime)).To(Equal(string(tc.runtime)))
			},

			Entry("kubernetes container runtime", runtimeTestCase{
				name:    "deployment-k8s",
				runtime: testdata.DeploymentRuntimeKubernetes,
			}),
			Entry("knative serving runtime", runtimeTestCase{
				name:    "deployment-knative",
				runtime: testdata.DeploymentRuntimeKnative,
			}),
		)
	})

	// MULTI-UNIT DEPLOYMENT

	Context("Multi-unit deployment", func() {
		It("routes a Deployment with multiple ServiceUnits through the domain", func() {
			const name = "deployment-multi-unit"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewMultiUnitDeploymentData(name, testNamespace, "for-kaniko-app")
			Expect(k8sClient.Create(ctx, deploymentFromData(d))).To(Succeed())
			DeferCleanup(func() {
				deployment := &environmentsv1alpha1.Deployment{}
				if err := k8sClient.Get(ctx, key, deployment); err == nil {
					_ = k8sClient.Delete(ctx, deployment)
					_, _ = newDeploymentReconciler(&FakeDeploymentDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			})

			domain := &FakeDeploymentDomain{}
			reconciler := newDeploymentReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			deploymentObj, ok := domain.Cmd.Obj.(*environmentsv1alpha1.Deployment)
			Expect(ok).To(BeTrue())

			var contract testdata.DeploymentContract
			Expect(json.Unmarshal(deploymentObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(contract.ServiceUnits).To(HaveLen(2))
			Expect(contract.ServiceUnits).To(ContainElements(
				name+"-api",
				name+"-worker",
			))
			Expect(contract.ImageAutomation).To(BeTrue())
		})
	})

	// NOT FOUND — CR deleted before reconcile loop runs

	Context("Deployment not found", func() {
		It("returns no error when the Deployment CR no longer exists", func() {
			domain := &FakeDeploymentDomain{}
			reconciler := newDeploymentReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "does-not-exist",
					Namespace: testNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeFalse())
		})
	})
})
