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

// -----------------------------------------------------------------------------
// Fake ServiceUnit Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
// -----------------------------------------------------------------------------

type FakeServiceUnitDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeServiceUnitDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeServiceUnitDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeServiceUnitDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeServiceUnitDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeServiceUnitDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("ServiceUnit")
}

// Note: testLogger is declared in build_controller_test.go — same package, no redeclaration.

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newServiceUnitReconciler(domain *FakeServiceUnitDomain) *envctrl.ServiceUnitReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("ServiceUnit"),
		domain,
	)

	eng := engine.NewEngine(reg, testLogger)

	return &envctrl.ServiceUnitReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      testLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// serviceUnitFromData converts a testdata.ServiceUnitData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// ServiceUnit.Spec is an opaque contract (Spec.Contract runtime.RawExtension)
// — the resolver parses it as raw JSON, not typed Go struct fields.
// testdata's ServiceUnitContract already carries the correct json tags
// (including the static-image vs build-ref distinction) for that raw
// contract shape, so this just marshals it directly.
func serviceUnitFromData(d *testdata.ServiceUnitData) *environmentsv1alpha1.ServiceUnit {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal service unit contract: %v", err))
	}
	return &environmentsv1alpha1.ServiceUnit{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1alpha1.ServiceUnitSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// -----------------------------------------------------------------------------
// ServiceUnit Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("ServiceUnit Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-serviceunit-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			su := &environmentsv1alpha1.ServiceUnit{}
			if err := k8sClient.Get(ctx, key, su); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, serviceUnitFromData(
						testdata.NewStaticServiceUnitData(name, testNamespace, "for-kaniko-app"),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			su := &environmentsv1alpha1.ServiceUnit{}
			if err := k8sClient.Get(ctx, key, su); err == nil {
				_ = k8sClient.Delete(ctx, su)
			}
		})

		It("routes the ServiceUnit update through the engine to the ServiceUnit domain", func() {
			domain := &FakeServiceUnitDomain{}
			reconciler := newServiceUnitReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("ServiceUnit"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same ServiceUnit multiple times", func() {
			domain := &FakeServiceUnitDomain{}
			reconciler := newServiceUnitReconciler(domain)

			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the ServiceUnit domain fails", func() {
			domain := &FakeServiceUnitDomain{
				Err: fmt.Errorf("serviceunit domain failure"),
			}
			reconciler := newServiceUnitReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// -------------------------------------------------------------------------
	// TYPE ROUTING
	// ServiceUnit types are static | build —
	// not build strategies. Fixed from the copy-paste.
	// -------------------------------------------------------------------------

	Context("ServiceUnit type routing", func() {
		type serviceUnitTestCase struct {
			name          string
			unitType      testdata.ServiceUnitType
			appType       testdata.ServiceUnitAppType
			stackType     testdata.ServiceUnitStackType
			containerPort int32
			size          int32
		}

		DescribeTable(
			"routes ServiceUnit with correct type and appType to the domain",
			func(tc serviceUnitTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				var d *testdata.ServiceUnitData
				if tc.unitType == testdata.ServiceUnitTypeStatic {
					d = testdata.NewStaticServiceUnitData(tc.name, testNamespace, "for-kaniko-app")
				} else {
					d = testdata.NewBuildServiceUnitData(tc.name, testNamespace, "for-kaniko-app")
				}
				d.Spec.Contract.AppType = tc.appType
				d.Spec.Contract.StackType = tc.stackType
				d.Spec.Contract.ContainerPort = tc.containerPort
				d.Spec.Contract.Size = tc.size

				Expect(k8sClient.Create(ctx, serviceUnitFromData(d))).To(Succeed())
				DeferCleanup(func() {
					su := &environmentsv1alpha1.ServiceUnit{}
					if err := k8sClient.Get(ctx, key, su); err == nil {
						_ = k8sClient.Delete(ctx, su)
					}
				})

				domain := &FakeServiceUnitDomain{}
				reconciler := newServiceUnitReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("ServiceUnit"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				suObj, ok := cmd.Obj.(*environmentsv1alpha1.ServiceUnit)
				Expect(ok).To(BeTrue())

				var contract testdata.ServiceUnitContract
				Expect(json.Unmarshal(suObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(string(contract.Type)).To(Equal(string(tc.unitType)))
				Expect(string(contract.AppType)).To(Equal(string(tc.appType)))
				Expect(string(contract.StackType)).To(Equal(string(tc.stackType)))
				Expect(contract.ContainerPort).To(Equal(tc.containerPort))
				Expect(contract.Size).To(Equal(tc.size))
			},

			// Matches real CR: serviceunit-for-kaniko-web-sample
			Entry("static web nodejs", serviceUnitTestCase{
				name:          "su-static-web",
				unitType:      testdata.ServiceUnitTypeStatic,
				appType:       testdata.ServiceUnitAppTypeWeb,
				stackType:     testdata.ServiceUnitStackTypeNodeJS,
				containerPort: 8080,
				size:          2,
			}),
			// Matches real CR: serviceunit-for-kaniko-worker-sample
			Entry("build worker python", serviceUnitTestCase{
				name:          "su-build-worker",
				unitType:      testdata.ServiceUnitTypeBuild,
				appType:       testdata.ServiceUnitAppTypeWorker,
				stackType:     testdata.ServiceUnitStackTypePython,
				containerPort: 9000,
				size:          1,
			}),
			Entry("static api go", serviceUnitTestCase{
				name:          "su-static-api",
				unitType:      testdata.ServiceUnitTypeStatic,
				appType:       testdata.ServiceUnitAppTypeAPI,
				stackType:     testdata.ServiceUnitStackTypeGo,
				containerPort: 8080,
				size:          2,
			}),
		)
	})

	// -------------------------------------------------------------------------
	// BUILD REF RESOLUTION
	// build-type ServiceUnit must carry a non-nil BuildRef
	// -------------------------------------------------------------------------

	Context("ServiceUnit build ref", func() {
		It("carries the buildRef name through to the domain for build-type units", func() {
			const name = "su-buildref"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewBuildServiceUnitData(name, testNamespace, "for-kaniko-app")
			Expect(k8sClient.Create(ctx, serviceUnitFromData(d))).To(Succeed())
			DeferCleanup(func() {
				su := &environmentsv1alpha1.ServiceUnit{}
				if err := k8sClient.Get(ctx, key, su); err == nil {
					_ = k8sClient.Delete(ctx, su)
				}
			})

			domain := &FakeServiceUnitDomain{}
			reconciler := newServiceUnitReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			suObj, ok := domain.Cmd.Obj.(*environmentsv1alpha1.ServiceUnit)
			Expect(ok).To(BeTrue())

			var contract testdata.ServiceUnitContract
			Expect(json.Unmarshal(suObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(string(contract.Type)).To(Equal(string(testdata.ServiceUnitTypeBuild)))
			Expect(contract.BuildRef).NotTo(BeNil())
			Expect(contract.BuildRef.Name).To(Equal("for-kaniko-app"))
			// Image must be empty for build type — resolved from BuildRun output
			Expect(contract.Image).To(BeEmpty())
		})

		It("carries the image through to the domain for static-type units", func() {
			const name = "su-static-image"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewStaticServiceUnitData(name, testNamespace, "for-kaniko-app")
			Expect(k8sClient.Create(ctx, serviceUnitFromData(d))).To(Succeed())
			DeferCleanup(func() {
				su := &environmentsv1alpha1.ServiceUnit{}
				if err := k8sClient.Get(ctx, key, su); err == nil {
					_ = k8sClient.Delete(ctx, su)
				}
			})

			domain := &FakeServiceUnitDomain{}
			reconciler := newServiceUnitReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			suObj, ok := domain.Cmd.Obj.(*environmentsv1alpha1.ServiceUnit)
			Expect(ok).To(BeTrue())

			var contract testdata.ServiceUnitContract
			Expect(json.Unmarshal(suObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(string(contract.Type)).To(Equal(string(testdata.ServiceUnitTypeStatic)))
			Expect(contract.Image).NotTo(BeEmpty())
			// BuildRef must be nil for static type — no Build CR dependency
			Expect(contract.BuildRef).To(BeNil())
		})
	})

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("ServiceUnit not found", func() {
		It("returns no error when the ServiceUnit CR no longer exists", func() {
			domain := &FakeServiceUnitDomain{}
			reconciler := newServiceUnitReconciler(domain)

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
