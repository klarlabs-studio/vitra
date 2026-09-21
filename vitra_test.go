package vitra_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
	officialnotification "go.klarlabs.de/vitra/plugin/official/notification"
	officialpath "go.klarlabs.de/vitra/plugin/official/path"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
	"go.klarlabs.de/vitra/worker"
)

type echoExec struct{}

func (echoExec) Execute(_ context.Context, _ domain.CommandName, input any) (any, error) {
	return input, nil
}

func TestRuntime_SecureByDefault(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}

	cmd, _ := domain.NewCommandDefinition("notify.show", "Show notification", "notifications.show")
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}

	caller, err := rt.CallerFor("main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Invoke(context.Background(), domain.InvocationRequest{
		Caller:  caller,
		Command: "notify.show",
		Input:   "hi",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialNoGrant {
		t.Fatalf("expected no_grant, got %v", err)
	}

	report := mustInspect(t, rt, "main")
	if !strings.Contains(report, "(none)") {
		t.Fatalf("expected empty surface report:\n%s", report)
	}
}

func TestRuntime_GrantThenInvokeAndInspect(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}

	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"read project files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{
			Name:      "fs.read",
			PathScope: &domain.PathScope{Allow: []string{"/project/**"}, Deny: []string{"/project/.secrets/**"}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("project.open", "Open project", "fs.read")
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}

	caller, _ := rt.CallerFor("main")
	res, err := rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		Input:        "/project/README.md",
		ResourcePath: "/project/README.md",
	})
	if err != nil || !res.Authorized {
		t.Fatalf("invoke failed: %+v %v", res, err)
	}

	// Navigation drops authority.
	if err := rt.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		t.Fatal(err)
	}
	caller, _ = rt.CallerFor("main")
	_, err = rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		ResourcePath: "/project/README.md",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialOriginMismatch {
		t.Fatalf("expected origin denial after navigate, got %v", err)
	}

	// Restore and inspect.
	if err := rt.NavigateWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	report := mustInspect(t, rt, "main")
	if !strings.Contains(report, "project-files") || !strings.Contains(report, "fs.read") {
		t.Fatalf("inspect report missing grants:\n%s", report)
	}
}

func TestRuntime_RequiresAppID(t *testing.T) {
	if _, err := vitra.New(vitra.Config{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRuntime_CloseWindowAndNilExecutor(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("x", "x", "notifications.show")
	if err := rt.RegisterCommand(cmd, nil); err == nil {
		t.Fatal("expected nil executor error")
	}
	// Nil executor must not leave a registered command.
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}
	if err := rt.CloseWindow(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.CallerFor("main"); err == nil {
		t.Fatal("closed window should not yield caller")
	}
	if _, err := rt.CallerFor("missing"); err == nil {
		t.Fatal("expected missing window error")
	}
}

func TestRuntime_AuthorizeSharesGateway(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := domain.NewCaller("main", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}
	d := rt.Authorize(caller, "tray.set", "")
	if d.Allowed {
		t.Fatal("expected deny without grant")
	}
	grant, err := domain.NewCapabilityGrant(
		"chrome", "chrome",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "tray.set"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	d = rt.Authorize(caller, "tray.set", "")
	if !d.Allowed {
		t.Fatalf("expected allow after grant: %+v", d)
	}
}

func TestRuntime_PolicyOverlayTightensGrant(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	grant, err := domain.NewCapabilityGrant(
		"shell", "shell",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "shell.exec"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("shell.run", "Run shell", "shell.exec")
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}

	caller, _ := rt.CallerFor("main")
	d := rt.Authorize(caller, "shell.exec", "")
	if !d.Allowed {
		t.Fatalf("expected grant allow before policy: %+v", d)
	}
	res, err := rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "shell.run"})
	if err != nil || !res.Authorized {
		t.Fatalf("invoke before policy: %+v %v", res, err)
	}

	eng, err := policy.NewEngine(policy.Document{
		DenyPermissions: []domain.PermissionName{"shell.exec"},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetPolicy(eng)
	if rt.Policy() != eng {
		t.Fatal("Policy() should return installed engine")
	}

	d = rt.Authorize(caller, "shell.exec", "")
	if d.Allowed || d.Reason != "denied by enterprise policy" {
		t.Fatalf("expected policy deny on Authorize: %+v", d)
	}
	_, err = rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "shell.run"})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Reason != "denied by enterprise policy" {
		t.Fatalf("expected policy deny on Invoke, got %v", err)
	}

	rt.SetPolicy(nil)
	if rt.Policy() != nil {
		t.Fatal("nil SetPolicy should clear engine")
	}
	d = rt.Authorize(caller, "shell.exec", "")
	if !d.Allowed {
		t.Fatalf("clearing policy should restore grant allow: %+v", d)
	}
}

func TestRuntime_RegisterOfficialPlugins(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		t.Fatal(err)
	}
	if owner, ok := rt.Plugins().OwnerOf("fs.read"); !ok || owner != officialfs.PluginID {
		t.Fatalf("fs.read owner = %q ok=%v", owner, ok)
	}
	if owner, ok := rt.Plugins().OwnerOf("dialog.open"); !ok || owner != officialdialog.PluginID {
		t.Fatalf("dialog.open owner = %q ok=%v", owner, ok)
	}
	if owner, ok := rt.Plugins().OwnerOf("notifications.show"); !ok || owner != officialnotification.PluginID {
		t.Fatalf("notifications.show owner = %q ok=%v", owner, ok)
	}
	if owner, ok := rt.Plugins().OwnerOf("path.open"); !ok || owner != officialpath.PluginID {
		t.Fatalf("path.open owner = %q ok=%v", owner, ok)
	}

	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	fsGrant, err := domain.NewCapabilityGrant(
		"files", "files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.read", PathScope: &domain.PathScope{Allow: []string{"/**"}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(fsGrant); err != nil {
		t.Fatal(err)
	}
	caller, _ := rt.CallerFor("main")
	_, err = rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "fs.read", ResourcePath: "/x"})
	var nf *domain.ErrNotFound
	if !errors.As(err, &nf) || nf.Entity != "command executor" {
		t.Fatalf("expected unbound fs executor, got %v", err)
	}

	grant, err := domain.NewCapabilityGrant(
		"dialogs", "dialogs",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "dialog.open"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	if err := rt.BindExecutor("dialog.open", domain.CommandExecutorFunc(func(context.Context, domain.CommandName, any) (any, error) {
		return []string{"/tmp/a"}, nil
	})); err != nil {
		t.Fatal(err)
	}
	res, err := rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "dialog.open"})
	if err != nil || !res.Authorized {
		t.Fatalf("dialog invoke: %+v %v", res, err)
	}
}

func TestRuntime_RejectPluginPermissionCollision(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		t.Fatal(err)
	}
	err = rt.RegisterPlugin(ctx, collidingFS{})
	var conflict *domain.ErrConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRuntime_ApplyUpdate_PolicyAndInstall(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("demo-binary-v2")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.example.demo", Version: "2.0.0", Channel: updater.ChannelBeta,
		Artifact: "demo", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "demo")

	eng, err := policy.NewEngine(policy.Document{
		AllowedUpdateChannels: []updater.Channel{updater.ChannelStable},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetPolicy(eng)
	if _, err := rt.ApplyUpdate(m, pub, artifact, dest); err == nil {
		t.Fatal("expected beta channel deny under production policy")
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied update must not write dest: %v", err)
	}

	m.Channel = updater.ChannelStable
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := rt.ApplyUpdate(m, pub, artifact, dest)
	if err != nil || plan.Version != "2.0.0" {
		t.Fatalf("apply: plan=%+v err=%v", plan, err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(artifact) {
		t.Fatalf("dest=%q err=%v", got, err)
	}
}

func TestRuntime_AuditEmitsDecisions(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	sink := &audit.MemorySink{}
	rt.SetAudit(sink)
	if rt.Audit() != sink {
		t.Fatal("Audit() should return installed sink")
	}

	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		t.Fatal(err)
	}

	caller, _ := rt.CallerFor("main")
	d := rt.Authorize(caller, "fs.read", "/x")
	if d.Allowed {
		t.Fatal("expected deny without grant")
	}
	_, _ = rt.Invoke(ctx, domain.InvocationRequest{Caller: caller, Command: "fs.read", ResourcePath: "/x"})

	grant, err := domain.NewCapabilityGrant(
		"files", "files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "shell.exec"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	eng, err := policy.NewEngine(policy.Document{
		DenyPermissions: []domain.PermissionName{"shell.exec"},
	}, policy.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetPolicy(eng)
	d = rt.Authorize(caller, "shell.exec", "")
	if d.Allowed {
		t.Fatal("expected policy deny")
	}

	events := sink.List()
	kinds := map[audit.Kind]int{}
	for _, e := range events {
		kinds[e.Kind]++
	}
	if kinds[audit.KindPluginRegister] < 1 {
		t.Fatalf("missing plugin.register: %+v", events)
	}
	if kinds[audit.KindCapabilityDecision] < 2 {
		t.Fatalf("missing capability.decision: %+v", events)
	}
	if kinds[audit.KindCommandInvoke] < 1 {
		t.Fatalf("missing command.invoke: %+v", events)
	}
	if kinds[audit.KindPolicyOverride] < 1 {
		t.Fatalf("missing policy.override: %+v", events)
	}

	rt.SetAudit(nil)
	before := len(sink.List())
	_ = rt.Authorize(caller, "shell.exec", "")
	if len(sink.List()) != before {
		t.Fatal("cleared audit sink should stop recording")
	}
}

func TestRuntime_StartWorker_CrashAndStop(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	sink := &audit.MemorySink{}
	rt.SetAudit(sink)
	ctx := context.Background()

	if err := rt.StartWorker(ctx, worker.Spec{ID: "crashy", Name: "crashy", MaxRestarts: 0, Elevated: true}, func(context.Context) error {
		return errors.New("boom")
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var crashed bool
	for time.Now().Before(deadline) {
		rec, err := rt.Worker("crashy")
		if err != nil {
			t.Fatal(err)
		}
		if rec.State == worker.StateCrashed {
			crashed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !crashed {
		t.Fatal("expected crashed worker")
	}

	started := make(chan struct{})
	if err := rt.StartWorker(ctx, worker.Spec{ID: "ok", Name: "ok", MaxRestarts: 0}, func(c context.Context) error {
		close(started)
		<-c.Done()
		return c.Err()
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	if err := rt.StopWorker("ok"); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ := rt.Worker("ok")
		if rec.State == worker.StateStopped {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec, _ := rt.Worker("ok"); rec.State != worker.StateStopped {
		t.Fatalf("expected stopped, got %s", rec.State)
	}
	if n := len(rt.Workers()); n < 2 {
		t.Fatalf("Workers() = %d", n)
	}

	kinds := map[audit.Kind]int{}
	for _, e := range sink.List() {
		kinds[e.Kind]++
	}
	if kinds[audit.KindWorkerLifecycle] < 3 {
		t.Fatalf("expected worker.lifecycle audits, got %+v", sink.List())
	}
}

type collidingFS struct{}

func (collidingFS) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID: "evil.fs", Name: "Evil", Version: plugin.SemVer{Major: 1},
		Permissions: []domain.PermissionName{"fs.read"},
		MinKernel:   plugin.SemVer{Major: 0, Minor: 3},
	}
}

func (collidingFS) Contribute() (plugin.Contribution, error) {
	cmd, err := domain.NewCommandDefinition("evil.read", "x", "fs.read")
	if err != nil {
		return plugin.Contribution{}, err
	}
	return plugin.Contribution{Commands: []*domain.CommandDefinition{cmd}}, nil
}

func mustInspect(t *testing.T, rt *vitra.Runtime, window domain.WindowID) string {
	t.Helper()
	surface, err := rt.InspectCapabilities(window)
	if err != nil {
		t.Fatal(err)
	}
	return vitra.FormatInspect(rt.AppID(), surface)
}
