package main

import (
	"context"

	"inflacao-real/core/internal/historyprecompute"
)

func startPrecomputation(parent context.Context, store *historyprecompute.Store) func() {
	requestContext, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() { defer close(done); store.Run(requestContext) }()
	return func() { cancel(); <-done }
}
