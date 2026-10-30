/*
Copyright 2026 The BlanketOps Authors.
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

// Package engine contains performance benchmarks for the core Engine.
// Run with: go test -bench=. -benchmem ./tests/benchmark/engine/...
package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	command "github.com/blanketops/environments/core/command"
	registry "github.com/blanketops/environments/core/registry"
	enginepkg "github.com/blanketops/environments/core/engine"
)

var benchGVK = schema.GroupVersionKind{Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "Build"}

// benchDomain for benchmarks - minimal overhead
type benchDomain struct {
	gvk       schema.GroupVersionKind
	mu        sync.Mutex
	handled   int
	handleErr error
	delay     time.Duration
}

func (f *benchDomain) GVK() schema.GroupVersionKind { return f.gvk }

func (f *benchDomain) Handle(ctx context.Context, cmd command.Command) error {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.handled++
	f.mu.Unlock()
	return f.handleErr
}

func (f *benchDomain) CanCreate(client.Object) bool                { return true }
func (f *benchDomain) CanUpdate(client.Object, client.Object) bool { return true }
func (f *benchDomain) CanDelete(client.Object) bool                { return true }

func (f *benchDomain) handledCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.handled
}

// BenchmarkEngine_Execute_Sync - baseline synchronous execution
func BenchmarkEngine_Execute_Sync(b *testing.B) {
	reg := registry.NewRegistry()
	d := &benchDomain{gvk: benchGVK}
	reg.RegisterDomain(benchGVK, d)
	e := enginepkg.NewEngine(reg, logr.Discard())

	cmd := command.Command{Type: command.CmdCreate, GVK: benchGVK}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = e.Execute(context.Background(), cmd)
	}
}

// BenchmarkEngine_Queue_Async - async queue with workers
func BenchmarkEngine_Queue_Async(b *testing.B) {
	reg := registry.NewRegistry()
	d := &benchDomain{gvk: benchGVK}
	reg.RegisterDomain(benchGVK, d)
	e := enginepkg.NewEngine(reg, logr.Discard())
	e.SetWorkers(4)
	e.StartWorkers()
	defer e.StopWorkers(time.Second)

	cmd := command.Command{Type: command.CmdCreate, GVK: benchGVK}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = e.Queue(context.Background(), cmd)
	}

	// Drain queue to ensure all processed
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkEngine_Execute_CircuitBreaker_Open - circuit breaker active
func BenchmarkEngine_Execute_CircuitBreaker_Open(b *testing.B) {
	reg := registry.NewRegistry()
	d := &benchDomain{gvk: benchGVK}
	reg.RegisterDomain(benchGVK, d)
	e := enginepkg.NewEngine(reg, logr.Discard())
	e.SetCircuitBreaker(&alwaysOpenBreaker{})

	cmd := command.Command{Type: command.CmdCreate, GVK: benchGVK}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = e.Execute(context.Background(), cmd)
	}
}

// BenchmarkEngine_Queue_Async_WithWork - async with simulated work
func BenchmarkEngine_Queue_Async_WithWork(b *testing.B) {
	reg := registry.NewRegistry()
	d := &benchDomain{gvk: benchGVK, delay: 100 * time.Microsecond}
	reg.RegisterDomain(benchGVK, d)
	e := enginepkg.NewEngine(reg, logr.Discard())
	e.SetWorkers(4)
	e.StartWorkers()
	defer e.StopWorkers(time.Second)

	cmd := command.Command{Type: command.CmdCreate, GVK: benchGVK}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = e.Queue(context.Background(), cmd)
	}

	// Drain queue
	time.Sleep(200 * time.Millisecond)
}

// BenchmarkEngine_CircuitBreaker_Check_Overhead - measures CB check overhead
func BenchmarkEngine_CircuitBreaker_Check_Overhead(b *testing.B) {
	reg := registry.NewRegistry()
	d := &benchDomain{gvk: benchGVK}
	reg.RegisterDomain(benchGVK, d)
	e := enginepkg.NewEngine(reg, logr.Discard())
	e.SetCircuitBreaker(&alwaysOpenBreaker{})

	cmd := command.Command{Type: command.CmdCreate, GVK: benchGVK}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = e.Execute(context.Background(), cmd)
	}
}

// alwaysOpenBreaker always returns false (circuit open)
type alwaysOpenBreaker struct{}

func (a *alwaysOpenBreaker) Allow(cmd command.Command) bool { return false }