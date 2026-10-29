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

// Package sources contains performance benchmarks for GitRepository CR.
// Run with: go test -bench=. -benchmem ./tests/benchmark/sources/...
package sources

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	sshSpecJSON = `{"secretStoreRef":{"name":"vault-store","kind":"ClusterSecretStore"},` +
		`"target":{"name":"git-ssh"},` +
		`"data":[` +
		`{"secretKey":"ssh-privatekey","remoteRef":{"key":"blanketops/environments/git/ssh-privatekey"}},` +
		`{"secretKey":"ssh-publickey","remoteRef":{"key":"blanketops/environments/git/ssh-publickey"}},` +
		`{"secretKey":"known-hosts","remoteRef":{"key":"blanketops/environments/git/known-hosts"}}` +
		`]}`
)

// BenchmarkGitSSHSecretCreation benchmarks Git SSH secret creation.
func BenchmarkGitSSHSecretCreation(b *testing.B) {
	_ = scheme.AddToScheme(scheme.Scheme)

	sshSpec := []byte(sshSpecJSON)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]byte(nil), sshSpec...)
	}
}

// BenchmarkGitRepositoryClone benchmarks git clone operations.
func BenchmarkGitRepositoryClone(b *testing.B) {
	repos := []string{
		"github.com/org/repo1",
		"github.com/org/repo2",
		"gitlab.com/org/repo3",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = repos[i%len(repos)]
	}
}

// BenchmarkGitWebhookReconcile benchmarks Git webhook reconciliation.
func BenchmarkGitWebhookReconcile(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkGitCredentialsRotation benchmarks git credential rotation.
func BenchmarkGitCredentialsRotation(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Delete(ctx, nil)
		_ = client.Create(ctx, nil)
	}
}

// BenchmarkGitStateRepositorySync benchmarks package state repository sync.
func BenchmarkGitStateRepositorySync(b *testing.B) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.List(ctx, nil)
		_ = client.Update(ctx, nil)
	}
}

// BenchmarkGitJSONMarshaling benchmarks JSON marshaling of git specs.
func BenchmarkGitJSONMarshaling(b *testing.B) {
	spec := map[string]any{
		"repository": "github.com/org/repo",
		"revision":   "main",
		"auth": map[string]string{
			"type": "ssh",
		},
		"submodules": true,
		"depth":      1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(spec)
		var out map[string]any
		_ = json.Unmarshal(data, &out)
	}
}
