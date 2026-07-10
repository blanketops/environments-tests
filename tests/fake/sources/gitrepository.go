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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sourcesv1alpha1 "github.com/BlanketOps/environments-api/api/sources/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments/core"

	// test data — apimachinery types we built
	testdata "github.com/blanketops-environments-tests/tests/data/sources"
)

// -----------------------------------------------------------------------------
// Fake GitRepository Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
//
// The GitRepository controller reconciles a Crossplane GitHub webhook provider
// resource as an owned child — registering the webhook on GitHub via the API.
// We test only that the domain receives the correct CR.
// -----------------------------------------------------------------------------

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

// gitRepositoryTestLogger is the discard logger for the sources package.
// Declared per-package — separate from environments, events, and networks packages.
var gitRepositoryTestLogger = logr.Discard()

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newGitRepositoryReconciler(domain *FakeGitRepositoryDomain) *GitRepositoryReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		sourcesv1alpha1.GroupVersion.WithKind("GitRepository"),
		domain,
	)

	engine := core.NewEngine(registry, gitRepositoryTestLogger)

	return &GitRepositoryReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      gitRepositoryTestLogger,
		Recorder: record.NewFakeRecorder(32),
	}
}

// gitRepositoryFromData converts a testdata.GitRepositoryData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
func gitRepositoryFromData(d *testdata.GitRepositoryData) *sourcesv1alpha1.GitRepository {
	webhooks := make([]sourcesv1alpha1.WebhookSubscription, len(d.Spec.Contract.Webhooks))
	for i, w := range d.Spec.Contract.Webhooks {
		events := make([]sourcesv1alpha1.WebhookEvent, len(w.Events))
		for j, e := range w.Events {
			events[j] = sourcesv1alpha1.WebhookEvent(e)
		}
		webhooks[i] = sourcesv1alpha1.WebhookSubscription{Events: events}
	}

	return &sourcesv1alpha1.GitRepository{
		ObjectMeta: d.ObjectMeta,
		Spec: sourcesv1alpha1.GitRepositorySpec{
			Contract: sourcesv1alpha1.GitRepositoryContract{
				Provider: d.Spec.Contract.Provider,
				HookURL:  d.Spec.Contract.HookURL,
				Repository: sourcesv1alpha1.GitRepository{
					Owner: d.Spec.Contract.Repository.Owner,
					Name:  d.Spec.Contract.Repository.Name,
				},
				Webhooks: webhooks,
			},
		},
	}
}

// -----------------------------------------------------------------------------
// GitRepository Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("GitRepository Controller", func() {
	ctx := context.Background()

	// GitRepository CRs are cluster-scoped — no namespace in the real CR.
	// envtest requires a namespace — using default.
	const testNamespace = "default"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "for-kaniko-app-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			gr := &sourcesv1alpha1.GitRepository{}
			if err := k8sClient.Get(ctx, key, gr); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, gitRepositoryFromData(
						testdata.NewGitRepositoryData(name, "for-kaniko-app"),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			gr := &sourcesv1alpha1.GitRepository{}
			if err := k8sClient.Get(ctx, key, gr); err == nil {
				_ = k8sClient.Delete(ctx, gr)
			}
		})

		It("routes the GitRepository update through the engine to the GitRepository domain", func() {
			domain := &FakeGitRepositoryDomain{}
			reconciler := newGitRepositoryReconciler(domain)

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
			reconciler := newGitRepositoryReconciler(domain)

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
				Err: fmt.Errorf("gitrepository domain failure"),
			}
			reconciler := newGitRepositoryReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// -------------------------------------------------------------------------
	// WEBHOOK EVENT ROUTING
	// GitRepository webhook events are push | pull_request —
	// not build strategies. Fixed from the copy-paste.
	// -------------------------------------------------------------------------

	Context("GitRepository webhook event routing", func() {
		type gitRepositoryTestCase struct {
			name          string
			repoName      string
			hookURL       string
			webhookEvents []testdata.WebhookEvent
		}

		DescribeTable(
			"routes GitRepository with correct webhook events to the domain",
			func(tc gitRepositoryTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewGitRepositoryData(tc.repoName, tc.repoName)
				d.Name = tc.name
				d.Spec.Contract.HookURL = tc.hookURL
				d.Spec.Contract.Webhooks = []testdata.WebhookSubscription{
					{Events: tc.webhookEvents},
				}

				Expect(k8sClient.Create(ctx, gitRepositoryFromData(d))).To(Succeed())
				DeferCleanup(func() {
					gr := &sourcesv1alpha1.GitRepository{}
					if err := k8sClient.Get(ctx, key, gr); err == nil {
						_ = k8sClient.Delete(ctx, gr)
					}
				})

				domain := &FakeGitRepositoryDomain{}
				reconciler := newGitRepositoryReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("GitRepository"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				grObj, ok := cmd.Obj.(*sourcesv1alpha1.GitRepository)
				Expect(ok).To(BeTrue())
				Expect(grObj.Spec.Contract.Provider).To(Equal("github"))
				Expect(grObj.Spec.Contract.HookURL).To(Equal(tc.hookURL))
				Expect(grObj.Spec.Contract.Repository.Owner).To(Equal("ntlaletsi70"))
				Expect(grObj.Spec.Contract.Webhooks).To(HaveLen(1))
				Expect(grObj.Spec.Contract.Webhooks[0].Events).To(HaveLen(len(tc.webhookEvents)))
			},

			// Matches real CR: for-kaniko-app — push + pull_request
			Entry("push and pull_request — cloudflared tunnel", gitRepositoryTestCase{
				name:     "gr-push-pr",
				repoName: "for-kaniko-app",
				hookURL:  "https://elimination-propecia-meter-strips.trycloudflare.com/",
				webhookEvents: []testdata.WebhookEvent{
					testdata.WebhookEventPush,
					testdata.WebhookEventPullRequest,
				},
			}),
			Entry("push only — cloudflared tunnel", gitRepositoryTestCase{
				name:     "gr-push-only",
				repoName: "for-kaniko-app",
				hookURL:  "https://elimination-propecia-meter-strips.trycloudflare.com/",
				webhookEvents: []testdata.WebhookEvent{
					testdata.WebhookEventPush,
				},
			}),
			Entry("push and pull_request — production ingress URL", gitRepositoryTestCase{
				name:     "gr-prod-ingress",
				repoName: "for-kaniko-app",
				hookURL:  "https://events.blanketops.dev/webhooks/github",
				webhookEvents: []testdata.WebhookEvent{
					testdata.WebhookEventPush,
					testdata.WebhookEventPullRequest,
				},
			}),
		)
	})

	// -------------------------------------------------------------------------
	// REPOSITORY OWNER AND NAME
	// -------------------------------------------------------------------------

	Context("GitRepository owner and name", func() {
		It("carries the repository owner and name through to the domain", func() {
			const name = "gr-repo-identity"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewGitRepositoryData("for-kaniko-app", "for-kaniko-app")
			d.Name = name
			Expect(k8sClient.Create(ctx, gitRepositoryFromData(d))).To(Succeed())
			DeferCleanup(func() {
				gr := &sourcesv1alpha1.GitRepository{}
				if err := k8sClient.Get(ctx, key, gr); err == nil {
					_ = k8sClient.Delete(ctx, gr)
				}
			})

			domain := &FakeGitRepositoryDomain{}
			reconciler := newGitRepositoryReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			grObj, ok := domain.Cmd.Obj.(*sourcesv1alpha1.GitRepository)
			Expect(ok).To(BeTrue())
			Expect(grObj.Spec.Contract.Repository.Owner).To(Equal("ntlaletsi70"))
			Expect(grObj.Spec.Contract.Repository.Name).To(Equal("for-kaniko-app"))
			// source-specific labels must be present
			Expect(grObj.Labels["sources.blanketops.dev/gitrepository"]).To(Equal("for-kaniko-app"))
			Expect(grObj.Labels["sources.blanketops.dev/provider"]).To(Equal("github-upjet"))
		})
	})

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("GitRepository not found", func() {
		It("returns no error when the GitRepository CR no longer exists", func() {
			domain := &FakeGitRepositoryDomain{}
			reconciler := newGitRepositoryReconciler(domain)

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