/*
Copyright 2026 The BlanketOps Authors.
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

package scenario

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
)

// full exercises the core delivery envelope: an Environment is admitted,
// reconciled (finalizer added, phase set), a composed ServiceUnit becomes
// Ready under it, and deleting the Environment tears the chain down. It uses a
// static ServiceUnit so the scenario is deterministic without external build /
// registry / git infrastructure.
var _ = Describe("full delivery chain", Label("full"), Ordered, func() {
	const (
		envName = "scenario-full"
		suName  = "scenario-full-api"
	)

	AfterAll(func() {
		deleteAndWait(&environmentsv1alpha1.Environment{
			ObjectMeta: objMeta(envName),
		})
		su := &environmentsv1alpha1.ServiceUnit{ObjectMeta: objMeta(suName)}
		_ = k8sClient.Delete(ctx, su)
	})

	It("admits and reconciles the Environment (finalizer + phase)", func() {
		createFresh(newEnvironment(envName, "development", "v1"))

		By("the controller adds its finalizer")
		Eventually(func() []string {
			env := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: envName, Namespace: testNamespace}, env); err != nil {
				return nil
			}
			return env.Finalizers
		}, reconcileTimeout, pollInterval).Should(
			ContainElement("environments.blanketops.dev/environment-finalizer"),
			"Environment should get its cascade finalizer",
		)

		By("the Environment reaches a resolved phase")
		Eventually(func() string { return envPhase(envName) }, reconcileTimeout, pollInterval).
			ShouldNot(BeEmpty(), "Environment should report a contract phase")
	})

	It("brings a static ServiceUnit to Ready", func() {
		createFresh(newStaticServiceUnit(suName, envName, "ghcr.io/blanketops/for-kaniko-app:v1"))

		Eventually(func() bool {
			return conditionTrue(&environmentsv1alpha1.ServiceUnit{}, suName, "ServiceUnitReady")
		}, reconcileTimeout, pollInterval).Should(BeTrue(), "ServiceUnit should become Ready")
	})

	It("tears the chain down when the Environment is deleted", func() {
		deleteAndWait(&environmentsv1alpha1.Environment{ObjectMeta: objMeta(envName)})

		By("the labelled ServiceUnit is cascaded away")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: suName, Namespace: testNamespace}, &environmentsv1alpha1.ServiceUnit{})
			return err != nil
		}, reconcileTimeout, pollInterval).Should(BeTrue(), "ServiceUnit should be cascade-deleted")
	})
})
