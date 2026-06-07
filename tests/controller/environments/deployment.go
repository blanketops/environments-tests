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
// Fake Deployment Domain (CORRECT seam)
// -----------------------------------------------------------------------------
//
// This is the ONLY thing we fake.
// The controller, registry, engine, and command flow are real.
type FakeDeploymentDomain struct {
	Called bool
	Cmd    core.Command
	Err    error
}

func (f *FakeDeploymentDomain) CanCreate(obj client.Object) bool { return true }
func (f *FakeDeploymentDomain) CanDelete(obj client.Object) bool { return true }
func (f *FakeDeploymentDomain) CanUpdate(obj client.Object, old client.Object) bool {
	return true
}

func (f *FakeDeploymentDomain) Handle(ctx context.Context, cmd core.Command) error {
	f.Called = true
	f.Cmd = cmd
	return f.Err
}

func (f *FakeDeploymentDomain) GVK() schema.GroupVersionKind {
	return environmentsv1alpha1.GroupVersion.WithKind("Deployment")
}

var testLogger = logr.Discard()

//
// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func newTestReconciler(domain *FakeDeploymentDomain) *DeploymentReconciler {
	registry := core.NewRegistry()
	registry.RegisterDomain(
		environmentsv1alpha1.GroupVersion.WithKind("Deployment"),
		domain,
	)

	engine := core.NewEngine(registry, testLogger)

	return &DeploymentReconciler{
		Client:   k8sClient,
		Scheme:   k8sClient.Scheme(),
		Engine:   engine,
		Log:      testLogger,
		Recorder: record.NewFakeRecorder(32), // ✅ REQUIRED
	}
}

// Object CR must reference construction from contract, we muust call blanketops-environments
func validDeployment(name string, strategy string) *environmentsv1alpha1.Deployment {
	return &environmentsv1alpha1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: environmentsv1alpha1.DeploymentSpec{
			ServiceUnits: ["for-kaniko-app-api"],
			Runtime: "kubernetes.io/container-runtime",
			Strategy: "Rolling",
			ImageAutomation: false,
			ReconciliationStrategy: "kustomize",
			GitOwner: "blanketops",
			ManifestsRepo: environmentsv1alpha1.ManifestsRepo{
				Url: "ssh://git@github.com/ntlaletsi70/for-kaniko-app-manifests.git",
				CloneSecret: "git-ssh-credentials",
				Strategy: "kustomization",
				Path:  "./overlays/dev",
			},
		},
	}
}

//
// -----------------------------------------------------------------------------
// Deployment Controller Tests
// -----------------------------------------------------------------------------

var _ = Describe("Deployment Controller", func() {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// BASIC BEHAVIOUR
	// -------------------------------------------------------------------------

	Context("Basic reconciliation behaviour", func() {
		const name = "test-Deployment-basic"

		key := types.NamespacedName{
			Name:      name,
			Namespace: "test",
		}

		BeforeEach(func() {
			Deployment := &environmentsv1alpha1.Deployment{}
			if err := k8sClient.Get(ctx, key, Deployment); errors.IsNotFound(err) {
				Expect(
					k8sClient.Create(ctx, validDeployment(name, "kaniko")),
				).To(Succeed())
			}
		})

		AfterEach(func() {
			Deployment := &environmentsv1alpha1.Deployment{}
			if err := k8sClient.Get(ctx, key, Deployment); err == nil {
				_ = k8sClient.Delete(ctx, Deployment)
			}
		})

		It("routes the Deployment update through the engine to the Deployment domain", func() {
			domain := &FakeDeploymentDomain{}
			reconciler := newTestReconciler(domain)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: key,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Called).To(BeTrue())

			Expect(domain.Cmd.GVK.Kind).To(Equal("Deployment"))
			Expect(domain.Cmd.Type).To(Equal(core.CmdUpdate))
			Expect(domain.Cmd.Obj).NotTo(BeNil())
		})

		It("is idempotent when reconciling the same Deployment multiple times", func() {
			domain := &FakeDeploymentDomain{}
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

		It("returns an error when the Deployment domain fails", func() {
			domain := &FakeDeploymentDomain{
				Err: fmt.Errorf("Deployment domain failure"),
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

	Context("Deployment strategy routing", func() {
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
			"routes Deployment with correct strategy",
			func(tc testCase) {
				key := types.NamespacedName{
					Name:      tc.name,
					Namespace: "default",
				}

				Deployment := validDeployment(tc.name, tc.strategy)
				Expect(k8sClient.Create(ctx, Deployment)).To(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, Deployment)
				})

				domain := &FakeDeploymentDomain{}
				reconciler := newTestReconciler(domain)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: key,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(domain.Called).To(BeTrue())

				cmd := domain.Cmd
				Expect(cmd.GVK.Kind).To(Equal("Deployment"))
				Expect(cmd.Type).To(Equal(core.CmdUpdate))

				DeploymentObj, ok := cmd.Obj.(*environmentsv1alpha1.Deployment)
				Expect(ok).To(BeTrue())

				Expect(DeploymentObj.Spec.Strategy.Name).To(Equal(tc.strategy))
				//Expect(DeploymentObj.Spec.ServiceAccount.Name).To(Equal(tc.
			},

			Entry("kaniko", testCase{
				name:     "Deployment-kaniko",
				strategy: "kaniko",
			}),
			Entry("Deploymentah", testCase{
				name:     "Deployment-Deploymentah",
				strategy: "Deploymentah-shipwright-managed-push",
			}),
			Entry("Deploymentpacks", testCase{
				name:     "Deployment-Deploymentpacks",
				strategy: "Deploymentpacks-v3",
			}),
		)
	})
})
