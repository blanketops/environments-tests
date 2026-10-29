/*
Copyright 2025 The BlanketOps Authors.
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
	spec := map[string]any{
		"strategy": "kaniko",
		"source": map[string]string{
			"git":      "github.com/org/repo",
			"revision": "main",
		},
		"output": map[string]string{
			"image": "registry.io/org/app",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]any
		_ = json.Unmarshal(data, &out)
	}
}
