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

package events

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

	eventsv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/events/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"
)

// -----------------------------------------------------------------------------
// Fake GitHubEvent Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeGitHubEventDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeGitHubEventDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeGitHubEventDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeGitHubEventDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeGitHubEventDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeGitHubEventDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("GitHubEvent")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeGitHubEventDomain) *GitHubEventReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		eventsv1alpha1.GroupVersion.WithKind("GitHubEvent"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &GitHubEventReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validGitHubEvent(name string, strategy string) *eventsv1alpha1.GitHubEvent {
	return &eventsv1alpha1.GitHubEvent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.GitHubEventSpec{
			Image: "docker.io/example/app:latest",

			Strategy: environmentsv1alpha1.Strategy{
				Kind: "ClusterGitHubEventStrategy",
				Name: strategy,
			},

			Source: environmentsv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name: "GitHubEvent-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// GitHubEvent Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("GitHubEvent Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-GitHubEvent-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			GitHubEvent := &eventsv1alpha1.GitHubEvent{}
			if err := k8sClient.Get(ctx, key, GitHubEvent); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validGitHubEvent(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			GitHubEvent := &eventsv1alpha1.GitHubEvent{}
			if err := k8sClient.Get(ctx, key, GitHubEvent); err == nil {
				_ = k8sClient.Delete(ctx, GitHubEvent)
			}
		})

		It("routes the GitHubEvent update through the engine to the GitHubEvent domain", func() {
			domain := &FakeGitHubEventDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("GitHubEvent"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same GitHubEvent multiple times", func() {
			domain := &FakeGitHubEventDomain{}
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

		It("returns an error when the GitHubEvent domain fails", func() {
			domain := &FakeGitHubEventDomain{
				Err: fmt.Errorf("GitHubEvent domain failure"),
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

	Context("GitHubEvent strategy routing", func() {
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
			"routes GitHubEvent with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				GitHubEvent := validGitHubEvent(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, GitHubEvent)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, GitHubEvent)
				})

				domain := &FakeGitHubEventDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("GitHubEvent"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				GitHubEventObj, ok := cmd.Obj.(*environmentsv1alpha1.GitHubEvent)
				Expect(ok).To(BeTrue())

				Expect(GitHubEventObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(GitHubEventObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "GitHubEvent-kaniko",
				strategy: "kaniko",
			}),
			Entry("GitHubEventah", testCase{
				name:     "GitHubEvent-GitHubEventah",
				strategy: "GitHubEventah-shipwright-managed-push",
			}),
			Entry("GitHubEventpacks", testCase{
				name:     "GitHubEvent-GitHubEventpacks",
				strategy: "GitHubEventpacks-v3",
			}),
		)
	})
})
