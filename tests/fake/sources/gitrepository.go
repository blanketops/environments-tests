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
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sourcesv1alpha1 "github.com/blanketops/environments-api/api/sources/v1alpha1"
	envctrl "github.com/blanketops/environments-controller/pkg/controller/sources"
	envruntime "github.com/blanketops/environments-controller/pkg/runtime"
	"github.com/blanketops/environments/core/command"
	"github.com/blanketops/environments/core/engine"
	"github.com/blanketops/environments/core/registry"

	// test data — apimachinery types we built
	testdata "github.com/blanketops/environments-tests/tests/data/sources"
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
	Cmd    command.Command
	Err    error
}

func (f *FakeGitRepositoryDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeGitRepositoryDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeGitRepositoryDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeGitRepositoryDomain) Handle(ctx context.Context, cmd command.Command) error {
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

func newGitRepositoryReconciler(domain *FakeGitRepositoryDomain) *envctrl.GitRepositoryReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		sourcesv1alpha1.GroupVersion.WithKind("GitRepository"),
		domain,
	)

	eng := engine.NewEngine(reg, gitRepositoryTestLogger)

	return &envctrl.GitRepositoryReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      gitRepositoryTestLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// gitRepositoryFromData converts a testdata.GitRepositoryData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// GitRepository.Spec is an opaque contract (Spec.Contract
// runtime.RawExtension) — the resolver parses it as raw JSON, not typed Go
// struct fields. testdata's GitRepositoryContract already carries the
// correct json tags for that raw contract shape, so this just marshals it
// directly.
func gitRepositoryFromData(d *testdata.GitRepositoryData) *sourcesv1alpha1.GitRepository {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal gitrepository contract: %v", err))
	}
	om := d.ObjectMeta
	// GitRepository is namespace-scoped; testdata doesn't carry a namespace,
	// so set it here to match the fixed namespace these tests run in.
	om.Namespace = "default"
	return &sourcesv1alpha1.GitRepository{
		ObjectMeta: om,
		Spec: sourcesv1alpha1.GitRepositorySpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// -----------------------------------------------------------------------------
// GitRepository Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("GitRepository Controller", func() {
	ctx := context.Background()

	// GitRepository CRs are namespace-scoped.
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
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newGitRepositoryReconciler(&FakeGitRepositoryDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the GitRepository update through the engine to the GitRepository domain", func() {
			domain := &FakeGitRepositoryDomain{}
			reconciler := newGitRepositoryReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("GitRepository"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
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
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the GitRepository domain fails", func() {
			domain := &FakeGitRepositoryDomain{
				Err: fmt.Errorf("gitrepository domain failure"),
			}
			reconciler := newGitRepositoryReconciler(domain)

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
						_, _ = newGitRepositoryReconciler(&FakeGitRepositoryDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeGitRepositoryDomain{}
				reconciler := newGitRepositoryReconciler(domain)

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
				Expect(cmd.GVK.Kind).To(Equal("GitRepository"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				grObj, ok := cmd.Obj.(*sourcesv1alpha1.GitRepository)
				Expect(ok).To(BeTrue())

				var contract testdata.GitRepositoryContract
				Expect(json.Unmarshal(grObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(contract.Provider).To(Equal("github"))
				Expect(contract.HookURL).To(Equal(tc.hookURL))
				Expect(contract.Repository.Owner).To(Equal("ntlaletsi70"))
				Expect(contract.Webhooks).To(HaveLen(1))
				Expect(contract.Webhooks[0].Events).To(HaveLen(len(tc.webhookEvents)))
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
					_, _ = newGitRepositoryReconciler(&FakeGitRepositoryDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			})

			domain := &FakeGitRepositoryDomain{}
			reconciler := newGitRepositoryReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			grObj, ok := domain.Cmd.Obj.(*sourcesv1alpha1.GitRepository)
			Expect(ok).To(BeTrue())

			var contract testdata.GitRepositoryContract
			Expect(json.Unmarshal(grObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(contract.Repository.Owner).To(Equal("ntlaletsi70"))
			Expect(contract.Repository.Name).To(Equal("for-kaniko-app"))
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
