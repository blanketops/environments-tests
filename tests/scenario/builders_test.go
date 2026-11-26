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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
)

// labelKey correlates composed CRs to their Environment (the controller keys
// cascade + aggregation off this label).
const labelKey = "environments.blanketops.dev/name"

func scenarioLabels(envName string) map[string]string {
	return map[string]string{
		labelKey:                              envName,
		"environments.blanketops.dev/type":    "dev",
		"app.kubernetes.io/managed-by":        "scenario-suite",
	}
}

// environmentContract is the resolver-valid raw JSON shape for an Environment.
type environmentContract struct {
	ApplicationName string          `json:"applicationName"`
	Branch          string          `json:"branch"`
	GitOwner        string          `json:"gitOwner"`
	EnvironmentType string          `json:"environmentType"`
	Version         string          `json:"version"`
	SecretStore     *secretStoreRef `json:"secretStore,omitempty"`
}

type secretStoreRef struct {
	Provider  string            `json:"provider,omitempty"`
	StoreName string            `json:"storeName,omitempty"`
	AWS       map[string]string `json:"aws,omitempty"`
}

// newEnvironment builds an Environment CR with a resolver-valid contract. A
// store is included so the engine can resolve its secret binding the same way
// production does.
func newEnvironment(name, envType, version string) *environmentsv1alpha1.Environment {
	raw, err := json.Marshal(environmentContract{
		ApplicationName: name,
		Branch:          "main",
		GitOwner:        "blanketops",
		EnvironmentType: envType,
		Version:         version,
		SecretStore: &secretStoreRef{
			Provider:  "aws",
			StoreName: "blanketops-environments-aws",
			AWS:       map[string]string{"region": "us-east-1"},
		},
	})
	Expect(err).NotTo(HaveOccurred(), "marshal environment contract")
	return &environmentsv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: scenarioLabels(name)},
		Spec:       environmentsv1alpha1.EnvironmentSpec{Contract: runtime.RawExtension{Raw: raw}},
	}
}

// serviceUnitContract is the resolver-valid shape for a static (image-only,
// no Build dependency) ServiceUnit — the lightest composed CR to drive a
// deterministic scenario without external build/registry infrastructure.
type serviceUnitContract struct {
	Type          string `json:"type"`
	Image         string `json:"image"`
	ContainerPort int    `json:"containerPort"`
	Size          int    `json:"size"`
	AppType       string `json:"appType"`
	StackType     string `json:"stackType"`
}

func newStaticServiceUnit(name, envName, image string) *environmentsv1alpha1.ServiceUnit {
	raw, err := json.Marshal(serviceUnitContract{
		Type: "static", Image: image, ContainerPort: 8080, Size: 1, AppType: "web", StackType: "nodejs",
	})
	Expect(err).NotTo(HaveOccurred(), "marshal serviceunit contract")
	return &environmentsv1alpha1.ServiceUnit{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace, Labels: scenarioLabels(envName)},
		Spec:       environmentsv1alpha1.ServiceUnitSpec{Contract: runtime.RawExtension{Raw: raw}},
	}
}

// --- lifecycle helpers -------------------------------------------------------

// createFresh deletes any stale copy of obj then creates it, so a rerun after
// a crashed scenario starts clean.
func createFresh(obj client.Object) {
	GinkgoHelper()
	_ = k8sClient.Delete(ctx, obj)
	Eventually(func() error {
		return client.IgnoreAlreadyExists(k8sClient.Create(ctx, obj))
	}, 30*pollInterval, pollInterval).Should(Succeed(), "create %s/%s", obj.GetNamespace(), obj.GetName())
}

// deleteAndWait deletes obj and waits for it to disappear (finalizers cleared).
func deleteAndWait(obj client.Object) {
	GinkgoHelper()
	key := client.ObjectKeyFromObject(obj)
	_ = k8sClient.Delete(ctx, obj)
	Eventually(func() bool {
		err := k8sClient.Get(ctx, key, obj)
		return err != nil
	}, reconcileTimeout, pollInterval).Should(BeTrue(), "%s should be deleted", key)
}

// envPhase returns the Environment's resolved contract phase (e.g.
// ENVIRONMENT_PHASE_PENDING / _READY), or "" if unset.
func envPhase(name string) string {
	env := &environmentsv1alpha1.Environment{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: testNamespace}, env); err != nil {
		return ""
	}
	var status struct {
		Contract struct {
			Phase string `json:"phase"`
		} `json:"contract"`
	}
	// Status is stored as structured fields; marshal/unmarshal keeps this
	// decoupled from the exact generated Go type across api versions.
	b, err := json.Marshal(env.Status)
	if err != nil {
		return ""
	}
	_ = json.Unmarshal(b, &status)
	return status.Contract.Phase
}

// objMeta is a name+namespace ObjectMeta for Get/Delete by key.
func objMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: testNamespace}
}

// objKey is the client.ObjectKey for a scenario CR by name.
func objKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: testNamespace}
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// conditionTrue fetches the named CR into obj and reports whether it has a
// status condition of condType with status "True". Decoded via round-trip
// JSON so it tolerates api-version drift in the generated status type.
func conditionTrue(obj client.Object, name, condType string) bool {
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: testNamespace}, obj); err != nil {
		return false
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return false
	}
	var wrapper struct {
		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return false
	}
	for _, c := range wrapper.Status.Conditions {
		if c.Type == condType && c.Status == "True" {
			return true
		}
	}
	return false
}
