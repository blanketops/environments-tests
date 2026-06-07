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

	networksv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/networks/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments-mvp/core"
)

// -----------------------------------------------------------------------------
// Fake Domain Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeDomainDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeDomainDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeDomainDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeDomainDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeDomainDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeDomainDomain) GVK() schema.GroupVersionKind {
	return networksv1alpha1.GroupVersion.WithKind("Domain")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeDomainDomain) *DomainReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		networksv1alpha1.GroupVersion.WithKind("Domain"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &DomainReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validDomain(name string, strategy string) *networksv1alpha1.Domain {
	return &networksv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: networksv1alpha1.DomainSpec{
			Image: "docker.io/example/app:latest",

			Strategy: networksv1alpha1.Strategy{
				Kind: "ClusterDomainStrategy",
				Name: strategy,
			},

			Source: networksv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: networksv1alpha1.ServiceAccount{
				Name: "Domain-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Domain Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Domain Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-Domain-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			Domain := &networksv1alpha1.Domain{}
			if err := k8sClient.Get(ctx, key, Domain); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validDomain(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Domain := &networksv1alpha1.Domain{}
			if err := k8sClient.Get(ctx, key, Domain); err == nil {
				_ = k8sClient.Delete(ctx, Domain)
			}
		})

		It("routes the Domain update through the engine to the Domain domain", func() {
			domain := &FakeDomainDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Domain"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Domain multiple times", func() {
			domain := &FakeDomainDomain{}
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

		It("returns an error when the Domain domain fails", func() {
			domain := &FakeDomainDomain{
				Err: fmt.Errorf("Domain domain failure"),
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

	Context("Domain strategy routing", func() {
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
			"routes Domain with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				Domain := validDomain(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, Domain)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, Domain)
				})

				domain := &FakeDomainDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Domain"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				DomainObj, ok := cmd.Obj.(*networksv1alpha1.Domain)
				Expect(ok).To(BeTrue())

				Expect(DomainObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(DomainObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "Domain-kaniko",
				strategy: "kaniko",
			}),
			Entry("Domainah", testCase{
				name:     "Domain-Domainah",
				strategy: "Domainah-shipwright-managed-push",
			}),
			Entry("Domainpacks", testCase{
				name:     "Domain-Domainpacks",
				strategy: "Domainpacks-v3",
			}),
		)
	})
})
