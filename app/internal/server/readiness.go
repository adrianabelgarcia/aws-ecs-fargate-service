package server

import "sync/atomic"

// Readiness reports whether the instance should receive traffic. It flips to
// false at the start of shutdown so the ALB stops routing new requests while
// in-flight requests finish. Liveness (/healthz) is deliberately independent.
type Readiness struct {
	ready atomic.Bool
}

// NewReadiness returns a Readiness that is initially not ready.
func NewReadiness() *Readiness { return &Readiness{} }

// SetReady marks the instance ready (true) or draining (false).
func (r *Readiness) SetReady(v bool) { r.ready.Store(v) }

// IsReady reports the current state.
func (r *Readiness) IsReady() bool { return r.ready.Load() }
