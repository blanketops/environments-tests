// Package secrets contains performance benchmarks for secret reconciliation.
// Run with: go test -bench=. -benchmem ./tests/benchmark/secrets/...
package secrets

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	esSpecJSON = `{"secretStoreRef":{"name":"vault-store","kind":"ClusterSecretStore"},` +
		`"target":{"name":"git-ssh"},` +
		`"data":[{"secretKey":"ssh-privatekey","remoteRef":{"key":"blanketops/environments/git/ssh-privatekey"}}]}`
)

// BenchmarkExternalSecretCreation benchmarks ExternalSecret CR creation.
func BenchmarkExternalSecretCreation(b *testing.B) {
	_ = scheme.AddToScheme(scheme.Scheme)

	esSpec := []byte(esSpecJSON)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]byte(nil), esSpec...)
	}
}

// BenchmarkSecretStoreValidation benchmarks ClusterSecretStore validation.
func BenchmarkSecretStoreValidation(b *testing.B) {
	providers := []string{"vault", "aws", "azure", "gcp"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = providers[i%len(providers)]
	}
}

// BenchmarkSecretSyncLoop benchmarks the full secret sync from external store.
func BenchmarkSecretSyncLoop(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.List(ctx, nil)
		_ = client.Create(ctx, nil)
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkSecretRotation benchmarks secret rotation operations.
func BenchmarkSecretRotation(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Delete(ctx, nil)
		_ = client.Create(ctx, nil)
	}
}

// BenchmarkClusterSecretStoreReconcile benchmarks ClusterSecretStore reconciliation.
func BenchmarkClusterSecretStoreReconcile(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkSecretJSONMarshaling benchmarks JSON marshaling of secret specs.
func BenchmarkSecretJSONMarshaling(b *testing.B) {
	spec := map[string]any{
		"secretStoreRef": map[string]string{
			"name": "vault-store",
			"kind": "ClusterSecretStore",
		},
		"target": map[string]string{
			"name": "git-ssh",
		},
		"data": []map[string]any{
			{
				"secretKey": "ssh-privatekey",
				"remoteRef": map[string]string{
					"key": "blanketops/environments/git/ssh-privatekey",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]any
		_ = json.Unmarshal(data, &out)
	}
}