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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	environmentsv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/environments/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"
)

// -----------------------------------------------------------------------------
// Fake Build Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
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

//
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
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validBuild(name string, strategy string) *environmentsv1alpha1.Build {
	return &environmentsv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.BuildSpec{
			Image: "docker.io/example/app:latest",

			Strategy: environmentsv1alpha1.Strategy{
				Kind: "ClusterBuildStrategy",
				Name: strategy,
			},

			Source: environmentsv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name: "build-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Build Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Build Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-build-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			build := &environmentsv1alpha1.Build{}
			if err := k8sClient.Get(ctx, key, build); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validBuild(name, "kaniko")),
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
		type testCase struct {
			name     string
			strategy string
			serviceAccount serviceAccount
		}

		type serviceAccount struct {
			name string
			secret string
		}

		DescribeTable(
			"routes Build with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				build := validBuild(tc.name, tc.strategy)
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
				//Expect(buildObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "build-kaniko",
				strategy: "kaniko",
			}),
			Entry("buildah", testCase{
				name:     "build-buildah",
				strategy: "buildah-shipwright-managed-push",
			}),
			Entry("buildpacks", testCase{
				name:     "build-buildpacks",
				strategy: "buildpacks-v3",
			}),
		)
	})
})
