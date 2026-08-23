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

	"github.com/go-logr/logr"

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

// -----------------------------------------------------------------------------
// Fake Domain Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
//
// The Domain controller reconciles:
//   platform TLS: DomainClaim + DomainMapping (wildcard cert pre-issued)
//   custom  TLS:  Issuer + DomainClaim + DomainMapping + Certificate
// We test only that the domain receives the correct CR — child resource
// reconciliation is covered in integration tests.
// -----------------------------------------------------------------------------

type FakeDomainDomain struct {
	Called bool
	Cmd    command.Command
	Err    error
}

func (f *FakeDomainDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeDomainDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeDomainDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeDomainDomain) Handle(ctx context.Context, cmd command.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeDomainDomain) GVK() schema.GroupVersionKind {
	return networksv1alpha1.GroupVersion.WithKind("Domain")
}

// domainTestLogger is the discard logger for the networks package.
// Declared per-package — separate from environments and events packages.
var domainTestLogger = logr.Discard()

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newDomainReconciler(domain *FakeDomainDomain) *envctrl.DomainReconciler {
	reg := registry.NewRegistry()
	reg.RegisterDomain(
		networksv1alpha1.GroupVersion.WithKind("Domain"),
		domain,
	)

	eng := engine.NewEngine(reg, domainTestLogger)

	return &envctrl.DomainReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Runtime:  &envruntime.Runtime{Registry: reg, Engine: eng},
		Log:      domainTestLogger,
		Recorder: events.NewFakeRecorder(32),
	}
}

// domainFromData converts a testdata.DomainData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
//
// Domain.Spec is an opaque contract (Spec.Contract runtime.RawExtension) —
// the resolver parses it as raw JSON, not typed Go struct fields.
// testdata's DomainContract already carries the correct json tags for that
// raw contract shape, so this just marshals it directly.
func domainFromData(d *testdata.DomainData) *networksv1alpha1.Domain {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal domain contract: %v", err))
	}
	return &networksv1alpha1.Domain{
		ObjectMeta: d.ObjectMeta,
		Spec: networksv1alpha1.DomainSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// -----------------------------------------------------------------------------
// Domain Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Domain Controller", func() {
	ctx := context.Background()

	// Domain CRs are namespace-scoped — tenant namespace
	const testNamespace = "dev"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "domain-for-kaniko-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			d := &networksv1alpha1.Domain{}
			if err := k8sClient.Get(ctx, key, d); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, domainFromData(
						testdata.NewPlatformDomainData(
							name, testNamespace,
							"api.dev.domain.co.za",
							"shopping-app-route",
							"environment-sample",
						),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			d := &networksv1alpha1.Domain{}
			if err := k8sClient.Get(ctx, key, d); err == nil {
				_ = k8sClient.Delete(ctx, d)
				// The real reconciler's finalizer gate runs regardless of which
				// domain is injected — nothing else is watching this cluster
				// during these tests, so drive the delete path once more here
				// or the object is stuck in Terminating forever.
				_, _ = newDomainReconciler(&FakeDomainDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("routes the Domain update through the engine to the Domain domain", func() {
			domain := &FakeDomainDomain{}
			reconciler := newDomainReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.GVK.Kind).To(Equal("Domain"))
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Domain multiple times", func() {
			domain := &FakeDomainDomain{}
			reconciler := newDomainReconciler(domain)

			for i := 0; i < 2; i++ {
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(domain.Called).To(BeTrue())
			Expect(domain.Cmd.Type).To(Equal(command.CmdUpdate))
		})

		It("returns an error when the Domain domain fails", func() {
			domain := &FakeDomainDomain{
				Err: fmt.Errorf("domain domain failure"),
			}
			reconciler := newDomainReconciler(domain)

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
	// TLS STRATEGY ROUTING
	// Domain TLS strategies are platform | custom —
	// not build strategies. Fixed from the copy-paste.
	//
	// platform: *.dev.domain.co.za wildcard cert — DNS01, no cert work needed.
	//           Controller emits DomainClaim + DomainMapping only.
	// custom:   client-owned zone — HTTP01 challenge.
	//           Controller emits Issuer + DomainClaim + DomainMapping + Certificate.
	// -------------------------------------------------------------------------

	Context("Domain TLS strategy routing", func() {
		type domainTestCase struct {
			name         string
			host         string
			tlsStrategy  testdata.TLSStrategy
			mtlsEnforced bool
			renewBefore  string
		}

		DescribeTable(
			"routes Domain with correct TLS strategy to the domain",
			func(tc domainTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				var d *testdata.DomainData
				if tc.tlsStrategy == testdata.TLSStrategyPlatform {
					d = testdata.NewPlatformDomainData(
						tc.name, testNamespace, tc.host, tc.name+"-route", "environment-sample",
					)
				} else {
					d = testdata.NewCustomDomainData(
						tc.name, testNamespace, tc.host, tc.name+"-route", "environment-sample",
					)
				}
				d.Spec.Contract.MTLS.Enforced = tc.mtlsEnforced
				d.Spec.Contract.RenewBefore = tc.renewBefore

				Expect(k8sClient.Create(ctx, domainFromData(d))).To(Succeed())
				DeferCleanup(func() {
					dom := &networksv1alpha1.Domain{}
					if err := k8sClient.Get(ctx, key, dom); err == nil {
						_ = k8sClient.Delete(ctx, dom)
						_, _ = newDomainReconciler(&FakeDomainDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
					}
				})

				domain := &FakeDomainDomain{}
				reconciler := newDomainReconciler(domain)

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
				Expect(cmd.GVK.Kind).To(Equal("Domain"))
				Expect(cmd.Type).To(Equal(command.CmdUpdate))

				domObj, ok := cmd.Obj.(*networksv1alpha1.Domain)
				Expect(ok).To(BeTrue())

				var contract testdata.DomainContract
				Expect(json.Unmarshal(domObj.Spec.Contract.Raw, &contract)).To(Succeed())
				Expect(contract.Host).To(Equal(tc.host))
				Expect(string(contract.TLSStrategy)).To(Equal(string(tc.tlsStrategy)))
				Expect(contract.MTLS.Enforced).To(Equal(tc.mtlsEnforced))
				Expect(contract.RenewBefore).To(Equal(tc.renewBefore))
			},

			// Matches real CR: domain-for-kaniko-sample
			Entry("platform — wildcard DNS01, mTLS enforced", domainTestCase{
				name:         "domain-platform-mtls",
				host:         "api.dev.domain.co.za",
				tlsStrategy:  testdata.TLSStrategyPlatform,
				mtlsEnforced: true,
				renewBefore:  "720h",
			}),
			Entry("platform — wildcard DNS01, no mTLS", domainTestCase{
				name:         "domain-platform-no-mtls",
				host:         "api.dev.domain.co.za",
				tlsStrategy:  testdata.TLSStrategyPlatform,
				mtlsEnforced: false,
				renewBefore:  "720h",
			}),
			Entry("custom — client-owned zone, HTTP01, mTLS enforced", domainTestCase{
				name:         "domain-custom-mtls",
				host:         "app.client-a.co.za",
				tlsStrategy:  testdata.TLSStrategyCustom,
				mtlsEnforced: true,
				renewBefore:  "720h",
			}),
		)
	})

	// -------------------------------------------------------------------------
	// ROUTE REF
	// Domain must carry a valid routeRef — Route owns the workload binding.
	// Domain owns the cert/mapping chain for the host.
	// -------------------------------------------------------------------------

	Context("Domain routeRef", func() {
		It("carries the routeRef name through to the domain", func() {
			const name = "domain-routeref"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewPlatformDomainData(
				name, testNamespace,
				"api.dev.domain.co.za",
				"shopping-app-route",
				"environment-sample",
			)
			Expect(k8sClient.Create(ctx, domainFromData(d))).To(Succeed())
			DeferCleanup(func() {
				dom := &networksv1alpha1.Domain{}
				if err := k8sClient.Get(ctx, key, dom); err == nil {
					_ = k8sClient.Delete(ctx, dom)
					_, _ = newDomainReconciler(&FakeDomainDomain{}).Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			})

			domain := &FakeDomainDomain{}
			reconciler := newDomainReconciler(domain)

			// First reconcile on a brand-new object only adds the finalizer
			// and returns — the domain isn't invoked until the next pass.
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			domObj, ok := domain.Cmd.Obj.(*networksv1alpha1.Domain)
			Expect(ok).To(BeTrue())

			var contract testdata.DomainContract
			Expect(json.Unmarshal(domObj.Spec.Contract.Raw, &contract)).To(Succeed())
			Expect(contract.RouteRef.Name).To(Equal("shopping-app-route"))
			Expect(contract.Host).To(Equal("api.dev.domain.co.za"))
		})
	})

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("Domain not found", func() {
		It("returns no error when the Domain CR no longer exists", func() {
			domain := &FakeDomainDomain{}
			reconciler := newDomainReconciler(domain)

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
