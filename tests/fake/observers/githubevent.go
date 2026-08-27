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

	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	githubeventobserver "github.com/blanketops/environments-controller/pkg/controller/observers/githubevent"
	githubeventapplication "github.com/blanketops/environments/pkg/apis/githubevent/application"
	argoeventsv1alpha1 "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventtestdata "github.com/blanketops/environments-tests/tests/data/events"
)

var _ = Describe("GitHubEvent Observer", func() {
	ctx := context.Background()
	const (
		appName   = "signup-app"
		eventName = "event-signup-app"
	)

	newReconciler := func() *githubeventobserver.Reconciler {
		return &githubeventobserver.Reconciler{
			Client: k8sClient,
			Status: githubeventapplication.NewStatusWriter(k8sClient, testLogger),
		}
	}

	reconcileEvent := func() {
		reconciler := newReconciler()
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: eventName, Namespace: "argo-events"},
		})
		Expect(err).NotTo(HaveOccurred())
	}

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &eventsv1alpha1.GitHubEvent{
			ObjectMeta: metav1.ObjectMeta{Name: eventName, Namespace: "argo-events"},
		})
		_ = k8sClient.Delete(ctx, &argoeventsv1alpha1.Sensor{
			ObjectMeta: metav1.ObjectMeta{Name: "github-sensor-" + eventName, Namespace: "argo-events"},
		})
	})

	It("does not write status while no webhook payload has arrived (EventID empty)", func() {
		d := eventtestdata.NewGitHubEventData(eventName, appName)
		d.Spec.Contract.EventID = ""
		event := githubEventToAPI(d)
		Expect(k8sClient.Create(ctx, event)).To(Succeed())

		reconcileEvent()

		updated := &eventsv1alpha1.GitHubEvent{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: eventName, Namespace: "argo-events"}, updated)).To(Succeed())
		Expect(updated.Status.Contract.Raw).To(BeEmpty())
		Expect(updated.Status.Conditions).To(BeEmpty())
	})

	It("marks GitHubEventReceiving once a payload has arrived but the Sensor hasn't reported success", func() {
		event := githubEventToAPI(eventtestdata.NewGitHubEventData(eventName, appName))
		Expect(k8sClient.Create(ctx, event)).To(Succeed())
		// Deliberately no Sensor object created — the observer must tolerate
		// a not-found Sensor gracefully rather than erroring.

		reconcileEvent()

		updated := &eventsv1alpha1.GitHubEvent{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: eventName, Namespace: "argo-events"}, updated)).To(Succeed())

		found := false
		for _, c := range updated.Status.Conditions {
			if c.Type == "GitHubEventReceiving" {
				found = true
			}
		}
		Expect(found).To(BeTrue(), "expected GitHubEventReceiving while the Sensor hasn't confirmed success")
	})

	It("marks GitHubEventReady once a payload has arrived and the Sensor reports success — the real sign-up confirmation", func() {
		event := githubEventToAPI(eventtestdata.NewGitHubEventData(eventName, appName))
		Expect(k8sClient.Create(ctx, event)).To(Succeed())

		sensor := &argoeventsv1alpha1.Sensor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "github-sensor-" + eventName,
				Namespace: "argo-events",
			},
		}
		Expect(k8sClient.Create(ctx, sensor)).To(Succeed())
		sensor.Status.Conditions = []argoeventsv1alpha1.Condition{{
			Type:   argoeventsv1alpha1.ConditionType("Succeeded"),
			Status: corev1.ConditionTrue,
		}}
		Expect(k8sClient.Status().Update(ctx, sensor)).To(Succeed())

		reconcileEvent()

		updated := &eventsv1alpha1.GitHubEvent{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: eventName, Namespace: "argo-events"}, updated)).To(Succeed())

		found := false
		for _, c := range updated.Status.Conditions {
			if c.Type == "GitHubEventReady" && string(c.Status) == "True" {
				found = true
			}
		}
		Expect(found).To(BeTrue(), "expected GitHubEventReady once the Sensor confirmed success")
	})
})
