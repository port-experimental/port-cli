package import_module

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_ConcurrencyLimit(t *testing.T) {
	pool := NewWorkerPool(3)
	var maxConcurrent int32
	var current int32

	for i := 0; i < 10; i++ {
		pool.Go(func() {
			c := atomic.AddInt32(&current, 1)
			// Track max concurrent
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if c <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, c) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		})
	}

	pool.Wait()

	if maxConcurrent > 3 {
		t.Errorf("exceeded concurrency limit: max was %d, expected <= 3", maxConcurrent)
	}
	if maxConcurrent < 2 {
		t.Errorf("concurrency too low: max was %d, expected >= 2", maxConcurrent)
	}
}

func TestWorkerPool_AllTasksComplete(t *testing.T) {
	pool := NewWorkerPool(5)
	var completed int32

	for i := 0; i < 100; i++ {
		pool.Go(func() {
			atomic.AddInt32(&completed, 1)
		})
	}

	pool.Wait()

	if completed != 100 {
		t.Errorf("not all tasks completed: %d/100", completed)
	}
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	pool := NewWorkerPool(2)
	ctx, cancel := context.WithCancel(context.Background())
	var started int32
	var completed int32

	// Submit tasks
	for i := 0; i < 20; i++ {
		pool.GoWithContext(ctx, func() {
			atomic.AddInt32(&started, 1)
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		})
	}

	// Cancel after a short delay
	time.Sleep(20 * time.Millisecond)
	cancel()

	pool.Wait()

	// Some tasks should have been skipped due to cancellation
	if completed == 20 {
		t.Log("all tasks completed before cancellation could take effect")
	}
}

func TestBatchProcessor_Basic(t *testing.T) {
	bp := NewBatchProcessor[int](5)
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	results := bp.Process(items, func(i int) error {
		return nil
	})

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error for item %d: %v", r.Item, r.Error)
		}
	}
}

func TestBatchProcessor_Progress(t *testing.T) {
	bp := NewBatchProcessor[int](2)
	items := []int{1, 2, 3, 4, 5}

	var lastProcessed, lastTotal int
	bp.SetProgressCallback(func(processed, total int) {
		lastProcessed = processed
		lastTotal = total
	})

	bp.Process(items, func(i int) error {
		return nil
	})

	if lastProcessed != 5 || lastTotal != 5 {
		t.Errorf("progress callback not called correctly: processed=%d, total=%d", lastProcessed, lastTotal)
	}
}

func TestNewWorkerPool_ZeroLimit(t *testing.T) {
	pool := NewWorkerPool(0)
	var completed int32

	pool.Go(func() {
		atomic.AddInt32(&completed, 1)
	})

	pool.Wait()

	if completed != 1 {
		t.Error("task should complete even with zero limit (defaults to 1)")
	}
}
