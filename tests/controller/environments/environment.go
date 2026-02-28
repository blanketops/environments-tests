/*
Copyright 2025.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	environmentv1 "github.com/ntlaletsi70/blanketops-environments-api/api/environments/v1alpha1"
)

var _ = Describe("Environment Controller", func() {
	Context("When reconciling an Environment resource", func() {
		const resourceName = "test-environment"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		env := &environmentv1.Environment{}

		BeforeEach(func() {
			By("creating the Environment custom resource")

			err := k8sClient.Get(ctx, typeNamespacedName, env)
			if err != nil && errors.IsNotFound(err) {
				resource := &environmentv1.Environment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// Spec: environmentv1.EnvironmentSpec{
					// 	Build: environmentv1.BuildSpec{
					// 		Source: environmentv1.GitSource{
					// 			URL:      "https://github.com/blanketops/sample.git",
					// 			Revision: "main",
					// 		},
					// 		Strategy: environmentv1.Strategy{
					// 			Name: "buildpacks-v3",
					// 			Kind: "ClusterBuildStrategy",
					// 		},
					// 	},
					// },
				}
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				if err != nil && errors.IsNotFound(err) {
					resource := &environmentv1.Environment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      resourceName,
							Namespace: "default",
						},
						Spec: environmentv1.EnvironmentSpec{
							// fill spec fields as needed
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				}

				// Also create the ServiceAccount that the reconciler expects
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "your-build-serviceaccount-name", // replace with expected name
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, sa)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &environmentv1.Environment{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up the Environment resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile the Environment resource without error", func() {
			By("invoking the reconciler")
			reconciler := &EnvironmentReconciler{
				//Client: k8sClient,
				//Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Add status/assertion checks if needed:
			// Example:
			// updated := &environmentv1.Environment{}
			// Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			// Expect(updated.Status.Conditions).NotTo(BeEmpty())
		})
	})
})
