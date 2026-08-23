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

	networksv1alpha1 "github.com/blanketops/environments-api/api/networks/v1alpha1"
	envctrl "github.com/blanketops/environments-controller/pkg/controller/networks"
	envruntime "github.com/blanketops/environments-controller/pkg/runtime"
	"github.com/blanketops/environments/core/command"
	"github.com/blanketops/environments/core/engine"
	"github.com/blanketops/environments/core/registry"

	// test data — apimachinery types we built
	testdata "github.com/blanketops/environments-tests/tests/data/networks"
)

// Fake Route Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
//
// The Route controller reconciles a Domain CR as an owned child.
// Route owns the workload binding (host, path, runtime).
// Domain owns the cert/mapping chain for the host.
// We test only that the domain receives the correct CR.

type FakeRouteDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeRouteDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeRouteDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeRouteDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeRouteDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeRouteDomain) GVK() schema.GroupVersionKind {
	return networksv1alpha1.GroupVersion.WithKind("Route")
}

// Note: domainTestLogger declared in domain_controller_test.go — same package.

// Test helpers

func newRouteReconciler(domain *FakeRouteDomain) *envctrl.RouteReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		networksv1alpha1.GroupVersion.WithKind("Route"),
		domain,
	)

	eng := engine.NewEngine(reg, domainTestLogger)

	return &envctrl.RouteReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      domainTestLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// routeFromData converts a testdata.RouteData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// Route.Spec is an opaque contract (Spec.Contract runtime.RawExtension) —
// the resolver parses it as raw JSON, not typed Go struct fields.
// testdata's RouteSpec already carries the correct json tags for that raw
// contract shape, so this just marshals it directly.
func routeFromData(d *testdata.RouteData) *networksv1alpha1.Route {
	raw, err := json.Marshal(d.Spec)
	if err != nil {
		panic(fmt.Sprintf("marshal route contract: %v", err))
	}
	return &networksv1alpha1.Route{
		ObjectMeta: d.ObjectMeta,
		Spec: networksv1alpha1.RouteSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// Route Controller Tests

var _ = Describe("Route Controller", func() {
	ctx := context.Background()

	// Route CRs are namespace-scoped — tenant namespace
	const testNamespace = "dev"

	// BASIC BEHAVIOUR

	Context("Basic reconciliation behaviour", func() {
		const name = "shopping-app-route-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			r := &networksv1alpha1.Route{}
			if err := k8sClient.Get(ctx, key, r); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, routeFromData(
						testdata.NewRouteData(name, testNamespace, "api.dev.domain.co.za", "for-kaniko-app"),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			r := &networksv1alpha1.Route{}
			if err := k8sClient.Get(ctx, key, r); err == nil {
				_ = k8sClient.Delete(ctx, r)
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newRouteReconciler(&FakeRouteDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the Route update through the engine to the Route domain", func() {
			domain := &FakeRouteDomain{}
			reconciler := newRouteReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("Route"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Route multiple times", func() {
			domain := &FakeRouteDomain{}
			reconciler := newRouteReconciler(domain)

			for range 2 {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the Route domain fails", func() {
			domain := &FakeRouteDomain{
				Err: fmt.Errorf("route domain failure"),
			}
			reconciler := newRouteReconciler(domain)

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

	// TLS + RUNTIME ROUTING
	// Route fields: host, path, enabled, tlsEnabled, runtime.
	// Not build strategies — fixed from the copy-paste.
	//
	// TLS:
	//   *.dev.domain.co.za → platform wildcard (DNS01, pre-issued)
	//   client-a.co.za     → custom per-tenant (HTTP01)
	//
	// Runtime:
	//   kubernetes.io/container-runtime — default, Kourier or nginx
	//   knative.io/serving              — scale-to-zero, DomainMapping

	Context("Route TLS and runtime routing", func() {
		type routeTestCase struct {
			name       string
			host       string
			path       string
			tlsEnabled bool
			enabled    bool
			runtime    testdata.RouteRuntime
		}

		DescribeTable(
			"routes Route with correct TLS and runtime configuration to the domain",
			func(tc routeTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewRouteData(tc.name, testNamespace, tc.host, "for-kaniko-app")
				d.Spec.Path = tc.path
				d.Spec.TLSEnabled = tc.tlsEnabled
				d.Spec.Enabled = tc.enabled
				d.Spec.Runtime = tc.runtime

				Expect(k8sClient.Create(ctx, routeFromData(d))).To(Succeed())
				DeferCleanup(func() {
					r := &networksv1alpha1.Route{}
					if err := k8sClient.Get(ctx, key, r); err == nil {
						_ = k8sClient.Delete(ctx, r)
						_, _ = newRouteReconciler(&FakeRouteDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeRouteDomain{}
				reconciler := newRouteReconciler(domain)

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
				Expect(cmd.GVK.Kind).To(Equal("Route"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				routeObj, ok := cmd.Obj.(*networksv1alpha1.Route)
				Expect(ok).To(BeTrue())

				var contract testdata.RouteSpec
				Expect(json.Unmarshal(routeObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(contract.Host).To(Equal(tc.host))
				Expect(contract.Path).To(Equal(tc.path))
				Expect(contract.TLSEnabled).To(Equal(tc.tlsEnabled))
				Expect(contract.Enabled).To(Equal(tc.enabled))
				Expect(string(contract.Runtime)).To(Equal(string(tc.runtime)))
			},

			// Matches real CR: shopping-app-route
			Entry("platform domain — TLS enabled, kubernetes runtime", routeTestCase{
				name:       "route-platform-tls",
				host:       "api.dev.domain.co.za",
				path:       "/",
				tlsEnabled: true,
				enabled:    true,
				runtime:    testdata.RouteRuntimeKubernetes,
			}),
			Entry("custom domain — TLS enabled, kubernetes runtime", routeTestCase{
				name:       "route-custom-tls",
				host:       "app.client-a.co.za",
				path:       "/",
				tlsEnabled: true,
				enabled:    true,
				runtime:    testdata.RouteRuntimeKubernetes,
			}),
			Entry("platform domain — HTTP only, no TLS", routeTestCase{
				name:       "route-http-only",
				host:       "api.dev.domain.co.za",
				path:       "/",
				tlsEnabled: false,
				enabled:    true,
				runtime:    testdata.RouteRuntimeKubernetes,
			}),
			Entry("platform domain — Knative runtime", routeTestCase{
				name:       "route-knative",
				host:       "api.dev.domain.co.za",
				path:       "/",
				tlsEnabled: true,
				enabled:    true,
				runtime:    testdata.RouteRuntimeKnative,
			}),
			Entry("platform domain — disabled route", routeTestCase{
				name:       "route-disabled",
				host:       "api.dev.domain.co.za",
				path:       "/",
				tlsEnabled: true,
				enabled:    false,
				runtime:    testdata.RouteRuntimeKubernetes,
			}),
		)
	})

	// PATH ROUTING

	Context("Route path routing", func() {
		It("carries the path through to the domain for sub-path routes", func() {
			const name = "route-sub-path"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewRouteData(name, testNamespace, "api.dev.domain.co.za", "for-kaniko-app")
			d.Spec.Path = "/api/v1"
			Expect(k8sClient.Create(ctx, routeFromData(d))).To(Succeed())
			DeferCleanup(func() {
				r := &networksv1alpha1.Route{}
				if err := k8sClient.Get(ctx, key, r); err == nil {
					_ = k8sClient.Delete(ctx, r)
					_, _ = newRouteReconciler(&FakeRouteDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			})

			domain := &FakeRouteDomain{}
			reconciler := newRouteReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			routeObj, ok := domain.Cmd.Obj.(*networksv1alpha1.Route)
			Expect(ok).To(BeTrue())

			var contract testdata.RouteSpec
			Expect(json.Unmarshal(routeObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(contract.Path).To(Equal("/api/v1"))
			Expect(contract.Host).To(Equal("api.dev.domain.co.za"))
		})
	})

	// NOT FOUND — CR deleted before reconcile loop runs

	Context("Route not found", func() {
		It("returns no error when the Route CR no longer exists", func() {
			domain := &FakeRouteDomain{}
			reconciler := newRouteReconciler(domain)

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
