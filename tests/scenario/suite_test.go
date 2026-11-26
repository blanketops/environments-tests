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

// Package scenario is the dedicated end-to-end scenario suite. Unlike the
// per-controller fake suites (tests/fake/*), these specs drive multi-step
// product journeys — a full delivery chain, a version upgrade, multiple
// environments side by side — against a REAL running cluster (CRDs installed
// via environments-install, controller-manager running). Each scenario carries
// a Ginkgo Label so CI can run one at a time:
//
//	go test ./tests/scenario/... -args -ginkgo.label-filter=upgrade
//
// Labels: full, upgrade, rollback, multi-env, git-auth, secret-rotation.
package scenario

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	environmentsv1alpha1 "github.com/blanketops/environments-api/api/environments/v1alpha1"
	eventsv1alpha1 "github.com/blanketops/environments-api/api/events/v1alpha1"
	sourcesv1alpha1 "github.com/blanketops/environments-api/api/sources/v1alpha1"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       *rest.Config
	k8sClient client.Client

	// testNamespace is where scenario CRs are created. Overridable so the
	// suite can target an install namespace (blanketops-environments) or a
	// scratch one. The namespace must already exist.
	testNamespace = envOr("SCENARIO_NAMESPACE", "blanketops-environments")
)

// Timeouts for Eventually/Consistently against a live cluster. Scenario steps
// wait on real reconciliation, so these are generous.
const (
	reconcileTimeout = 3 * time.Minute
	pollInterval     = 2 * time.Second
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestScenarios(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Environments Scenario Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("registering the BlanketOps CR schemes")
	Expect(environmentsv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(sourcesv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(eventsv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	By("connecting to the running cluster")
	var err error
	cfg, err = config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
})
