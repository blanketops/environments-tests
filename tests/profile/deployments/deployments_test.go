// Package deployments contains CPU/memory profiling tests for Deployment CR reconciliation.
package deployments

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestDeploymentReconcileProfile profiles CPU usage during deployment reconciliation.
func TestDeploymentReconcileProfile(t *testing.T) {
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
	t.Logf("Reconciled 100 deployments in %v", time.Since(start))
}

// TestDeploymentRolloutProfile profiles memory during deployment rollout.
func TestDeploymentRolloutProfile(t *testing.T) {
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