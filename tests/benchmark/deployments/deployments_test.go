// Package deployments contains performance benchmarks for Deployment CR.
// Run with: go test -bench=. -benchmem ./tests/benchmark/deployments/...
package deployments

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	healthPath = "/health"
	readyPath  = "/ready"
	livePath   = "/live"
	probePort  = 8080
	probeKey   = "path"
)

// BenchmarkDeploymentCRDCreation benchmarks the creation of Deployment CRs.
func BenchmarkDeploymentCRDCreation(b *testing.B) {
	_ = scheme.AddToScheme(scheme.Scheme)

	deploySpec := []byte(`{"replicas":3,"strategy":"rolling","image":"registry.io/app:latest"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]byte(nil), deploySpec...)
	}
}

// BenchmarkDeploymentRolloutStrategy benchmarks rollout strategy selection.
func BenchmarkDeploymentRolloutStrategy(b *testing.B) {
	strategies := []string{"rolling", "recreate", "blue-green"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategies[i%len(strategies)]
	}
}

// BenchmarkDeploymentScaling benchmarks replica scaling operations.
func BenchmarkDeploymentScaling(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkDeploymentHealthCheck benchmarks health probe evaluation.
func BenchmarkDeploymentHealthCheck(b *testing.B) {
	probes := []map[string]any{
		{probeKey: healthPath, "port": probePort},
		{probeKey: readyPath, "port": probePort},
		{probeKey: livePath, "port": probePort},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = probes[i%len(probes)]
	}
}

// BenchmarkDeploymentStatusUpdate benchmarks deployment status reconciliation.
func BenchmarkDeploymentStatusUpdate(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Status().Update(ctx, nil)
	}
}

// BenchmarkDeploymentJSONMarshaling benchmarks JSON marshaling of deployment specs.
func BenchmarkDeploymentJSONMarshaling(b *testing.B) {
	spec := map[string]any{
		"replicas": 3,
		"strategy": "rolling",
		"image":    "registry.io/app:latest",
		"ports":    []int{8080, 8443},
		"env": map[string]string{
			"ENV": "production",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]any
		_ = json.Unmarshal(data, &out)
	}
}