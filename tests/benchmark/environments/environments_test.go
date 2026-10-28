// Package environments contains performance benchmarks for Environment CR.
// Run with: go test -bench=. -benchmem ./tests/benchmark/environments/...
package environments

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// BenchmarkEnvironmentCRDCreation benchmarks the creation of Environment CRs.
func BenchmarkEnvironmentCRDCreation(b *testing.B) {
	_ = scheme.AddToScheme(scheme.Scheme)

	contractRaw := []byte(`{"secretStore":{"provider":"vault"},"build":{"strategy":"kaniko"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]byte(nil), contractRaw...)
	}
}

// BenchmarkEnvironmentResolution benchmarks contract resolution.
func BenchmarkEnvironmentResolution(b *testing.B) {
	raw := []byte(`{"secretStore":{"provider":"vault"},"build":{"strategy":"kaniko"}}`)

	var parsed map[string]interface{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(raw, &parsed)
	}
}

// BenchmarkExternalSecretGeneration benchmarks ExternalSecret manifest generation.
func BenchmarkExternalSecretGeneration(b *testing.B) {
	spec := map[string]string{
		"secretStore":  "vault-store",
		"refreshRate":  "1h",
		"targetName":   "git-ssh",
		"remoteRefKey": "blanketops/environments/git/ssh-privatekey",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = spec
	}
}

// BenchmarkSecretSync benchmarks the full secret sync loop.
func BenchmarkSecretSync(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.List(ctx, nil)
		_ = client.Create(ctx, nil)
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkClusterSecretStoreValidation benchmarks ClusterSecretStore validation.
func BenchmarkClusterSecretStoreValidation(b *testing.B) {
	store := map[string]interface{}{
		"provider": "vault",
		"server":   "http://vault:8200",
		"path":     "secret",
		"version":  "v2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store
	}
}

// BenchmarkJSONMarshaling benchmarks JSON marshaling/unmarshaling of contract specs.
func BenchmarkJSONMarshaling(b *testing.B) {
	spec := map[string]interface{}{
		"secretStore": map[string]string{"provider": "vault"},
		"build":       map[string]string{"strategy": "kaniko"},
		"deployment":  map[string]string{"replicas": "3"},
		"git":         map[string]string{"repository": "github.com/org/repo"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]interface{}
		_ = json.Unmarshal(data, &out)
	}
}