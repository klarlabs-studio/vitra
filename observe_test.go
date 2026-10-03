package vitra_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

type ctxKey string

// recorder is an Observer that records what it saw and tags the handler's
// context, as a tracer adds its span.
type recorder struct {
	name   string
	mu     sync.Mutex
	starts []domain.Invocation
	ends   []observed
	order  *[]string
}

type observed struct {
	inv     domain.Invocation
	elapsed time.Duration
	err     error
	tagged  bool
}

func (r *recorder) Start(ctx context.Context, inv domain.Invocation) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, inv)
	if r.order != nil {
		*r.order = append(*r.order, "start "+r.name)
	}
	return context.WithValue(ctx, ctxKey(r.name), r.name)
}

func (r *recorder) Finish(ctx context.Context, inv domain.Invocation, elapsed time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ends = append(r.ends, observed{inv, elapsed, err, ctx.Value(ctxKey(r.name)) == r.name})
	if r.order != nil {
		*r.order = append(*r.order, "finish "+r.name)
	}
}

type pingIn struct {
	Fail bool `json:"fail,omitempty"`
}

// observedRuntime has a window "main" granted app.ping, and a typed
// app.ping command that reports the trace tag it sees.
func observedRuntime(t *testing.T) (*vitra.Runtime, domain.Caller) {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.observe"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(ctx, "other", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	g, err := domain.NewCapabilityGrant("ping", "ping", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal}, []domain.PermissionSpec{{Name: "app.ping"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(g); err != nil {
		t.Fatal(err)
	}
	err = vitra.Register(rt, vitra.Command[pingIn, string]{
		Name: "app.ping", Permission: "app.ping",
		Handler: func(ctx context.Context, inv domain.Invocation, in pingIn) (string, error) {
			if in.Fail {
				return "", errors.New("ping failed")
			}
			tag, _ := ctx.Value(ctxKey("trace")).(string)
			return string(inv.Caller.Window) + ":" + tag, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := rt.CallerFor("main")
	if err != nil {
		t.Fatal(err)
	}
	return rt, caller
}

// An observer sees each authorized invocation with its caller, grant,
// duration and error, and what it adds to the context reaches the handler.
func TestObserve_AuthorizedInvocations(t *testing.T) {
	rt, caller := observedRuntime(t)
	rec := &recorder{name: "trace"}
	rt.Observe(rec)
	ctx := context.Background()

	res, err := rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "main:trace" {
		t.Fatalf("handler output %v: the observer's context did not reach it", res.Output)
	}
	_, err = rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{"fail": true}})
	if err == nil {
		t.Fatal("failing handler succeeded")
	}

	if len(rec.starts) != 2 || len(rec.ends) != 2 {
		t.Fatalf("starts %d, ends %d", len(rec.starts), len(rec.ends))
	}
	first := rec.ends[0]
	if first.inv.Caller != caller || first.inv.Command != "app.ping" || first.inv.Grant != "ping" {
		t.Fatalf("observed invocation %+v", first.inv)
	}
	if first.err != nil || first.elapsed < 0 || !first.tagged {
		t.Fatalf("first finish %+v", first)
	}
	if rec.ends[1].err == nil || rec.ends[1].err.Error() != "ping failed" {
		t.Fatalf("second finish err %v", rec.ends[1].err)
	}
}

// Denied calls never reach observers: the audit sink records them.
func TestObserve_DeniedInvocationsAreNotObserved(t *testing.T) {
	rt, _ := observedRuntime(t)
	rec := &recorder{name: "trace"}
	rt.Observe(rec)
	other, err := rt.CallerFor("other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: other, Command: "app.ping", Input: map[string]any{}}); err == nil {
		t.Fatal("ungranted window invoked app.ping")
	}
	if len(rec.starts) != 0 || len(rec.ends) != 0 {
		t.Fatalf("observer saw a denied call: %+v %+v", rec.starts, rec.ends)
	}
}

// spoofer tries to change who the handler thinks called it.
type spoofer struct{}

func (spoofer) Start(ctx context.Context, inv domain.Invocation) context.Context {
	inv.Caller.Window = "other"
	inv.Grant = "forged"
	return domain.WithInvocation(ctx, inv)
}

func (spoofer) Finish(context.Context, domain.Invocation, time.Duration, error) {}

// An observer cannot change the caller the handler sees.
func TestObserve_CannotReplaceTheInvocation(t *testing.T) {
	rt, caller := observedRuntime(t)
	rt.Observe(spoofer{})
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "main:" {
		t.Fatalf("handler saw caller %v after an observer replaced the invocation", res.Output)
	}
}

type panicker struct{ inStart bool }

func (p panicker) Start(ctx context.Context, _ domain.Invocation) context.Context {
	if p.inStart {
		panic("observer start")
	}
	return ctx
}

func (p panicker) Finish(context.Context, domain.Invocation, time.Duration, error) {
	if !p.inStart {
		panic("observer finish")
	}
}

// A broken observer cannot break or change the command.
func TestObserve_PanickingObserverIsContained(t *testing.T) {
	rt, caller := observedRuntime(t)
	rec := &recorder{name: "trace"}
	rt.Observe(panicker{inStart: true})
	rt.Observe(rec)
	rt.Observe(panicker{inStart: false})
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{}})
	if err != nil || res.Output != "main:trace" {
		t.Fatalf("invoke with panicking observers = %v, %v", res, err)
	}
	if len(rec.ends) != 1 {
		t.Fatalf("the healthy observer finished %d times", len(rec.ends))
	}
}

// Observers start in the order added and finish in reverse, like nested
// spans.
func TestObserve_NestedOrder(t *testing.T) {
	rt, caller := observedRuntime(t)
	var order []string
	rt.Observe(&recorder{name: "outer", order: &order})
	rt.Observe(&recorder{name: "inner", order: &order})
	if _, err := rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"start outer", "start inner", "finish inner", "finish outer"}
	if len(order) != len(want) {
		t.Fatalf("order %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
}

// Adding observers while commands run is safe.
func TestObserve_ConcurrentWithInvocations(t *testing.T) {
	rt, caller := observedRuntime(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			rt.Observe(&recorder{name: "trace"})
		}()
		go func() {
			defer wg.Done()
			_, _ = rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "app.ping", Input: map[string]any{"fail": i%2 == 0}})
		}()
	}
	wg.Wait()
}
