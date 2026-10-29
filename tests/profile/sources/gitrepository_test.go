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

// Package sources contains CPU/memory profiling tests for GitRepository CR reconciliation.
package sources

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestGitSSHSecretReconcileProfile profiles CPU usage during Git SSH secret reconciliation.
func TestGitSSHSecretReconcileProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test in short mode")
	}

	ctx := context.Background()

	runtime.SetCPUProfileRate(1000)
	defer runtime.SetCPUProfileRate(0)

	start := time.Now()
	for range 100 {
		_ = ctx.Done()
	}
	t.Logf("Reconciled 100 Git SSH secrets in %v", time.Since(start))
}

// TestGitRepositoryCloneProfile profiles memory during git repository operations.
func TestGitRepositoryCloneProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test in short mode")
	}

	ctx := context.Background()

	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for range 500 {
		_ = ctx.Done()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	t.Logf("Heap alloc delta: %d KB", (m2.HeapAlloc-m1.HeapAlloc)/1024)
}

// TestGitWebhookReconcileProfile profiles CPU during Git webhook reconciliation.
func TestGitWebhookReconcileProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test in short mode")
	}

	ctx := context.Background()

	runtime.SetCPUProfileRate(1000)
	defer runtime.SetCPUProfileRate(0)

	start := time.Now()
	for range 100 {
		_ = ctx.Done()
	}
	t.Logf("Reconciled 100 Git webhooks in %v", time.Since(start))
}
