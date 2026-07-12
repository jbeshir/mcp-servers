package ratelimit

import (
	"context"
	"testing"
)

func TestWaitReturnsUnderPermissiveRate(t *testing.T) {
	l := New(1000, 10)
	if err := l.Wait(t.Context()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestWaitRespectsCancelledContext(t *testing.T) {
	l := New(0.001, 1)

	// The single burst token is available immediately, so drain it first...
	if err := l.Wait(t.Context()); err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	// ...then a second Wait must block on the (near-zero) rate and observe cancellation.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestNewClampsSubOneBurstToOne(t *testing.T) {
	l := New(1000, 0)
	if err := l.Wait(t.Context()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}
