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

package observers

import (
	"context"
	"encoding/json"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	buildrunobserver "github.com/blanketops/environments-controller/pkg/controller/observers/buildrun"
	builddomain "github.com/blanketops/environments/pkg/apis/build/domain"
	buildapplication "github.com/blanketops/environments/pkg/apis/build/application"
	shipwrightv1alpha1 "github.com/shipwright-io/build/pkg/apis/build/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	buildtestdata "github.com/blanketops/environments-tests/tests/data/environments"
)

var _ = Describe("BuildRun Observer", func() {
	ctx := context.Background()
	const testNamespace = "default"
	const (
		appName   = "buildrun-app"
		buildName = "build-buildrun-app"
	)

	var build *environmentsv1alpha1.Build

	BeforeEach(func() {
		build = buildToAPI(buildtestdata.NewBuildData(buildName, testNamespace, appName))
		Expect(k8sClient.Create(ctx, build)).To(Succeed())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, build)
	})

	newReconciler := func() *buildrunobserver.Reconciler {
		return &buildrunobserver.Reconciler{
			Client: k8sClient,
			Status: buildapplication.NewStatusWriter(k8sClient, testLogger),
		}
	}

	reconcileRun := func(name string) {
		reconciler := newReconciler()
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: testNamespace},
		})
		Expect(err).NotTo(HaveOccurred())
	}

	It("does nothing for a non-terminal BuildRun", func() {
		run := buildRunFixture(buildName+"-run-pending", testNamespace, buildName)
		Expect(k8sClient.Create(ctx, run)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, run) }()

		reconcileRun(run.Name)

		updated := &environmentsv1alpha1.Build{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
		Expect(updated.Status.Contract.Raw).To(BeEmpty())
	})

	It("writes BuildSuccess when the BuildRun's Succeeded condition is True", func() {
		run := buildRunFixture(buildName+"-run-ok", testNamespace, buildName)
		Expect(k8sClient.Create(ctx, run)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, run) }()

		run.Status.Conditions = shipwrightv1alpha1.Conditions{{
			Type:   shipwrightv1alpha1.Succeeded,
			LastTransitionTime: metav1.Now(),
			Status: corev1.ConditionTrue,
		}}
		Expect(k8sClient.Status().Update(ctx, run)).To(Succeed())

		reconcileRun(run.Name)

		updated := &environmentsv1alpha1.Build{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())

		found := false
		for _, c := range updated.Status.Conditions {
			if c.Type == "BuildSuccess" {
				found = true
				Expect(string(c.Status)).To(Equal("True"))
			}
		}
		Expect(found).To(BeTrue(), "expected a BuildSuccess condition")

		var contract builddomain.BuildStatus
		Expect(json.Unmarshal(updated.Status.Contract.Raw, &contract)).To(Succeed())
		Expect(contract.Success).To(BeTrue())
		Expect(contract.ExecutionRef).To(Equal(run.Name))
	})

	It("writes BuildFailed when the BuildRun's Succeeded condition is False", func() {
		run := buildRunFixture(buildName+"-run-fail", testNamespace, buildName)
		Expect(k8sClient.Create(ctx, run)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, run) }()

		run.Status.Conditions = shipwrightv1alpha1.Conditions{{
			Type:               shipwrightv1alpha1.Succeeded,
			Status:             corev1.ConditionFalse,
			Message:            "image push failed",
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, run)).To(Succeed())

		reconcileRun(run.Name)

		updated := &environmentsv1alpha1.Build{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())

		found := false
		for _, c := range updated.Status.Conditions {
			if c.Type == "BuildFailed" {
				found = true
			}
		}
		Expect(found).To(BeTrue(), "expected a BuildFailed condition")

		var contract builddomain.BuildStatus
		Expect(json.Unmarshal(updated.Status.Contract.Raw, &contract)).To(Succeed())
		Expect(contract.Success).To(BeFalse())
		Expect(contract.Message).To(Equal("image push failed"))
	})

	It("does nothing when the BuildRun has no owning build label", func() {
		run := &shipwrightv1alpha1.BuildRun{}
		run.Name = "orphan-run"
		run.Namespace = testNamespace
		Expect(k8sClient.Create(ctx, run)).To(Succeed())
		defer func() { _ = k8sClient.Delete(ctx, run) }()

		run.Status.Conditions = shipwrightv1alpha1.Conditions{{
			Type:               shipwrightv1alpha1.Succeeded,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, run)).To(Succeed())

		reconcileRun(run.Name)

		updated := &environmentsv1alpha1.Build{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
		Expect(updated.Status.Contract.Raw).To(BeEmpty())
	})
})
