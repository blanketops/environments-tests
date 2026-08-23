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

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	envctrl "github.com/blanketops/environments-controller/pkg/controller/environments"
	envruntime "github.com/blanketops/environments-controller/pkg/runtime"
	"github.com/blanketops/environments/core/command"
	"github.com/blanketops/environments/core/engine"
	"github.com/blanketops/environments/core/registry"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	testdata "github.com/blanketops/environments-tests/tests/data/environments"
)

// Fake Build Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.

type FakeBuildDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeBuildDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeBuildDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeBuildDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeBuildDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeBuildDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Build")
}

var testLogger = logr.Discard()

// Test helpers

func newBuildTestReconciler(domain *FakeBuildDomain) *envctrl.BuildReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Build"),
		domain,
	)

	eng := engine.NewEngine(reg, testLogger)

	return &envctrl.BuildReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      testLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// buildFromData converts a testdata.BuildData into the API type the controller
// expects. This is the bridge between the test data package and the API types.
//
// Build.Spec is an opaque contract (Spec.Contract runtime.RawExtension) —
// the resolver parses it as raw JSON, not typed Go struct fields. testdata's
// BuildSpec already carries the correct json tags for that raw contract
// shape, so this just marshals it directly rather than re-mapping fields
// onto a typed struct that no longer exists on the real API type.
func buildFromData(d *testdata.BuildData) *environmentsv1alpha1.Build {
	raw, err := json.Marshal(d.Spec)
	if err != nil {
		panic(fmt.Sprintf("marshal build contract: %v", err))
	}
	return &environmentsv1alpha1.Build{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1alpha1.BuildSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// validBuild constructs a Build API object from the canonical test data.
// Uses NewBuildData so all fixtures share the same source of truth.
func validBuild(name, namespace, strategy string) *environmentsv1alpha1.Build {
	var d *testdata.BuildData
	switch strategy {
	case "buildah-shipwright-managed-push":
		d = testdata.NewBuildahBuildData(name, namespace, "for-kaniko-app")
	case "buildpacks-v3":
		d = testdata.NewBuildpacksBuildData(name, namespace, "for-kaniko-app")
	default:
		// kaniko or any other strategy
		d = testdata.NewBuildData(name, namespace, "for-kaniko-app")
		d.Spec.Strategy.Name = strategy
	}
	return buildFromData(d)
}

// Build Controller Tests

var _ = Describe("Build Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// BASIC BEHAVIOUR

	Context("Basic reconciliation behaviour", func() {
		const name = "test-build-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			build := &environmentsv1alpha1.Build{}
			if err := k8sClient.Get(ctx, key, build); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validBuild(name, testNamespace, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			build := &environmentsv1alpha1.Build{}
			if err := k8sClient.Get(ctx, key, build); err == nil {
				_ = k8sClient.Delete(ctx, build)
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newBuildTestReconciler(&FakeBuildDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the Build update through the engine to the Build domain", func() {
			domain := &FakeBuildDomain{}
			reconciler := newBuildTestReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("Build"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Build multiple times", func() {
			domain := &FakeBuildDomain{}
			reconciler := newBuildTestReconciler(domain)

			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the Build domain fails", func() {
			domain := &FakeBuildDomain{
				Err: fmt.Errorf("build domain failure"),
			}
			reconciler := newBuildTestReconciler(domain)

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

	// STRATEGY ROUTING

	Context("Build strategy routing", func() {
		type buildTestCase struct {
			name     string
			strategy string
		}

		DescribeTable(
			"routes Build with correct strategy to the domain",
			func(tc buildTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				build := validBuild(tc.name, testNamespace, tc.strategy)
				Expect(k8sClient.Create(ctx, build)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, build)
					_, _ = newBuildTestReconciler(&FakeBuildDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				})

				domain := &FakeBuildDomain{}
				reconciler := newBuildTestReconciler(domain)

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
				Expect(cmd.GVK.Kind).To(Equal("Build"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				buildObj, ok := cmd.Obj.(*environmentsv1alpha1.Build)
				Expect(ok).To(BeTrue())

				var contract testdata.BuildSpec
				Expect(json.Unmarshal(buildObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(contract.Strategy.Name).To(Equal(tc.strategy))
				Expect(contract.ServiceAccount.Name).To(Equal("build-bot"))
				Expect(contract.ServiceAccount.Secret).To(Equal("docker-registry-credentials"))
			},

			Entry("kaniko", buildTestCase{
				name:     "build-kaniko",
				strategy: "kaniko",
			}),
			Entry("buildah", buildTestCase{
				name:     "build-buildah",
				strategy: "buildah-shipwright-managed-push",
			}),
			Entry("buildpacks", buildTestCase{
				name:     "build-buildpacks",
				strategy: "buildpacks-v3",
			}),
		)
	})

	// NOT FOUND — CR deleted before reconcile loop runs

	Context("Build not found", func() {
		It("returns no error when the Build CR no longer exists", func() {
			domain := &FakeBuildDomain{}
			reconciler := newBuildTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "does-not-exist",
					Namespace: testNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			// Domain should not be called — nothing to reconcile
			Expect(domain.Called).To(BeFalse())
		})
	})
})
