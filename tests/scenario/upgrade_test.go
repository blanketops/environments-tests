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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
)

// upgrade bumps an Environment's contract version in place and asserts the
// controller re-reconciles the new revision without wedging — the version
// change lands and a phase is still reported.
var _ = Describe("version upgrade", Label("upgrade"), Ordered, func() {
	const envName = "scenario-upgrade"

	AfterAll(func() {
		deleteAndWait(&environmentsv1alpha1.Environment{ObjectMeta: objMeta(envName)})
	})

	It("reconciles v1 then the in-place bump to v2", func() {
		createFresh(newEnvironment(envName, "development", "v1"))

		By("v1 reaches a resolved phase")
		Eventually(func() string { return envPhase(envName) }, reconcileTimeout, pollInterval).
			ShouldNot(BeEmpty())

		By("bumping the contract to v2")
		Eventually(func() error {
			env := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: envName, Namespace: testNamespace}, env); err != nil {
				return err
			}
			env.Spec.Contract = newEnvironment(envName, "development", "v2").Spec.Contract
			return k8sClient.Update(ctx, env)
		}, reconcileTimeout, pollInterval).Should(Succeed())

		By("the stored contract reflects v2 and the env stays reconciled")
		Eventually(func() string {
			env := &environmentsv1alpha1.Environment{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: envName, Namespace: testNamespace}, env); err != nil {
				return ""
			}
			var c environmentContract
			if err := json.Unmarshal(env.Spec.Contract.Raw, &c); err != nil {
				return ""
			}
			return c.Version
		}, reconcileTimeout, pollInterval).Should(Equal("v2"))

		Consistently(func() string { return envPhase(envName) }, 10*pollInterval, pollInterval).
			ShouldNot(BeEmpty(), "Environment should remain reconciled after the upgrade")
	})
})
