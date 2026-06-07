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
// Fake Package Domain (correct seam)
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are all real.
// -----------------------------------------------------------------------------

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
	return environmentsv1.GroupVersion.WithKind("Package")
}

// Note: testLogger is declared in build_controller_test.go — same package, no redeclaration.

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newPackageReconciler(domain *FakePackageDomain) *PackageReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1.GroupVersion.WithKind("Package"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &PackageReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32),
	}
}

// packageFromData converts a testdata.PackageData into the API type
// the controller expects. Single conversion point — nothing scattered across tests.
func packageFromData(d *testdata.PackageData) *environmentsv1.Package {
	return &environmentsv1.Package{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1.PackageSpec{
			Contract: environmentsv1.PackageContract{
				Name:               d.Spec.Contract.Name,
				Version:            d.Spec.Contract.Version,
				Enabled:            d.Spec.Contract.Enabled,
				PackageName:        d.Spec.Contract.PackageName,
				PackageVersion:     d.Spec.Contract.PackageVersion,
				PackageDescription: d.Spec.Contract.PackageDescription,
				PackageMaintainers: func() []environmentsv1.PackageMaintainer {
					out := make([]environmentsv1.PackageMaintainer, len(d.Spec.Contract.PackageMaintainers))
					for i, m := range d.Spec.Contract.PackageMaintainers {
						out[i] = environmentsv1.PackageMaintainer{
							Name:  m.Name,
							Email: m.Email,
						}
					}
					return out
				}(),
				PackageRepository: environmentsv1.PackageRepository{
					URL:               d.Spec.Contract.PackageRepository.URL,
					CredentialsSecret: d.Spec.Contract.PackageRepository.CredentialsSecret,
				},
				PackageKappDiff: d.Spec.Contract.PackageKappDiff,
				StateRepo: environmentsv1.StateRepo{
					URL:         d.Spec.Contract.StateRepo.URL,
					Ref:         environmentsv1.StateRepoRef{Branch: d.Spec.Contract.StateRepo.Ref.Branch},
					CloneSecret: d.Spec.Contract.StateRepo.CloneSecret,
					Strategy:    d.Spec.Contract.StateRepo.Strategy,
					Path:        d.Spec.Contract.StateRepo.Path,
				},
			},
		},
	}
}

// -----------------------------------------------------------------------------
// Package Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Package Controller", func() {
	ctx := context.Background()

	const testNamespace = "default"

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-package-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: testNamespace,
		}

		BeforeEach(func() {
			pkg := &environmentsv1.Package{}
			if err := k8sClient.Get(ctx, key, pkg); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, packageFromData(
						testdata.NewPackageData(name, testNamespace),
					)),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			pkg := &environmentsv1.Package{}
			if err := k8sClient.Get(ctx, key, pkg); err == nil {
				_ = k8sClient.Delete(ctx, pkg)
			}
		})

		It("routes the Package update through the engine to the Package domain", func() {
			domain := &FakePackageDomain{}
			reconciler := newPackageReconciler(domain)

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
			reconciler := newPackageReconciler(domain)

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
				Err: fmt.Errorf("package domain failure"),
			}
			reconciler := newPackageReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).To(HaveOccurred())
			Expect(domain.Called).To(BeTrue())
		})
	})

	// -------------------------------------------------------------------------
	// RECONCILIATION STRATEGY ROUTING
	// Package strategies are kustomization | helm —
	// not build strategies. Fixed from the copy-paste.
	// -------------------------------------------------------------------------

	Context("Package reconciliation strategy routing", func() {
		type packageTestCase struct {
			name        string
			strategy    string
			kappDiff    bool
			stateBranch string
			statePath   string
		}

		DescribeTable(
			"routes Package with correct reconciliation strategy to the domain",
			func(tc packageTestCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: testNamespace,
				}

				d := testdata.NewPackageData(tc.name, testNamespace)
				d.Spec.Contract.StateRepo.Strategy = tc.strategy
				d.Spec.Contract.PackageKappDiff = tc.kappDiff
				d.Spec.Contract.StateRepo.Ref.Branch = tc.stateBranch
				d.Spec.Contract.StateRepo.Path = tc.statePath

				Expect(k8sClient.Create(ctx, packageFromData(d))).To(Succeed())
				DeferCleanup(func() {
					pkg := &environmentsv1.Package{}
					if err := k8sClient.Get(ctx, key, pkg); err == nil {
						_ = k8sClient.Delete(ctx, pkg)
					}
				})

				domain := &FakePackageDomain{}
				reconciler := newPackageReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Package"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				pkgObj, ok := cmd.Obj.(*environmentsv1.Package)
				Expect(ok).To(BeTrue())
				Expect(pkgObj.Spec.Contract.StateRepo.Strategy).To(Equal(tc.strategy))
				Expect(pkgObj.Spec.Contract.PackageKappDiff).To(Equal(tc.kappDiff))
				Expect(pkgObj.Spec.Contract.StateRepo.Ref.Branch).To(Equal(tc.stateBranch))
				Expect(pkgObj.Spec.Contract.StateRepo.Path).To(Equal(tc.statePath))
				Expect(pkgObj.Spec.Contract.PackageRepository.CredentialsSecret).To(Equal("git-ssh-credentials-packages"))
			},

			Entry("kustomization — kapp-diff enabled", packageTestCase{
				name:        "pkg-kustomize-diff",
				strategy:    "kustomization",
				kappDiff:    true,
				stateBranch: "master",
				statePath:   "./clusters/dev",
			}),
			Entry("kustomization — kapp-diff disabled", packageTestCase{
				name:        "pkg-kustomize-no-diff",
				strategy:    "kustomization",
				kappDiff:    false,
				stateBranch: "master",
				statePath:   "./clusters/dev",
			}),
			Entry("kustomization — staging branch", packageTestCase{
				name:        "pkg-kustomize-staging",
				strategy:    "kustomization",
				kappDiff:    true,
				stateBranch: "main",
				statePath:   "./clusters/staging",
			}),
		)
	})

	// -------------------------------------------------------------------------
	// ENABLED / DISABLED
	// -------------------------------------------------------------------------

	Context("Package enabled flag", func() {
		It("routes a disabled Package through the domain — reconciliation suspended, not deleted", func() {
			const name = "pkg-disabled"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewDisabledPackageData(name, testNamespace)
			Expect(k8sClient.Create(ctx, packageFromData(d))).To(Succeed())
			DeferCleanup(func() {
				pkg := &environmentsv1.Package{}
				if err := k8sClient.Get(ctx, key, pkg); err == nil {
					_ = k8sClient.Delete(ctx, pkg)
				}
			})

			domain := &FakePackageDomain{}
			reconciler := newPackageReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			pkgObj, ok := domain.Cmd.Obj.(*environmentsv1.Package)
			Expect(ok).To(BeTrue())
			// Domain receives the disabled Package — suspension logic lives in domain
			Expect(pkgObj.Spec.Contract.Enabled).To(BeFalse())
		})
	})

	// -------------------------------------------------------------------------
	// MAINTAINER AND METADATA
	// -------------------------------------------------------------------------

	Context("Package maintainer and metadata", func() {
		It("carries maintainer metadata through to the domain", func() {
			const name = "pkg-metadata"
			key := types.NamespacedName{Name: name, Namespace: testNamespace}

			d := testdata.NewPackageData(name, testNamespace)
			Expect(k8sClient.Create(ctx, packageFromData(d))).To(Succeed())
			DeferCleanup(func() {
				pkg := &environmentsv1.Package{}
				if err := k8sClient.Get(ctx, key, pkg); err == nil {
					_ = k8sClient.Delete(ctx, pkg)
				}
			})

			domain := &FakePackageDomain{}
			reconciler := newPackageReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())

			pkgObj, ok := domain.Cmd.Obj.(*environmentsv1.Package)
			Expect(ok).To(BeTrue())
			Expect(pkgObj.Spec.Contract.PackageMaintainers).To(HaveLen(1))
			Expect(pkgObj.Spec.Contract.PackageMaintainers[0].Name).To(Equal("Neo"))
			Expect(pkgObj.Spec.Contract.PackageMaintainers[0].Email).To(Equal("neo@blanketops.online"))
			Expect(pkgObj.Spec.Contract.PackageVersion).To(Equal("v1.2.3"))
		})
	})

	// -------------------------------------------------------------------------
	// NOT FOUND — CR deleted before reconcile loop runs
	// -------------------------------------------------------------------------

	Context("Package not found", func() {
		It("returns no error when the Package CR no longer exists", func() {
			domain := &FakePackageDomain{}
			reconciler := newPackageReconciler(domain)

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