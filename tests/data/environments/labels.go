/*
Copyright 2026 BlanketOps.

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

// fixtureLabels returns the standard BlanketOps labels every environments
// fixture carries, scoped to envName in a dev environment. It returns a new
// map on each call, so a builder can change a label (e.g. the type) safely.
func fixtureLabels(envName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":                         "blanketops-environments",
		"app.kubernetes.io/part-of":                      "blanketops",
		"app.kubernetes.io/managed-by":                   "kustomize",
		"app.kubernetes.io/version":                      "v0.0.1",
		"environments.blanketops.dev/name":               envName,
		"environments.blanketops.dev/type":               "dev",
		"environments.blanketops.dev/api-version":        "v0.1.4",
		"environments.blanketops.dev/contract-version":   "v0.1.7",
		"environments.blanketops.dev/controller-version": "v0.2.3",
		"environments.blanketops.dev/operator-version":   "v0.2.1",
	}
}
