package main

import "sync"

// runConcurrent runs fn for each item with bounded concurrency, returning the
// per-item errors in the original order. Used to parallelise independent API
// calls (e.g. bulk operations).
func runConcurrent[T any](items []T, limit int, fn func(T) error) []error {
	if limit < 1 {
		limit = 1
	}
	errs := make([]error, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	for i, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, it T) {
			defer wg.Done()
			defer func() { <-sem }()
			errs[i] = fn(it)
		}(i, it)
	}
	wg.Wait()
	return errs
}
