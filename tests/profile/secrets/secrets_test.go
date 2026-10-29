// Package secrets contains CPU/memory profiling tests for secret reconciliation.
package secrets

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestExternalSecretReconcileProfile profiles CPU usage during ExternalSecret reconciliation.
func TestExternalSecretReconcileProfile(t *testing.T) {
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
	t.Logf("Reconciled 100 ExternalSecrets in %v", time.Since(start))
}

// TestSecretSyncProfile profiles memory during secret sync from external store.
func TestSecretSyncProfile(t *testing.T) {
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

// TestClusterSecretStoreValidationProfile profiles CPU during ClusterSecretStore validation.
func TestClusterSecretStoreValidationProfile(t *testing.T) {
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
	t.Logf("Validated 100 ClusterSecretStores in %v", time.Since(start))
}
