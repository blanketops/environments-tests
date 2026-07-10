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
	"fmt"
	// test data — apimachinery types we built

	"github.com/go-logr/logr"
	environmentsv1alpha1 "github.com/BlanketOps/environments-api/api/environments/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	testdata "github.com/blanketops-environments-tests/tests/data/environments"
)

// -----------------------------------------------------------------------------
// Fake Build Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
// -----------------------------------------------------------------------------

type FakeBuildDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeBuildDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeBuildDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeBuildDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeBuildDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeBuildDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Build")
}

var testLogger = logr.Discard()

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeBuildDomain) *BuildReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Build"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &BuildReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32),
	}
}

// buildFromData converts a testdata.BuildData into the API type the controller
// expects. This is the bridge between the test data package and the API types.
func buildFromData(d *testdata.BuildData) *environmentsv1alpha1.Build {
	return &environmentsv1alpha1.Build{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1alpha1.BuildSpec{
			Image: d.Spec.Image,
			Strategy: environmentsv1alpha1.Strategy{
				Kind: string(d.Spec.Strategy.Kind),
				Name: d.Spec.Strategy.Name,
			},
			Source: environmentsv1alpha1.GitSource{
				URL:         d.Spec.Source.URL,
				Revision:    d.Spec.Source.Revision,
				Context:     d.Spec.Source.ContextDir,
				CloneSecret: d.Spec.Source.CloneSecret,
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name:   d.Spec.ServiceAccount.Name,
				Secret: d.Spec.ServiceAccount.Secret,
			},
		},
	}
}

// validBuild constructs a Build API object from the canonical test data.
// Uses NewBuildData so all fixtures share the same source of truth.
func validBuild(name, namespace, strategy string) *environmentsv1alpha1.Build {
	var d *testdata.BuildData
	switch strategy {
	case "buildah-shipwright-managed-push":
		d = testdata.NewBuildahBuildData(name, namespace)
	case "buildpacks-v3":
		d = testdata.NewBuildpacksBuildData(name, namespace)
	default:
		// kaniko or any other strategy
		d = testdata.NewBuildData(name, namespace)
		d.Spec.Strategy.Name = strategy
	}
	return buildFromData(d)
}

// -----------------------------------------------------------------------------
// Build Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Build Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

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
			}
		})

		It("routes the Build update through the engine to the Build domain", func() {
			domain := &FakeBuildDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("Build"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Build multiple times", func() {
			domain := &FakeBuildDomain{}
			reconciler := newTestReconciler(domain)

			for i := 0; i < 2; i++ {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
		})

		It("returns an error when the Build domain fails", func() {
			domain := &FakeBuildDomain{
				Err: fmt.Errorf("build domain failure"),
			}
			reconciler := newTestReconciler(domain)

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
				})

				domain := &FakeBuildDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Build"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				buildObj, ok := cmd.Obj.(*environmentsv1alpha1.Build)
				Expect(ok).To(BeTrue())
				Expect(buildObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				Expect(buildObj.Spec.ServiceAccount.Name).To(Equal("build-bot"))
				Expect(buildObj.Spec.ServiceAccount.Secret).To(Equal("docker-registry-credentials"))
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

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("Build not found", func() {
		It("returns no error when the Build CR no longer exists", func() {
			domain := &FakeBuildDomain{}
			reconciler := newTestReconciler(domain)

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
