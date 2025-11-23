package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

func TestNew(t *testing.T) {
	t.Run("creates rate limiter with correct initial state", func(t *testing.T) {
		rl := New(10, time.Second)

		if rl.rate != 10 {
			t.Errorf("expected rate 10, got %d", rl.rate)
		}
		if rl.interval != time.Second {
			t.Errorf("expected interval 1s, got %v", rl.interval)
		}
		if rl.capacity != 10 {
			t.Errorf("expected initial capacity 10, got %d", rl.capacity)
		}
		if rl.maxCapacity != 10 {
			t.Errorf("expected max capacity 10, got %d", rl.maxCapacity)
		}
	})
}

func TestAllow(t *testing.T) {
	t.Run("allows requests within capacity", func(t *testing.T) {
		rl := New(5, time.Second)

		// Should allow 5 requests
		for i := 0; i < 5; i++ {
			if !rl.Allow() {
				t.Errorf("request %d should be allowed", i+1)
			}
		}

		// 6th request should be rejected (capacity exhausted)
		if rl.Allow() {
			t.Error("request 6 should be rejected")
		}
	})

	t.Run("rejects requests when capacity exhausted", func(t *testing.T) {
		rl := New(2, time.Second)

		// Exhaust capacity
		rl.Allow()
		rl.Allow()

		// Should reject
		if rl.Allow() {
			t.Error("should reject when capacity is 0")
		}
	})

	t.Run("refills capacity over time", func(t *testing.T) {
		// 10 requests per second = 1 request per 100ms
		rl := New(10, time.Second)

		// Exhaust capacity
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		// Should be rejected immediately
		if rl.Allow() {
			t.Error("should reject immediately after exhaustion")
		}

		// Wait for capacity to refill (200ms = ~2 requests)
		time.Sleep(200 * time.Millisecond)

		// Should allow 1-2 requests now
		if !rl.Allow() {
			t.Error("should allow after refill period")
		}
	})
}

func TestGetAvailableCapacity(t *testing.T) {
	t.Run("returns correct available capacity", func(t *testing.T) {
		rl := New(10, time.Second)

		if capacity := rl.GetAvailableCapacity(); capacity != 10 {
			t.Errorf("expected capacity 10, got %d", capacity)
		}

		// Consume 3 slots
		rl.Allow()
		rl.Allow()
		rl.Allow()

		if capacity := rl.GetAvailableCapacity(); capacity != 7 {
			t.Errorf("expected capacity 7, got %d", capacity)
		}
	})

	t.Run("accounts for refill when checking capacity", func(t *testing.T) {
		rl := New(10, time.Second)

		// Exhaust capacity
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		if capacity := rl.GetAvailableCapacity(); capacity != 0 {
			t.Errorf("expected capacity 0, got %d", capacity)
		}

		// Wait for refill
		time.Sleep(500 * time.Millisecond)

		// Should have refilled some capacity
		capacity := rl.GetAvailableCapacity()
		if capacity < 3 || capacity > 6 {
			t.Errorf("expected capacity between 3-6 after 500ms, got %d", capacity)
		}
	})
}

func TestReset(t *testing.T) {
	t.Run("resets to full capacity", func(t *testing.T) {
		rl := New(10, time.Second)

		// Consume some capacity
		for i := 0; i < 7; i++ {
			rl.Allow()
		}

		if capacity := rl.GetAvailableCapacity(); capacity != 3 {
			t.Errorf("expected capacity 3 before reset, got %d", capacity)
		}

		// Reset
		rl.Reset()

		if capacity := rl.GetAvailableCapacity(); capacity != 10 {
			t.Errorf("expected capacity 10 after reset, got %d", capacity)
		}
	})
}

func TestWait(t *testing.T) {
	t.Run("returns immediately when capacity available", func(t *testing.T) {
		rl := New(5, time.Second)

		ctx := context.Background()
		start := time.Now()

		if err := rl.Wait(ctx); err != nil {
			t.Errorf("Wait should not error: %v", err)
		}

		elapsed := time.Since(start)
		if elapsed > 50*time.Millisecond {
			t.Errorf("Wait should return immediately, took %v", elapsed)
		}
	})

	t.Run("waits when capacity exhausted", func(t *testing.T) {
		// 10 requests per second = 1 request per 100ms
		rl := New(10, time.Second)

		// Exhaust capacity
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		ctx := context.Background()
		start := time.Now()

		if err := rl.Wait(ctx); err != nil {
			t.Errorf("Wait should not error: %v", err)
		}

		elapsed := time.Since(start)
		// Should wait at least ~100ms for one slot to refill
		if elapsed < 50*time.Millisecond {
			t.Errorf("Wait should wait for refill, took only %v", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		rl := New(1, time.Second)

		// Exhaust capacity
		rl.Allow()

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := rl.Wait(ctx)
		if err == nil {
			t.Error("Wait should return error when context cancelled")
		}
		if err != context.DeadlineExceeded {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	})
}

func TestExecute(t *testing.T) {
	t.Run("executes function when capacity available", func(t *testing.T) {
		rl := New(5, time.Second)

		executed := false
		fn := func() error {
			executed = true
			return nil
		}

		ctx := context.Background()
		if err := rl.Execute(ctx, fn); err != nil {
			t.Errorf("Execute should not error: %v", err)
		}

		if !executed {
			t.Error("function should have been executed")
		}
	})

	t.Run("waits before executing when capacity exhausted", func(t *testing.T) {
		rl := New(10, time.Second)

		// Exhaust capacity
		for i := 0; i < 10; i++ {
			rl.Allow()
		}

		executed := false
		fn := func() error {
			executed = true
			return nil
		}

		ctx := context.Background()
		start := time.Now()

		if err := rl.Execute(ctx, fn); err != nil {
			t.Errorf("Execute should not error: %v", err)
		}

		elapsed := time.Since(start)
		if elapsed < 50*time.Millisecond {
			t.Errorf("Execute should wait for refill, took only %v", elapsed)
		}

		if !executed {
			t.Error("function should have been executed after waiting")
		}
	})

	t.Run("returns function error", func(t *testing.T) {
		rl := New(5, time.Second)

		expectedErr := errors.New(errors.ErrorCodeUnknown, "test error")
		fn := func() error {
			return expectedErr
		}

		ctx := context.Background()
		err := rl.Execute(ctx, fn)

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestTryExecute(t *testing.T) {
	t.Run("executes when capacity available", func(t *testing.T) {
		rl := New(5, time.Second)

		executed := false
		fn := func() error {
			executed = true
			return nil
		}

		if err := rl.TryExecute(fn); err != nil {
			t.Errorf("TryExecute should not error: %v", err)
		}

		if !executed {
			t.Error("function should have been executed")
		}
	})

	t.Run("returns rate limit error when capacity exhausted", func(t *testing.T) {
		rl := New(2, time.Second)

		// Exhaust capacity
		rl.Allow()
		rl.Allow()

		executed := false
		fn := func() error {
			executed = true
			return nil
		}

		err := rl.TryExecute(fn)
		if err == nil {
			t.Error("TryExecute should return error when capacity exhausted")
		}

		if errors.GetCode(err) != errors.ErrorCodeRateLimitExceeded {
			t.Errorf("expected rate limit error, got %v", err)
		}

		if executed {
			t.Error("function should not have been executed")
		}
	})

	t.Run("returns function error", func(t *testing.T) {
		rl := New(5, time.Second)

		expectedErr := errors.New(errors.ErrorCodeUnknown, "test error")
		fn := func() error {
			return expectedErr
		}

		err := rl.TryExecute(fn)
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("handles concurrent Allow calls safely", func(t *testing.T) {
		rl := New(100, time.Second)

		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		// Launch 150 concurrent requests (should allow 100)
		for i := 0; i < 150; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if rl.Allow() {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// Should have allowed exactly 100 requests
		if successCount != 100 {
			t.Errorf("expected 100 successful requests, got %d", successCount)
		}
	})

	t.Run("handles concurrent operations safely", func(t *testing.T) {
		rl := New(50, time.Second)

		var wg sync.WaitGroup

		// Mix of Allow, GetAvailableCapacity, and Reset calls
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				switch n % 3 {
				case 0:
					rl.Allow()
				case 1:
					rl.GetAvailableCapacity()
				case 2:
					if n > 50 {
						rl.Reset()
					}
				}
			}(i)
		}

		wg.Wait()

		// Should not panic or deadlock
		capacity := rl.GetAvailableCapacity()
		if capacity < 0 || capacity > 50 {
			t.Errorf("capacity should be between 0-50, got %d", capacity)
		}
	})
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("PerSecond creates correct rate limiter", func(t *testing.T) {
		rl := PerSecond(10)

		if rl.rate != 10 {
			t.Errorf("expected rate 10, got %d", rl.rate)
		}
		if rl.interval != time.Second {
			t.Errorf("expected interval 1s, got %v", rl.interval)
		}
	})

	t.Run("PerMinute creates correct rate limiter", func(t *testing.T) {
		rl := PerMinute(60)

		if rl.rate != 60 {
			t.Errorf("expected rate 60, got %d", rl.rate)
		}
		if rl.interval != time.Minute {
			t.Errorf("expected interval 1m, got %v", rl.interval)
		}
	})

	t.Run("PerHour creates correct rate limiter", func(t *testing.T) {
		rl := PerHour(3600)

		if rl.rate != 3600 {
			t.Errorf("expected rate 3600, got %d", rl.rate)
		}
		if rl.interval != time.Hour {
			t.Errorf("expected interval 1h, got %v", rl.interval)
		}
	})
}

func TestRefillMechanism(t *testing.T) {
	t.Run("refills capacity proportionally to elapsed time", func(t *testing.T) {
		// 20 requests per second = 1 request per 50ms
		rl := New(20, time.Second)

		// Exhaust capacity
		for i := 0; i < 20; i++ {
			rl.Allow()
		}

		// Wait for ~500ms (should refill ~10 requests)
		time.Sleep(500 * time.Millisecond)

		capacity := rl.GetAvailableCapacity()
		// Allow some variance due to timing
		if capacity < 8 || capacity > 12 {
			t.Errorf("expected capacity between 8-12 after 500ms, got %d", capacity)
		}
	})

	t.Run("does not exceed max capacity", func(t *testing.T) {
		rl := New(10, time.Second)

		// Consume some capacity
		for i := 0; i < 5; i++ {
			rl.Allow()
		}

		// Wait longer than interval
		time.Sleep(1500 * time.Millisecond)

		// Should be capped at maxCapacity (10)
		capacity := rl.GetAvailableCapacity()
		if capacity != 10 {
			t.Errorf("expected capacity capped at 10, got %d", capacity)
		}
	})

	t.Run("refills continuously over time", func(t *testing.T) {
		// 100 requests per second = 1 request per 10ms
		rl := New(100, time.Second)

		// Exhaust capacity
		for i := 0; i < 100; i++ {
			rl.Allow()
		}

		// Check capacity increases over time
		previousCapacity := 0
		for i := 0; i < 5; i++ {
			time.Sleep(30 * time.Millisecond)
			capacity := rl.GetAvailableCapacity()
			if capacity <= previousCapacity {
				t.Errorf("capacity should increase over time, iteration %d: previous=%d, current=%d",
					i, previousCapacity, capacity)
			}
			previousCapacity = capacity
		}
	})
}
