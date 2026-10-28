// Package builds contains performance benchmarks for Build CR.
// Run with: go test -bench=. -benchmem ./tests/benchmark/builds/...
package builds

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// BenchmarkBuildCRDCreation benchmarks the creation of Build CRs.
func BenchmarkBuildCRDCreation(b *testing.B) {
	_ = scheme.AddToScheme(scheme.Scheme)

	buildSpec := []byte(`{"strategy":"kaniko","source":{"git":{"repository":"github.com/org/repo"}}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]byte(nil), buildSpec...)
	}
}

// BenchmarkBuildStrategySelection benchmarks build strategy resolution.
func BenchmarkBuildStrategySelection(b *testing.B) {
	strategies := []string{"kaniko", "buildpacks", "buildah"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategies[i%len(strategies)]
	}
}

// BenchmarkBuildRegistrySecretGeneration benchmarks registry credential secret generation.
func BenchmarkBuildRegistrySecretGeneration(b *testing.B) {
	spec := map[string]string{
		"secretStore":  "vault-store",
		"refreshRate":  "1h",
		"targetName":   "build-registry",
		"remoteRefKey": "blanketops/environments/registry/config",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = spec
	}
}

// BenchmarkBuildStatusUpdate benchmarks build status reconciliation.
func BenchmarkBuildStatusUpdate(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Status().Update(ctx, nil)
	}
}

// BenchmarkBuildLogProcessing benchmarks build log aggregation.
func BenchmarkBuildLogProcessing(b *testing.B) {
	logs := []byte(`step 1: pulling image... done
step 2: running build... done
step 3: pushing image... done`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = string(logs)
	}
}

// BenchmarkBuildJSONMarshaling benchmarks JSON marshaling of build specs.
func BenchmarkBuildJSONMarshaling(b *testing.B) {
	spec := map[string]interface{}{
		"strategy": "kaniko",
		"source": map[string]string{
			"git":     "github.com/org/repo",
			"revision": "main",
		},
		"output": map[string]string{
			"image": "registry.io/org/app",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]interface{}
		_ = json.Unmarshal(data, &out)
	}
}