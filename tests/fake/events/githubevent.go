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

	"github.com/go-logr/logr"

	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	envctrl "github.com/blanketops/environments-controller/pkg/controller/events"
	envruntime "github.com/blanketops/environments-controller/pkg/runtime"
	"github.com/blanketops/environments/core/command"
	"github.com/blanketops/environments/core/engine"
	"github.com/blanketops/environments/core/registry"

	// test data — apimachinery types we built
	testdata "github.com/blanketops/environments-tests/tests/data/events"
)

// Fake GitHubEvent Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
//
// The GitHubEvent controller reconciles three child Argo Events resources:
//   EventSource, EventBus, Sensor — all owned by the GitHubEvent CR.
// We test only that the domain receives the correct CR — child resource
// reconciliation is covered in integration tests.

type FakeGitHubEventDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeGitHubEventDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeGitHubEventDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeGitHubEventDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeGitHubEventDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeGitHubEventDomain) GVK() schema.GroupVersionKind {
	return eventsv1alpha1.GroupVersion.WithKind("GitHubEvent")
}

// githubEventTestLogger is the discard logger for the events package.
// Declared per-package — separate from the environments package testLogger.
var githubEventTestLogger = logr.Discard()

// Test helpers

func newGitHubEventReconciler(domain *FakeGitHubEventDomain) *envctrl.GitHubEventReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		eventsv1alpha1.GroupVersion.WithKind("GitHubEvent"),
		domain,
	)

	eng := engine.NewEngine(reg, githubEventTestLogger)

	return &envctrl.GitHubEventReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      githubEventTestLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// gitHubEventFromData converts a testdata.GitHubEventData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// GitHubEvent.Spec is an opaque contract (Spec.Contract runtime.RawExtension)
// — the resolver parses it as raw JSON, not typed Go struct fields.
// testdata's GitHubEventContract already carries the correct json tags for
// that raw contract shape, so this just marshals it directly.
func gitHubEventFromData(d *testdata.GitHubEventData) *eventsv1alpha1.GitHubEvent {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal githubevent contract: %v", err))
	}
	return &eventsv1alpha1.GitHubEvent{
		ObjectMeta: d.ObjectMeta,
		Spec: eventsv1alpha1.GitHubEventSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// GitHubEvent Controller Tests

var _ = Describe("GitHubEvent Controller", func() {
	ctx := context.Background()

	// GitHubEvent CRs live in argo-events namespace —
	// Argo Events controller watches this namespace.
	const testNamespace = "argo-events"

	// BASIC BEHAVIOUR

	Context("Basic reconciliation behaviour", func() {
		const name = "push-main-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			ge := &eventsv1alpha1.GitHubEvent{}
			if err := k8sClient.Get(ctx, key, ge); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, gitHubEventFromData(
						testdata.NewGitHubEventData(name, "for-kaniko-app"),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			ge := &eventsv1alpha1.GitHubEvent{}
			if err := k8sClient.Get(ctx, key, ge); err == nil {
				_ = k8sClient.Delete(ctx, ge)
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newGitHubEventReconciler(&FakeGitHubEventDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the GitHubEvent update through the engine to the GitHubEvent domain", func() {
			domain := &FakeGitHubEventDomain{}
			reconciler := newGitHubEventReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("GitHubEvent"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same GitHubEvent multiple times", func() {
			domain := &FakeGitHubEventDomain{}
			reconciler := newGitHubEventReconciler(domain)

			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the GitHubEvent domain fails", func() {
			domain := &FakeGitHubEventDomain{
				Err: fmt.Errorf("githubevent domain failure"),
			}
			reconciler := newGitHubEventReconciler(domain)

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

	// EVENT TYPE ROUTING
	// GitHubEvent types are push | pull_request —
	// not build strategies. Fixed from the copy-paste.

	Context("GitHubEvent type routing", func() {
		type gitHubEventTestCase struct {
			name      string
			eventType testdata.GitHubEventType
			ref       string
			envName   string
		}

		DescribeTable(
			"routes GitHubEvent with correct event type and ref to the domain",
			func(tc gitHubEventTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewGitHubEventData(tc.name, tc.envName)
				d.Spec.Contract.EventType = tc.eventType
				d.Spec.Contract.Ref = tc.ref

				Expect(k8sClient.Create(ctx, gitHubEventFromData(d))).To(Succeed())
				DeferCleanup(func() {
					ge := &eventsv1alpha1.GitHubEvent{}
					if err := k8sClient.Get(ctx, key, ge); err == nil {
						_ = k8sClient.Delete(ctx, ge)
						_, _ = newGitHubEventReconciler(&FakeGitHubEventDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeGitHubEventDomain{}
				reconciler := newGitHubEventReconciler(domain)

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
				Expect(cmd.GVK.Kind).To(Equal("GitHubEvent"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				geObj, ok := cmd.Obj.(*eventsv1alpha1.GitHubEvent)
				Expect(ok).To(BeTrue())

				var contract testdata.GitHubEventContract
				Expect(json.Unmarshal(geObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(string(contract.EventType)).To(Equal(string(tc.eventType)))
				Expect(contract.Ref).To(Equal(tc.ref))
				// Repository must always be owner/name format
				Expect(contract.Repository).To(Equal("ntlaletsi70/" + tc.envName))
				// Webhook secret ref must be present
				Expect(contract.Webhook.SecretRef.Name).To(Equal("github-webhook-secret"))
				Expect(contract.Webhook.SecretRef.Key).To(Equal("secret"))
			},

			// Matches real CR: push-main-001
			Entry("push to main", gitHubEventTestCase{
				name:      "ge-push-main",
				eventType: testdata.GitHubEventTypePush,
				ref:       "refs/heads/main",
				envName:   "for-kaniko-app",
			}),
			// master ref — matches BuildTrigger ref in for-kaniko-app pipeline
			Entry("push to master", gitHubEventTestCase{
				name:      "ge-push-master",
				eventType: testdata.GitHubEventTypePush,
				ref:       "refs/heads/master",
				envName:   "for-kaniko-app",
			}),
			Entry("pull request", gitHubEventTestCase{
				name:      "ge-pull-request",
				eventType: testdata.GitHubEventTypePullRequest,
				ref:       "refs/heads/main",
				envName:   "for-kaniko-app",
			}),
		)
	})

	// WEBHOOK SECRET REF
	// Controller must not process events without a webhook secret ref —
	// payload signature verification depends on it.

	Context("GitHubEvent webhook secret ref", func() {
		It("carries the webhook secretRef name and key through to the domain", func() {
			const name = "ge-webhook-secret"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewGitHubEventData(name, "for-kaniko-app")
			Expect(k8sClient.Create(ctx, gitHubEventFromData(d))).To(Succeed())
			DeferCleanup(func() {
				ge := &eventsv1alpha1.GitHubEvent{}
				if err := k8sClient.Get(ctx, key, ge); err == nil {
					_ = k8sClient.Delete(ctx, ge)
					_, _ = newGitHubEventReconciler(&FakeGitHubEventDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			})

			domain := &FakeGitHubEventDomain{}
			reconciler := newGitHubEventReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			geObj, ok := domain.Cmd.Obj.(*eventsv1alpha1.GitHubEvent)
			Expect(ok).To(BeTrue())

			var contract testdata.GitHubEventContract
			Expect(json.Unmarshal(geObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(contract.Webhook.SecretRef.Name).To(Equal("github-webhook-secret"))
			Expect(contract.Webhook.SecretRef.Key).To(Equal("secret"))
		})
	})

	// NOT FOUND — CR deleted before reconcile loop runs

	Context("GitHubEvent not found", func() {
		It("returns no error when the GitHubEvent CR no longer exists", func() {
			domain := &FakeGitHubEventDomain{}
			reconciler := newGitHubEventReconciler(domain)

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
