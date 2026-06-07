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
// Fake Package Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakePackageDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakePackageDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakePackageDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakePackageDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakePackageDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakePackageDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Package")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakePackageDomain) *PackageReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Package"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &PackageReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validPackage(name string, strategy string) *environmentsv1alpha1.Package {
	return &environmentsv1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.PackageSpec{
			Image: "docker.io/example/app:latest",

			Strategy: environmentsv1alpha1.Strategy{
				Kind: "ClusterPackageStrategy",
				Name: strategy,
			},

			Source: environmentsv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name: "Package-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Package Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Package Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-Package-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			Package := &environmentsv1alpha1.Package{}
			if err := k8sClient.Get(ctx, key, Package); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validPackage(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Package := &environmentsv1alpha1.Package{}
			if err := k8sClient.Get(ctx, key, Package); err == nil {
				_ = k8sClient.Delete(ctx, Package)
			}
		})

		It("routes the Package update through the engine to the Package domain", func() {
			domain := &FakePackageDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Package"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Package multiple times", func() {
			domain := &FakePackageDomain{}
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

		It("returns an error when the Package domain fails", func() {
			domain := &FakePackageDomain{
				Err: fmt.Errorf("Package domain failure"),
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

	Context("Package strategy routing", func() {
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
			"routes Package with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				Package := validPackage(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, Package)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, Package)
				})

				domain := &FakePackageDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Package"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				PackageObj, ok := cmd.Obj.(*environmentsv1alpha1.Package)
				Expect(ok).To(BeTrue())

				Expect(PackageObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(PackageObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "Package-kaniko",
				strategy: "kaniko",
			}),
			Entry("Packageah", testCase{
				name:     "Package-Packageah",
				strategy: "Packageah-shipwright-managed-push",
			}),
			Entry("Packagepacks", testCase{
				name:     "Package-Packagepacks",
				strategy: "Packagepacks-v3",
			}),
		)
	})
})
