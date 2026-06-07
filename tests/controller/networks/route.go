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

package networks

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
// Fake Route Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeRouteDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeRouteDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeRouteDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeRouteDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeRouteDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeRouteDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Route")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeRouteDomain) *RouteReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Route"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &RouteReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validRoute(name string, strategy string) *environmentsv1alpha1.Route {
	return &environmentsv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.RouteSpec{
			Image: "docker.io/example/app:latest",

			Strategy: environmentsv1alpha1.Strategy{
				Kind: "ClusterRouteStrategy",
				Name: strategy,
			},

			Source: environmentsv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name: "Route-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Route Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Route Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-Route-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			Route := &environmentsv1alpha1.Route{}
			if err := k8sClient.Get(ctx, key, Route); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validRoute(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Route := &environmentsv1alpha1.Route{}
			if err := k8sClient.Get(ctx, key, Route); err == nil {
				_ = k8sClient.Delete(ctx, Route)
			}
		})

		It("routes the Route update through the engine to the Route domain", func() {
			domain := &FakeRouteDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Route"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Route multiple times", func() {
			domain := &FakeRouteDomain{}
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

		It("returns an error when the Route domain fails", func() {
			domain := &FakeRouteDomain{
				Err: fmt.Errorf("Route domain failure"),
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

	Context("Route strategy routing", func() {
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
			"routes Route with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				Route := validRoute(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, Route)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, Route)
				})

				domain := &FakeRouteDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Route"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				RouteObj, ok := cmd.Obj.(*environmentsv1alpha1.Route)
				Expect(ok).To(BeTrue())

				Expect(RouteObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(RouteObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "Route-kaniko",
				strategy: "kaniko",
			}),
			Entry("Routeah", testCase{
				name:     "Route-Routeah",
				strategy: "Routeah-shipwright-managed-push",
			}),
			Entry("Routepacks", testCase{
				name:     "Route-Routepacks",
				strategy: "Routepacks-v3",
			}),
		)
	})
})
