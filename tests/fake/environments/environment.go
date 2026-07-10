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

	environmentsv1alpha1 "github.com/BlanketOps/environments-api/api/environments/v1alpha1"
	"github.com/ntlaletsi70/blanketops-environments/core"
)

// -----------------------------------------------------------------------------
// Fake Environment Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeEnvironmentDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeEnvironmentDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeEnvironmentDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeEnvironmentDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeEnvironmentDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeEnvironmentDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Environment")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeEnvironmentDomain) *EnvironmentReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Environment"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &EnvironmentReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

func validEnvironment(name string, strategy string) *environmentsv1alpha1.Environment {
	return &environmentsv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: environmentsv1alpha1.EnvironmentSpec{
			Image: "docker.io/example/app:latest",

			Strategy: environmentsv1alpha1.Strategy{
				Kind: "ClusterEnvironmentStrategy",
				Name: strategy,
			},

			Source: environmentsv1alpha1.GitSource{
				URL:      "https://github.com/example/repo.git",
				Revision: "main",
				Context: "."
			},
			ServiceAccount: environmentsv1alpha1.ServiceAccount{
				Name: "Environment-bot",
				Secret: "docker-registry"
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Environment Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Environment Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-Environment-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			Environment := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, key, Environment); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validEnvironment(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Environment := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, key, Environment); err == nil {
				_ = k8sClient.Delete(ctx, Environment)
			}
		})

		It("routes the Environment update through the engine to the Environment domain", func() {
			domain := &FakeEnvironmentDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Environment"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Environment multiple times", func() {
			domain := &FakeEnvironmentDomain{}
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

		It("returns an error when the Environment domain fails", func() {
			domain := &FakeEnvironmentDomain{
				Err: fmt.Errorf("Environment domain failure"),
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

	Context("Environment strategy routing", func() {
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
			"routes Environment with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				Environment := validEnvironment(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, Environment)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, Environment)
				})

				domain := &FakeEnvironmentDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Environment"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				EnvironmentObj, ok := cmd.Obj.(*environmentsv1alpha1.Environment)
				Expect(ok).To(BeTrue())

				Expect(EnvironmentObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(EnvironmentObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "Environment-kaniko",
				strategy: "kaniko",
			}),
			Entry("Environmentah", testCase{
				name:     "Environment-Environmentah",
				strategy: "Environmentah-shipwright-managed-push",
			}),
			Entry("Environmentpacks", testCase{
				name:     "Environment-Environmentpacks",
				strategy: "Environmentpacks-v3",
			}),
		)
	})
})
