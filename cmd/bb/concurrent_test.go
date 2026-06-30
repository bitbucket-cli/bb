package main

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestRunConcurrent(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	var calls int32

	errs := runConcurrent(items, 2, func(n int) error {
		atomic.AddInt32(&calls, 1)
		if n%2 == 0 {
			return fmt.Errorf("even: %d", n)
		}
		return nil
	})

	if calls != 5 {
		t.Errorf("calls = %d, want 5", calls)
	}
	if len(errs) != 5 {
		t.Fatalf("len(errs) = %d, want 5", len(errs))
	}
	// Errors must align with input order.
	for i, n := range items {
		if n%2 == 0 && errs[i] == nil {
			t.Errorf("item %d (%d) should have errored", i, n)
		}
		if n%2 == 1 && errs[i] != nil {
			t.Errorf("item %d (%d) should not have errored: %v", i, n, errs[i])
		}
	}
}

func TestRunConcurrent_Empty(t *testing.T) {
	if errs := runConcurrent([]string{}, 4, func(string) error { return nil }); len(errs) != 0 {
		t.Errorf("expected no errors for empty input, got %d", len(errs))
	}
}
