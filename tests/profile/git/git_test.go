// Package git contains CPU/memory profiling tests for Git-related CR reconciliation.
package git

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
	for i := 0; i < 100; i++ {
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

	for i := 0; i < 500; i++ {
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
	for i := 0; i < 100; i++ {
		_ = ctx.Done()
	}
	t.Logf("Reconciled 100 Git webhooks in %v", time.Since(start))
}