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

// Package observers holds conformance suites for the observer reconcilers
// under environments-controller/internal/controller/observers — the
// side-channel layer that watches non-BlanketOps infrastructure objects
// (Shipwright BuildRun, Argo Events Sensor) and reflects their real-world
// state back onto the owning BlanketOps CR. Distinct from tests/fake/events
// and tests/fake/environments, which exercise the domain reconcilers.
package observers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	argoeventsv1alpha1 "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	shipwrightv1alpha1 "github.com/shipwright-io/build/pkg/apis/build/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       *rest.Config
	k8sClient client.Client
)

var testLogger = logr.Discard()

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Observer Controller Suite")
}

// This suite runs against a real, already-running cluster — the same
// KUBECONFIG-resolved cluster tests/fake/environments and tests/fake/events
// use. It additionally requires the Shipwright BuildRun and Argo Events
// Sensor CRDs (not their controllers — these tests drive fake objects
// directly, same as elsewhere in this repo).
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = environmentsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = eventsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = shipwrightv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = argoeventsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("connecting to the running cluster")
	cfg, err = config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// GitHubEvent CRs live in the argo-events namespace by convention — this
	// is fixture setup for these tests, not standing up the environment.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "argo-events"}}
	if err := k8sClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// Build/BuildRun fixtures live in a namespace dedicated to this package
	// rather than "default" — tests/grpc's ListBuilds scans "default"
	// unscoped, and go test runs packages concurrently against the same
	// shared cluster by default, so a Build sitting in "default" here (this
	// package's applyRetry MaxAttempts scenario deliberately runs ~3s to
	// get a deterministic BuildRun ordering) can get caught mid-flight by
	// another package's list call.
	observersNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "observers-test"}}
	if err := k8sClient.Create(ctx, observersNS); err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	cancel()
})
