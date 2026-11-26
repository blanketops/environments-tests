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

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
)

// multi-env stands up two Environments of different types side by side and
// asserts each reconciles independently (both get finalizers + phases), then
// tears both down — proving environments are isolated, not conflated.
var _ = Describe("multiple environments side by side", Label("multi-env"), Ordered, func() {
	const (
		devEnv = "scenario-multi-dev"
		qaEnv  = "scenario-multi-qa"
	)
	const finalizer = "environments.blanketops.dev/environment-finalizer"

	AfterAll(func() {
		deleteAndWait(&environmentsv1alpha1.Environment{ObjectMeta: objMeta(devEnv)})
		deleteAndWait(&environmentsv1alpha1.Environment{ObjectMeta: objMeta(qaEnv)})
	})

	It("reconciles a development and a qa environment independently", func() {
		createFresh(newEnvironment(devEnv, "development", "v1"))
		createFresh(newEnvironment(qaEnv, "qa", "v1"))

		for _, name := range []string{devEnv, qaEnv} {
			By("both get their finalizer and a phase: " + name)
			Eventually(func() bool {
				env := &environmentsv1alpha1.Environment{}
				if err := k8sClient.Get(ctx, objKey(name), env); err != nil {
					return false
				}
				return containsString(env.Finalizers, finalizer) && envPhase(name) != ""
			}, reconcileTimeout, pollInterval).Should(BeTrue(), "%s should reconcile independently", name)
		}
	})
})
