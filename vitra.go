// Package vitra is the entry point for the Vitra runtime kernel — a secure,
// capability-oriented desktop application runtime for Go + web frontends.
//
// Phase 2 adds desktop completeness on top of the secure kernel: multi-window
// lifecycle, navigation policy, window-owned subscriptions, and capability-
// gated desktop services (menu/tray/dialog/clipboard/shortcuts/deeplinks).
// Phase 3 plugins register through RegisterPlugin; hosts BindExecutor for
// contributed commands. Phase 5 enterprise policy installs via SetPolicy and
// can only tighten Authorize/Invoke decisions. SetAudit records capability,
// plugin, and update outcomes for fleet diagnostics.
//
// Example:
//
//	rt := vitra.New(vitra.Config{AppID: "com.example.demo"})
//	_ = rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal)
//	_ = rt.RegisterGrant(grant)
//	_ = rt.RegisterCommand(cmd, executor)
//	result, err := rt.Invoke(ctx, domain.InvocationRequest{...})
package vitra

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.klarlabs.de/vitra/application"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/inmemory"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
	"go.klarlabs.de/vitra/worker"
)

// Version is the kernel API version. Generated frontend bindings should be
// tied to this version (reliability invariant 8).
const Version = "0.3.0"

// Config configures a Runtime.
type Config struct {
	AppID            domain.AppID
	TrustedOrigins   []domain.Origin
	AllowExternalNav bool
}

// Runtime is the fluent entry point for the Vitra kernel.
// A Runtime is NOT safe to configure concurrently; Invoke is safe once
// configuration is complete.
type Runtime struct {
	appID domain.AppID

	grants        domain.GrantRepository
	windows       domain.WindowRepository
	commands      domain.CommandRepository
	resources     domain.ResourceRepository
	subscriptions domain.SubscriptionRepository
	executors     *inmemory.ExecutorRegistry
	navPolicy     *domain.NavigationPolicy
	plugins       *plugin.Registry
	policyEng     *policy.Engine
	auditSink     audit.Sink
	workers       *worker.Supervisor
	invoker       *domain.InvocationService

	registerGrant   *application.RegisterGrantUseCase
	registerCommand *application.RegisterCommandUseCase
	openWindow      *application.OpenWindowUseCase
	navigateWindow  *application.NavigateWindowUseCase
	navigatePolicy  *application.NavigateWithPolicyUseCase
	closeWindow     *application.CloseWindowUseCase
	invoke          *application.InvokeCommandUseCase
	inspect         *application.InspectCapabilitiesUseCase
	subscribe       *application.SubscribeEventUseCase
	emit            *application.EmitEventUseCase
}

// New constructs a Runtime with in-memory adapters.
func New(cfg Config) (*Runtime, error) {
	if cfg.AppID == "" {
		return nil, &domain.ErrValidation{Message: "app id is required"}
	}
	policy, err := domain.NewNavigationPolicy(cfg.TrustedOrigins, cfg.AllowExternalNav)
	if err != nil {
		return nil, err
	}
	kernel, err := ParseKernelVersion(Version)
	if err != nil {
		return nil, err
	}
	rt := &Runtime{
		appID:         cfg.AppID,
		grants:        inmemory.NewGrantRepo(),
		windows:       inmemory.NewWindowRepo(),
		commands:      inmemory.NewCommandRepo(),
		resources:     inmemory.NewResourceRepo(),
		subscriptions: inmemory.NewSubscriptionRepo(),
		executors:     inmemory.NewExecutorRegistry(),
		navPolicy:     policy,
		plugins:       plugin.NewRegistry(kernel),
		workers:       worker.NewSupervisor(),
	}
	rt.wire()
	return rt, nil
}

// ParseKernelVersion parses a dotted major.minor.patch kernel version string.
func ParseKernelVersion(v string) (plugin.SemVer, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return plugin.SemVer{}, &domain.ErrValidation{Message: "kernel version must be major.minor.patch"}
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return plugin.SemVer{}, &domain.ErrValidation{Message: "kernel version must be numeric major.minor.patch"}
	}
	return plugin.SemVer{Major: maj, Minor: min, Patch: pat}, nil
}

func (rt *Runtime) wire() {
	rt.workers.OnTransition = func(rec worker.Record) {
		outcome := "allowed"
		if rec.State == worker.StateCrashed {
			outcome = "error"
		}
		rt.emitAudit(audit.Event{
			Kind:    audit.KindWorkerLifecycle,
			Actor:   string(rec.Spec.ID),
			Action:  string(rec.State),
			Outcome: outcome,
			Detail:  rec.LastError,
			Metadata: map[string]any{
				"name":     rec.Spec.Name,
				"elevated": rec.Spec.Elevated,
				"restarts": rec.Restarts,
			},
		})
	}
	rt.invoker = &domain.InvocationService{
		Commands:  rt.commands,
		Grants:    rt.grants,
		Windows:   rt.windows,
		Executors: rt.executors,
	}
	rt.registerGrant = &application.RegisterGrantUseCase{Grants: rt.grants}
	rt.registerCommand = &application.RegisterCommandUseCase{Commands: rt.commands}
	rt.openWindow = &application.OpenWindowUseCase{Windows: rt.windows}
	rt.navigateWindow = &application.NavigateWindowUseCase{Windows: rt.windows}
	rt.navigatePolicy = &application.NavigateWithPolicyUseCase{Windows: rt.windows, Policy: rt.navPolicy}
	rt.closeWindow = &application.CloseWindowUseCase{
		Windows: rt.windows, Resources: rt.resources, Subscriptions: rt.subscriptions,
	}
	rt.invoke = &application.InvokeCommandUseCase{Invoker: rt.invoker}
	rt.inspect = &application.InspectCapabilitiesUseCase{Grants: rt.grants, Windows: rt.windows}
	rt.subscribe = &application.SubscribeEventUseCase{Windows: rt.windows, Subscriptions: rt.subscriptions}
	rt.emit = &application.EmitEventUseCase{Windows: rt.windows, Subscriptions: rt.subscriptions}
}

// AppID returns the application id.
func (rt *Runtime) AppID() domain.AppID { return rt.appID }

// Plugins returns the plugin registry.
func (rt *Runtime) Plugins() *plugin.Registry { return rt.plugins }

// SetPolicy installs an enterprise policy overlay (Phase 5). Nil clears it.
// Policy can only tighten grants — never loosen denials.
func (rt *Runtime) SetPolicy(eng *policy.Engine) {
	rt.policyEng = eng
	if eng == nil {
		rt.invoker.Overlay = nil
		return
	}
	rt.invoker.Overlay = eng.OverlayDecision
}

// Policy returns the installed enterprise policy engine, if any.
func (rt *Runtime) Policy() *policy.Engine { return rt.policyEng }

// SetAudit installs an audit sink (Phase 5). Nil clears it.
func (rt *Runtime) SetAudit(s audit.Sink) { rt.auditSink = s }

// Audit returns the installed audit sink, if any.
func (rt *Runtime) Audit() audit.Sink { return rt.auditSink }

func (rt *Runtime) emitAudit(e audit.Event) {
	if rt.auditSink == nil {
		return
	}
	_ = rt.auditSink.Append(e)
}

// StartWorker supervises an in-process worker (Phase 5). Elevated work belongs
// here so the WebView host stays non-admin (invariant 8). OS process adapters
// are out of scope for this facade.
func (rt *Runtime) StartWorker(ctx context.Context, spec worker.Spec, run worker.Runner) error {
	if err := rt.workers.Start(ctx, spec, run); err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindWorkerLifecycle, Actor: string(spec.ID), Action: "start",
			Outcome: "error", Detail: err.Error(),
		})
		return err
	}
	rt.emitAudit(audit.Event{
		Kind: audit.KindWorkerLifecycle, Actor: string(spec.ID), Action: "start",
		Outcome: "allowed", Detail: spec.Name,
		Metadata: map[string]any{"elevated": spec.Elevated},
	})
	return nil
}

// StopWorker requests a graceful stop.
func (rt *Runtime) StopWorker(id worker.ID) error {
	if err := rt.workers.Stop(id); err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindWorkerLifecycle, Actor: string(id), Action: "stop",
			Outcome: "error", Detail: err.Error(),
		})
		return err
	}
	rt.emitAudit(audit.Event{
		Kind: audit.KindWorkerLifecycle, Actor: string(id), Action: "stop", Outcome: "allowed",
	})
	return nil
}

// Worker returns inspectable bookkeeping for one worker.
func (rt *Runtime) Worker(id worker.ID) (worker.Record, error) {
	return rt.workers.Get(id)
}

// Workers lists all supervised worker records.
func (rt *Runtime) Workers() []worker.Record { return rt.workers.List() }

// RegisterGrant installs a capability grant.
func (rt *Runtime) RegisterGrant(grant *domain.CapabilityGrant) error {
	return rt.registerGrant.Execute(grant)
}

// RegisterCommand registers a command and its executor.
func (rt *Runtime) RegisterCommand(cmd *domain.CommandDefinition, exec domain.CommandExecutor) error {
	if exec == nil {
		return &domain.ErrValidation{Message: "command executor is required"}
	}
	if err := rt.registerCommand.Execute(cmd); err != nil {
		return err
	}
	return rt.executors.Register(cmd.Name(), exec)
}

// RegisterPlugin validates and installs a plugin contribution.
// Command definitions are registered without executors; call BindExecutor
// (or RegisterCommand for app-owned commands) to attach host handlers.
func (rt *Runtime) RegisterPlugin(ctx context.Context, p plugin.Plugin) error {
	id := string(p.Manifest().ID)
	if err := rt.plugins.Register(ctx, p); err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindPluginRegister, Actor: id, Action: "register",
			Outcome: "error", Detail: err.Error(),
		})
		return err
	}
	reg, err := rt.plugins.Get(p.Manifest().ID)
	if err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindPluginRegister, Actor: id, Action: "register",
			Outcome: "error", Detail: err.Error(),
		})
		return err
	}
	for _, cmd := range reg.Contribution.Commands {
		if err := rt.registerCommand.Execute(cmd); err != nil {
			rt.emitAudit(audit.Event{
				Kind: audit.KindPluginRegister, Actor: id, Action: "register",
				Outcome: "error", Detail: err.Error(),
			})
			return err
		}
	}
	rt.emitAudit(audit.Event{
		Kind: audit.KindPluginRegister, Actor: id, Action: "register", Outcome: "allowed",
	})
	return nil
}

// BindExecutor attaches a host executor to an already-registered command
// (typically one contributed by a plugin).
func (rt *Runtime) BindExecutor(name domain.CommandName, exec domain.CommandExecutor) error {
	if exec == nil {
		return &domain.ErrValidation{Message: "command executor is required"}
	}
	if _, err := rt.commands.Get(name); err != nil {
		return err
	}
	return rt.executors.Register(name, exec)
}

// OpenWindow creates a window with no ambient privileges.
func (rt *Runtime) OpenWindow(_ context.Context, id domain.WindowID, origin domain.Origin) (*domain.Window, error) {
	return rt.openWindow.Execute(id, origin)
}

// NavigateWindow changes a window origin without policy checks (authority does not follow).
func (rt *Runtime) NavigateWindow(_ context.Context, id domain.WindowID, origin domain.Origin) error {
	return rt.navigateWindow.Execute(id, origin)
}

// NavigateWindowGuarded evaluates navigation policy before navigating.
func (rt *Runtime) NavigateWindowGuarded(_ context.Context, id domain.WindowID, origin domain.Origin) error {
	return rt.navigatePolicy.Execute(id, origin)
}

// CloseWindow closes a window and releases owned resources and subscriptions.
func (rt *Runtime) CloseWindow(_ context.Context, id domain.WindowID) error {
	return rt.closeWindow.Execute(id)
}

// SubscribeEvent registers a window-owned event subscription.
func (rt *Runtime) SubscribeEvent(id domain.SubscriptionID, event domain.EventName, window domain.WindowID) (*domain.Subscription, error) {
	return rt.subscribe.Execute(id, event, window)
}

// EmitEvent resolves open window subscribers for a named event.
// Hosts (e.g. app.App.Emit) deliver payloads to those windows via PostMessage.
func (rt *Runtime) EmitEvent(event domain.EventName, payload any) ([]application.EventDelivery, error) {
	return rt.emit.Execute(event, payload)
}

// NavigationPolicy returns the runtime navigation policy.
func (rt *Runtime) NavigationPolicy() *domain.NavigationPolicy { return rt.navPolicy }

// Invoke runs a frontend command through the capability gateway.
func (rt *Runtime) Invoke(ctx context.Context, req domain.InvocationRequest) (*domain.InvocationResult, error) {
	res, err := rt.invoke.Execute(ctx, req)
	outcome := "allowed"
	detail := ""
	if err != nil {
		var denied *domain.ErrDenied
		if errors.As(err, &denied) {
			outcome = "denied"
			detail = denied.Reason
		} else {
			outcome = "error"
			detail = err.Error()
		}
	}
	rt.emitAudit(audit.Event{
		Kind:    audit.KindCommandInvoke,
		Window:  string(req.Caller.Window),
		Origin:  string(req.Caller.Origin),
		Action:  string(req.Command),
		Outcome: outcome,
		Detail:  detail,
	})
	return res, err
}

// Authorize evaluates a permission against registered grants.
// Implements desktop.Gateway so host chrome can share the kernel gateway.
// When an enterprise policy is installed, it may tighten an allow into a deny.
func (rt *Runtime) Authorize(caller domain.Caller, permission domain.PermissionName, resourcePath string) domain.Decision {
	grants, err := rt.grants.List()
	if err != nil {
		d := domain.Decision{
			Permission: permission,
			Code:       domain.DenialNoGrant,
			Reason:     err.Error(),
		}
		rt.emitAudit(audit.Event{
			Kind: audit.KindCapabilityDecision, Window: string(caller.Window), Origin: string(caller.Origin),
			Action: string(permission), Outcome: "denied", Detail: d.Reason,
		})
		return d
	}
	d := domain.NewCapabilityGateway(grants...).Authorize(caller, permission, resourcePath)
	if rt.policyEng != nil {
		before := d
		d = rt.policyEng.OverlayDecision(permission, d)
		if before.Allowed && !d.Allowed {
			rt.emitAudit(audit.Event{
				Kind: audit.KindPolicyOverride, Window: string(caller.Window), Origin: string(caller.Origin),
				Action: string(permission), Outcome: "denied", Detail: d.Reason,
			})
		}
	}
	outcome := "denied"
	if d.Allowed {
		outcome = "allowed"
	}
	rt.emitAudit(audit.Event{
		Kind: audit.KindCapabilityDecision, Window: string(caller.Window), Origin: string(caller.Origin),
		Action: string(permission), Outcome: outcome, Detail: d.Reason,
	})
	return d
}

// ApplyUpdate verifies a signed update (and optional enterprise policy), then
// atomically installs the artifact at destPath (invariant 9).
func (rt *Runtime) ApplyUpdate(m updater.Manifest, pub ed25519.PublicKey, artifact []byte, destPath string) (updater.InstallPlan, error) {
	if rt.policyEng != nil {
		if err := rt.policyEng.AuthorizeUpdate(m); err != nil {
			rt.emitAudit(audit.Event{
				Kind: audit.KindUpdatePlan, Actor: m.AppID, Action: string(m.Channel),
				Outcome: "denied", Detail: err.Error(),
				Metadata: map[string]any{"version": m.Version},
			})
			return updater.InstallPlan{}, err
		}
	}
	plan, err := updater.PlanInstall(m, pub, artifact)
	if err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindUpdatePlan, Actor: m.AppID, Action: string(m.Channel),
			Outcome: "denied", Detail: err.Error(),
			Metadata: map[string]any{"version": m.Version},
		})
		return updater.InstallPlan{}, err
	}
	if err := updater.ApplyInstall(plan, artifact, destPath); err != nil {
		rt.emitAudit(audit.Event{
			Kind: audit.KindUpdatePlan, Actor: plan.AppID, Action: string(plan.Channel),
			Outcome: "error", Detail: err.Error(),
			Metadata: map[string]any{"version": plan.Version},
		})
		return updater.InstallPlan{}, err
	}
	rt.emitAudit(audit.Event{
		Kind: audit.KindUpdatePlan, Actor: plan.AppID, Action: string(plan.Channel),
		Outcome: "allowed", Detail: destPath,
		Metadata: map[string]any{"version": plan.Version, "sha256": plan.SHA256},
	})
	return plan, nil
}

// InspectCapabilities returns the effective privileged surface for a window.
func (rt *Runtime) InspectCapabilities(windowID domain.WindowID) (domain.EffectiveSurface, error) {
	return rt.inspect.Execute(windowID)
}

// CallerFor returns the live caller identity for a window (native boundary).
func (rt *Runtime) CallerFor(windowID domain.WindowID) (domain.Caller, error) {
	win, err := rt.windows.Get(windowID)
	if err != nil {
		return domain.Caller{}, err
	}
	return win.Caller()
}

// FormatInspect renders a human-readable capability inspection report.
func FormatInspect(appID domain.AppID, surfaces ...domain.EffectiveSurface) string {
	return FormatInspectFull(appID, nil, surfaces...)
}

// FormatInspectFull renders capability and optional plugin ownership surfaces.
func FormatInspectFull(appID domain.AppID, plugins []plugin.PermissionOwnership, surfaces ...domain.EffectiveSurface) string {
	out := fmt.Sprintf("Application: %s\nKernel: %s\n", appID, Version)
	out += "\nWindows\n"
	for _, s := range surfaces {
		out += fmt.Sprintf("  %s\n    origin: %s\n    capabilities:\n", s.Window, s.Origin)
		if len(s.GrantNames) == 0 {
			out += "      (none)\n"
			continue
		}
		for _, g := range s.GrantNames {
			out += fmt.Sprintf("      %s\n", g)
		}
	}
	out += "\nEffective privileged surface\n"
	anyPerm := false
	for _, s := range surfaces {
		for _, p := range s.Permissions {
			anyPerm = true
			line := fmt.Sprintf("  %s", p.Name)
			if len(p.PathAllow) > 0 {
				line += fmt.Sprintf("    %v", p.PathAllow)
			}
			out += line + "\n"
		}
	}
	if !anyPerm {
		out += "  (none)\n"
	}
	if len(plugins) > 0 {
		out += "\nPlugins (permission ownership)\n"
		for _, o := range plugins {
			out += fmt.Sprintf("  %s  owned by %s\n", o.Permission, o.Plugin)
		}
	}
	return out
}
