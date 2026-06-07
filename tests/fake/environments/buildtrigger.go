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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	environmentsv1 "github.com/ntlaletsi70/blanketops-environments-api/api/environments/v1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"

	// test data — apimachinery types we built
	testdata "github.com/ntlaletsi70/blanketops-environments-tests/environments"
)

// -----------------------------------------------------------------------------
// Fake BuildTrigger Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
// -----------------------------------------------------------------------------

type FakeBuildTriggerDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeBuildTriggerDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeBuildTriggerDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeBuildTriggerDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeBuildTriggerDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeBuildTriggerDomain) GVK() schema.GroupVersionKind {
	return environmentsv1.GroupVersion.WithKind("BuildTrigger")
}

var buildTriggerTestLogger = logr.Discard()

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newBuildTriggerReconciler(domain *FakeBuildTriggerDomain) *BuildTriggerReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1.GroupVersion.WithKind("BuildTrigger"),
		domain,
	)

	engine := core.NewEngine(registry, buildTriggerTestLogger)

	return &BuildTriggerReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      buildTriggerTestLogger,
		Recorder: record.NewFakeRecorder(32),
	}
}

// buildTriggerFromData converts a testdata.BuildTriggerData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
func buildTriggerFromData(d *testdata.BuildTriggerData) *environmentsv1.BuildTrigger {
	return &environmentsv1.BuildTrigger{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1.BuildTriggerSpec{
			Source:    environmentsv1.BuildTriggerSource(d.Spec.Source),
			EventType: environmentsv1.BuildTriggerEventType(d.Spec.EventType),
			Repository: environmentsv1.Repository{
				Owner: d.Spec.Repository.Owner,
				Name:  d.Spec.Repository.Name,
			},
			Ref:     d.Spec.Ref,
			SHA:     d.Spec.SHA,
			Actor:   d.Spec.Actor,
			EventID: d.Spec.EventID,
			BuildRef: environmentsv1.BuildRef{
				Name: d.Spec.BuildRef.Name,
			},
			PayloadPolicy: environmentsv1.PayloadPolicy{
				Allow: d.Spec.PayloadPolicy.Allow,
			},
		},
	}
}

// -----------------------------------------------------------------------------
// BuildTrigger Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("BuildTrigger Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-buildtrigger-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			bt := &environmentsv1.BuildTrigger{}
			if err := k8sClient.Get(ctx, key, bt); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, buildTriggerFromData(
						testdata.NewBuildTriggerData(name, testNamespace),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			bt := &environmentsv1.BuildTrigger{}
			if err := k8sClient.Get(ctx, key, bt); err == nil {
				_ = k8sClient.Delete(ctx, bt)
			}
		})

		It("routes the BuildTrigger through the engine to the BuildTrigger domain", func() {
			domain := &FakeBuildTriggerDomain{}
			reconciler := newBuildTriggerReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("BuildTrigger"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same BuildTrigger multiple times", func() {
			domain := &FakeBuildTriggerDomain{}
			reconciler := newBuildTriggerReconciler(domain)

			for i := 0; i < 2; i++ {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
		})

		It("returns an error when the BuildTrigger domain fails", func() {
			domain := &FakeBuildTriggerDomain{
				Err: fmt.Errorf("buildtrigger domain failure"),
			}
			reconciler := newBuildTriggerReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// -------------------------------------------------------------------------
	// EVENT TYPE ROUTING
	// -------------------------------------------------------------------------

	Context("BuildTrigger event type routing", func() {
		type triggerTestCase struct {
			name      string
			eventType testdata.BuildTriggerEventType
			ref       string
			sha       string
		}

		DescribeTable(
			"routes BuildTrigger with correct event type to the domain",
			func(tc triggerTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewBuildTriggerData(tc.name, testNamespace)
				d.Spec.EventType = tc.eventType
				d.Spec.Ref = tc.ref
				d.Spec.SHA = tc.sha

				Expect(k8sClient.Create(ctx, buildTriggerFromData(d))).To(Succeed())
				DeferCleanup(func() {
					bt := &environmentsv1.BuildTrigger{}
					if err := k8sClient.Get(ctx, key, bt); err == nil {
						_ = k8sClient.Delete(ctx, bt)
					}
				})

				domain := &FakeBuildTriggerDomain{}
				reconciler := newBuildTriggerReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("BuildTrigger"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				btObj, ok := cmd.Obj.(*environmentsv1.BuildTrigger)
				Expect(ok).To(BeTrue())
				Expect(string(btObj.Spec.EventType)).To(Equal(string(tc.eventType)))
				Expect(btObj.Spec.Ref).To(Equal(tc.ref))
				Expect(btObj.Spec.BuildRef.Name).To(Equal("build-for-kaniko-sample"))
			},

			Entry("push to main", triggerTestCase{
				name:      "bt-push-main",
				eventType: testdata.BuildTriggerEventTypePush,
				ref:       "refs/heads/main",
				sha:       "abc1234567890def",
			}),
			Entry("pull request", triggerTestCase{
				name:      "bt-pull-request",
				eventType: testdata.BuildTriggerEventTypePullRequest,
				ref:       "refs/pull/42/merge",
				sha:       "",
			}),
			Entry("manual trigger", triggerTestCase{
				name:      "bt-manual",
				eventType: testdata.BuildTriggerEventTypeManual,
				ref:       "refs/heads/main",
				sha:       "",
			}),
		)
	})

	// -------------------------------------------------------------------------
	// PAYLOAD POLICY
	// -------------------------------------------------------------------------

	Context("BuildTrigger payload policy", func() {
		It("routes a gated trigger (allow=false) through the domain for rejection handling", func() {
			const name = "bt-rejected"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewRejectedBuildTriggerData(name, testNamespace)
			Expect(k8sClient.Create(ctx, buildTriggerFromData(d))).To(Succeed())
			DeferCleanup(func() {
				bt := &environmentsv1.BuildTrigger{}
				if err := k8sClient.Get(ctx, key, bt); err == nil {
					_ = k8sClient.Delete(ctx, bt)
				}
			})

			domain := &FakeBuildTriggerDomain{}
			reconciler := newBuildTriggerReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			btObj, ok := domain.Cmd.Obj.(*environmentsv1.BuildTrigger)
			Expect(ok).To(BeTrue())
			// Domain receives the trigger regardless of allow flag —
			// rejection logic lives in the domain, not the controller
			Expect(btObj.Spec.PayloadPolicy.Allow).To(BeFalse())
		})

		It("routes a manual trigger (no SHA, no EventID) through the domain", func() {
			const name = "bt-manual-policy"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewManualBuildTriggerData(name, testNamespace)
			Expect(k8sClient.Create(ctx, buildTriggerFromData(d))).To(Succeed())
			DeferCleanup(func() {
				bt := &environmentsv1.BuildTrigger{}
				if err := k8sClient.Get(ctx, key, bt); err == nil {
					_ = k8sClient.Delete(ctx, bt)
				}
			})

			domain := &FakeBuildTriggerDomain{}
			reconciler := newBuildTriggerReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			btObj, ok := domain.Cmd.Obj.(*environmentsv1.BuildTrigger)
			Expect(ok).To(BeTrue())
			Expect(string(btObj.Spec.Source)).To(Equal(string(testdata.BuildTriggerSourceManual)))
			Expect(btObj.Spec.SHA).To(BeEmpty())
			Expect(btObj.Spec.EventID).To(BeEmpty())
			Expect(btObj.Spec.Actor).To(Equal("platform-operator"))
		})
	})

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("BuildTrigger not found", func() {
		It("returns no error when the BuildTrigger CR no longer exists", func() {
			domain := &FakeBuildTriggerDomain{}
			reconciler := newBuildTriggerReconciler(domain)

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