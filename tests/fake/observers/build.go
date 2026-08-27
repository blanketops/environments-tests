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
	"fmt"
	"time"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	buildobserver "github.com/blanketops/environments-controller/pkg/controller/observers/build"
	buildrunobserver "github.com/blanketops/environments-controller/pkg/controller/observers/buildrun"
	buildapplication "github.com/blanketops/environments/pkg/apis/build/application"
	builddomain "github.com/blanketops/environments/pkg/apis/build/domain"
	shipwrightv1alpha1 "github.com/shipwright-io/build/pkg/apis/build/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	buildtestdata "github.com/blanketops/environments-tests/tests/data/environments"
	eventtestdata "github.com/blanketops/environments-tests/tests/data/events"
)

// buildToAPI converts a testdata.BuildData into the real Build API type,
// same conversion point pattern as tests/fake/environments/build.go.
func buildToAPI(d *buildtestdata.BuildData) *environmentsv1alpha1.Build {
	raw, err := json.Marshal(d.Spec)
	if err != nil {
		panic(fmt.Sprintf("marshal build contract: %v", err))
	}
	return &environmentsv1alpha1.Build{
		ObjectMeta: d.ObjectMeta,
		Spec: environmentsv1alpha1.BuildSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// githubEventToAPI converts a testdata.GitHubEventData into the real
// GitHubEvent API type. Same conversion point pattern as
// tests/fake/events/githubevent.go's gitHubEventFromData.
func githubEventToAPI(d *eventtestdata.GitHubEventData) *eventsv1alpha1.GitHubEvent {
	raw, err := json.Marshal(d.Spec.Contract)
	if err != nil {
		panic(fmt.Sprintf("marshal githubevent contract: %v", err))
	}
	return &eventsv1alpha1.GitHubEvent{
		ObjectMeta: d.ObjectMeta,
		Spec: eventsv1alpha1.GitHubEventSpec{
			Contract: runtime.RawExtension{Raw: raw},
		},
	}
}

// setBuildStatus marshals a builddomain.BuildStatus and writes it to the
// Build's status subresource — the same shape buildrun-observer itself
// produces, so applyRetry sees exactly what it would in production.
func setBuildStatus(ctx context.Context, build *environmentsv1alpha1.Build, status builddomain.BuildStatus) {
	raw, err := json.Marshal(status)
	Expect(err).NotTo(HaveOccurred())
	build.Status.Contract = runtime.RawExtension{Raw: raw}
	Expect(k8sClient.Status().Update(ctx, build)).To(Succeed())
}

// buildRunFixture returns a Shipwright BuildRun labeled to correlate with
// the given Build, matching what build-observer's applyRetry looks up by
// build.blanketops.dev/name.
func buildRunFixture(name, namespace, buildName string) *shipwrightv1alpha1.BuildRun {
	return &shipwrightv1alpha1.BuildRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"build.blanketops.dev/name": buildName},
		},
	}
}

var _ = Describe("Build Observer", func() {
	ctx := context.Background()
	const testNamespace = "default"

	// applyTriggers: mirrors GitHubEvent commit metadata onto Build
	// annotations, cross-namespace, correlated by environments.blanketops.dev/name.

	Context("applyTriggers", func() {
		const (
			appName   = "trig-app"
			buildName = "build-trig-app"
			eventName = "event-trig-app"
		)
		var build *environmentsv1alpha1.Build

		BeforeEach(func() {
			build = buildToAPI(buildtestdata.NewBuildData(buildName, testNamespace, appName))
			Expect(k8sClient.Create(ctx, build)).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, build)
			_ = k8sClient.Delete(ctx, &eventsv1alpha1.GitHubEvent{
				ObjectMeta: metav1.ObjectMeta{Name: eventName, Namespace: "argo-events"},
			})
		})

		It("mirrors a matching GitHubEvent's commit metadata onto the Build's annotations", func() {
			d := eventtestdata.NewGitHubEventData(eventName, appName)
			d.Spec.Contract.CommitSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
			event := githubEventToAPI(d)
			Expect(k8sClient.Create(ctx, event)).To(Succeed())

			reconciler := &buildobserver.Reconciler{Client: k8sClient}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: buildName, Namespace: testNamespace},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())

			Expect(updated.Annotations["build.blanketops.dev/trigger-sha"]).To(Equal("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
			Expect(updated.Annotations["build.blanketops.dev/trigger-ref"]).To(Equal("refs/heads/main"))
			Expect(updated.Annotations["build.blanketops.dev/trigger-type"]).To(Equal("push"))
			Expect(updated.Annotations["build.blanketops.dev/trigger-source"]).To(Equal("github"))
		})

		It("does not patch when no GitHubEvent event type matches the Build's allowed triggers", func() {
			// NewBuildData's default policy only allows push and pull_request.
			d := eventtestdata.NewGitHubEventData(eventName, appName)
			d.Spec.Contract.EventType = "release"
			d.Spec.Contract.CommitSHA = "cccccccccccccccccccccccccccccccccccccccc"
			event := githubEventToAPI(d)
			Expect(k8sClient.Create(ctx, event)).To(Succeed())

			reconciler := &buildobserver.Reconciler{Client: k8sClient}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: buildName, Namespace: testNamespace},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
			Expect(updated.Annotations["build.blanketops.dev/trigger-sha"]).To(BeEmpty())
		})

		It("does not redundantly re-patch when the commit SHA hasn't changed", func() {
			d := eventtestdata.NewGitHubEventData(eventName, appName)
			d.Spec.Contract.CommitSHA = "1111111111111111111111111111111111111111"
			event := githubEventToAPI(d)
			Expect(k8sClient.Create(ctx, event)).To(Succeed())

			reconciler := &buildobserver.Reconciler{Client: k8sClient}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: buildName, Namespace: testNamespace}}

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			afterFirst := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, afterFirst)).To(Succeed())
			Expect(afterFirst.Annotations["build.blanketops.dev/trigger-sha"]).To(Equal("1111111111111111111111111111111111111111"))
			rvAfterFirst := afterFirst.ResourceVersion

			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			afterSecond := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, afterSecond)).To(Succeed())
			Expect(afterSecond.ResourceVersion).To(Equal(rvAfterFirst), "reconciling with an unchanged SHA should not produce a redundant patch")
		})
	})

	// applyRetry: counts BuildRuns against Policy.Retry.MaxAttempts and bumps
	// retry-attempt to request another dispatch on a failed, terminal attempt.

	Context("applyRetry", func() {
		const (
			appName   = "retry-app"
			buildName = "build-retry-app"
		)
		var build *environmentsv1alpha1.Build

		BeforeEach(func() {
			// NewBuildData defaults to Retry{OnFailure:true, MaxAttempts:3}.
			build = buildToAPI(buildtestdata.NewBuildData(buildName, testNamespace, appName))
			Expect(k8sClient.Create(ctx, build)).To(Succeed())
		})

		AfterEach(func() {
			var runs shipwrightv1alpha1.BuildRunList
			_ = k8sClient.List(ctx, &runs, client.InNamespace(testNamespace), client.MatchingLabels{"build.blanketops.dev/name": buildName})
			for i := range runs.Items {
				_ = k8sClient.Delete(ctx, &runs.Items[i])
			}
			_ = k8sClient.Delete(ctx, build)
		})

		reconcileBuild := func(name string) {
			reconciler := &buildobserver.Reconciler{Client: k8sClient}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: testNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
		}

		It("does nothing when the Build's retry policy has onFailure disabled", func() {
			d := buildtestdata.NewBuildData(buildName+"-noretry", testNamespace, appName)
			d.Spec.Policy.Retry.OnFailure = false
			noRetryBuild := buildToAPI(d)
			Expect(k8sClient.Create(ctx, noRetryBuild)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, noRetryBuild) }()

			setBuildStatus(ctx, noRetryBuild, builddomain.BuildStatus{Triggered: true, Success: false, ExecutionRef: "nonexistent"})

			reconcileBuild(noRetryBuild.Name)

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: noRetryBuild.Name, Namespace: testNamespace}, updated)).To(Succeed())
			Expect(updated.Annotations["build.blanketops.dev/retry-attempt"]).To(BeEmpty())
		})

		It("bumps retry-attempt when the latest BuildRun failed and attempts remain", func() {
			run := buildRunFixture(buildName+"-run-1", testNamespace, buildName)
			Expect(k8sClient.Create(ctx, run)).To(Succeed())

			setBuildStatus(ctx, build, builddomain.BuildStatus{
				Triggered:    true,
				Success:      false,
				ExecutionRef: run.Name,
			})

			reconcileBuild(buildName)

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
			Expect(updated.Annotations["build.blanketops.dev/retry-attempt"]).To(Equal("2"))
		})

		It("bumps retry-attempt from a real buildrun-observer outcome — the full observer chain, not hand-set status", func() {
			run := buildRunFixture(buildName+"-run-chain", testNamespace, buildName)
			Expect(k8sClient.Create(ctx, run)).To(Succeed())
			run.Status.Conditions = shipwrightv1alpha1.Conditions{{
				Type:               shipwrightv1alpha1.Succeeded,
				Status:             corev1.ConditionFalse,
				Message:            "kaniko exited 1",
				LastTransitionTime: metav1.Now(),
			}}
			Expect(k8sClient.Status().Update(ctx, run)).To(Succeed())

			brReconciler := &buildrunobserver.Reconciler{
				Client: k8sClient,
				Status: buildapplication.NewStatusWriter(k8sClient, testLogger),
			}
			_, err := brReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: run.Name, Namespace: testNamespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// No manual setBuildStatus here — build-observer must see the
			// outcome buildrun-observer itself just persisted.
			reconcileBuild(buildName)

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
			Expect(updated.Annotations["build.blanketops.dev/retry-attempt"]).To(Equal("2"))
		})

		It("stops bumping once attempts reach the retry policy's MaxAttempts", func() {
			// MaxAttempts is 3 (NewBuildData default) — create 3 BuildRuns so
			// attempts already meets the ceiling. CreationTimestamp is
			// second-granularity, so space creation out to get a
			// deterministic "latest" for the race guard.
			var latest *shipwrightv1alpha1.BuildRun
			for i := 0; i < 3; i++ {
				run := buildRunFixture(fmt.Sprintf("%s-run-%d", buildName, i), testNamespace, buildName)
				Expect(k8sClient.Create(ctx, run)).To(Succeed())
				if latest == nil || run.CreationTimestamp.After(latest.CreationTimestamp.Time) {
					latest = run
				}
				time.Sleep(1100 * time.Millisecond)
			}

			setBuildStatus(ctx, build, builddomain.BuildStatus{
				Triggered:    true,
				Success:      false,
				ExecutionRef: latest.Name,
			})

			reconcileBuild(buildName)

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
			Expect(updated.Annotations["build.blanketops.dev/retry-attempt"]).To(BeEmpty(), "3 attempts already exist against MaxAttempts=3 — should not bump further")
		})

		It("clears retry-attempt once the build succeeds", func() {
			original := build.DeepCopy()
			build.Annotations = map[string]string{"build.blanketops.dev/retry-attempt": "2"}
			Expect(k8sClient.Patch(ctx, build, client.MergeFrom(original))).To(Succeed())

			setBuildStatus(ctx, build, builddomain.BuildStatus{Triggered: true, Success: true})

			reconcileBuild(buildName)

			updated := &environmentsv1alpha1.Build{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: buildName, Namespace: testNamespace}, updated)).To(Succeed())
			_, stillPresent := updated.Annotations["build.blanketops.dev/retry-attempt"]
			Expect(stillPresent).To(BeFalse())
		})
	})
})
