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

package sources

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

	sourcesv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/sources/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"
)

// -----------------------------------------------------------------------------
// Fake GitRepository Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeGitRepositoryDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeGitRepositoryDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeGitRepositoryDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeGitRepositoryDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeGitRepositoryDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeGitRepositoryDomain) GVK() schema.GroupVersionKind {
	return sourcesv1alpha1.GroupVersion.WithKind("GitRepository")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeGitRepositoryDomain) *GitRepositoryReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		sourcesv1alpha1.GroupVersion.WithKind("GitRepository"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &GitRepositoryReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validGitRepository(name string, strategy string) *sourcesv1alpha1.GitRepository {
	return &sourcesv1alpha1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: sourcesv1alpha1.GitRepositorySpec{
			Image: "docker.io/example/app:latest",

			Strategy: sourcesv1alpha1.Strategy{
				Kind: "ClusterGitRepositoryStrategy",
				Name: strategy,
			},

			Source: sourcesv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: sourcesv1alpha1.ServiceAccount{
				Name: "GitRepository-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// GitRepository Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("GitRepository Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-GitRepository-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			GitRepository := &sourcesv1alpha1.GitRepository{}
			if err := k8sClient.Get(ctx, key, GitRepository); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validGitRepository(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			GitRepository := &sourcesv1alpha1.GitRepository{}
			if err := k8sClient.Get(ctx, key, GitRepository); err == nil {
				_ = k8sClient.Delete(ctx, GitRepository)
			}
		})

		It("routes the GitRepository update through the engine to the GitRepository domain", func() {
			domain := &FakeGitRepositoryDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("GitRepository"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same GitRepository multiple times", func() {
			domain := &FakeGitRepositoryDomain{}
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

		It("returns an error when the GitRepository domain fails", func() {
			domain := &FakeGitRepositoryDomain{
				Err: fmt.Errorf("GitRepository domain failure"),
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

	Context("GitRepository strategy routing", func() {
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
			"routes GitRepository with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				GitRepository := validGitRepository(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, GitRepository)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, GitRepository)
				})

				domain := &FakeGitRepositoryDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("GitRepository"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				GitRepositoryObj, ok := cmd.Obj.(*sourcesv1alpha1.GitRepository)
				Expect(ok).To(BeTrue())

				Expect(GitRepositoryObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(GitRepositoryObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "GitRepository-kaniko",
				strategy: "kaniko",
			}),
			Entry("GitRepositoryah", testCase{
				name:     "GitRepository-GitRepositoryah",
				strategy: "GitRepositoryah-shipwright-managed-push",
			}),
			Entry("GitRepositorypacks", testCase{
				name:     "GitRepository-GitRepositorypacks",
				strategy: "GitRepositorypacks-v3",
			}),
		)
	})
})
