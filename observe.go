package vitra

import (
	"context"
	"time"

	"go.klarlabs.de/vitra/domain"
)

// Observer watches the command invocations the capability gateway
// authorized: for logging, metrics and tracing.
//
// It is not middleware. It runs only after the gateway has allowed the call
// and cannot change the decision, the caller, the input or the result.
// Start may add to the handler's context (a trace span, a request-scoped
// logger); the runtime then re-attaches the authorized invocation, so an
// observer cannot change who the handler thinks called it. Denied calls
// never reach observers: the audit sink (SetAudit) records them. A panic in
// an observer is recovered and ignored.
type Observer interface {
	// Start is called before the handler runs and returns the context the
	// handler receives.
	Start(ctx context.Context, inv domain.Invocation) context.Context
	// Finish is called after the handler returns, with the context Start
	// returned, how long the handler took, and its error.
	Finish(ctx context.Context, inv domain.Invocation, elapsed time.Duration, err error)
}

// Observe adds o to the observers of authorized command invocations.
// Observers start in the order they were added and finish in reverse, as
// nested spans do. Observers cannot be removed.
func (rt *Runtime) Observe(o Observer) {
	if o == nil {
		return
	}
	rt.observersMu.Lock()
	defer rt.observersMu.Unlock()
	rt.observers = append(rt.observers, o)
}

// observe is the InvocationService hook: it runs the observers around one
// authorized execution.
func (rt *Runtime) observe(ctx context.Context, inv domain.Invocation) (context.Context, func(error)) {
	rt.observersMu.Lock()
	observers := append([]Observer(nil), rt.observers...)
	rt.observersMu.Unlock()
	if len(observers) == 0 {
		return ctx, func(error) {}
	}
	ctxs := make([]context.Context, len(observers))
	for i, o := range observers {
		ctx = startObserver(o, ctx, inv)
		ctxs[i] = ctx
	}
	began := time.Now()
	return ctx, func(err error) {
		elapsed := time.Since(began)
		for i := len(observers) - 1; i >= 0; i-- {
			finishObserver(observers[i], ctxs[i], inv, elapsed, err)
		}
	}
}

// startObserver runs o.Start, keeping ctx when it panics or returns nil.
func startObserver(o Observer, ctx context.Context, inv domain.Invocation) (next context.Context) {
	next = ctx
	defer func() {
		if recover() != nil {
			next = ctx
		}
	}()
	if c := o.Start(ctx, inv); c != nil {
		next = c
	}
	return next
}

func finishObserver(o Observer, ctx context.Context, inv domain.Invocation, elapsed time.Duration, err error) {
	defer func() { _ = recover() }()
	o.Finish(ctx, inv, elapsed, err)
}
