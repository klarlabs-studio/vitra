// Package vitra is the entry point for the Vitra runtime kernel — a secure,
// capability-oriented desktop application runtime for Go + web frontends.
//
// Phase 2 adds desktop completeness on top of the secure kernel: multi-window
// lifecycle, navigation policy, window-owned subscriptions, and capability-
// gated desktop services (menu/tray/dialog/clipboard/shortcuts/deeplinks).
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
	"fmt"

	"go.klarlabs.de/vitra/application"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/inmemory"
)

// Version is the kernel API version. Generated frontend bindings should be
// tied to this version (reliability invariant 8).
const Version = "0.2.0"

// Config configures a Runtime.
type Config struct {
	AppID             domain.AppID
	TrustedOrigins    []domain.Origin
	AllowExternalNav  bool
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

	registerGrant   *application.RegisterGrantUseCase
	registerCommand *application.RegisterCommandUseCase
	openWindow      *application.OpenWindowUseCase
	navigateWindow  *application.NavigateWindowUseCase
	navigatePolicy  *application.NavigateWithPolicyUseCase
	closeWindow     *application.CloseWindowUseCase
	invoke          *application.InvokeCommandUseCase
	inspect         *application.InspectCapabilitiesUseCase
	subscribe       *application.SubscribeEventUseCase
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
	rt := &Runtime{
		appID:         cfg.AppID,
		grants:        inmemory.NewGrantRepo(),
		windows:       inmemory.NewWindowRepo(),
		commands:      inmemory.NewCommandRepo(),
		resources:     inmemory.NewResourceRepo(),
		subscriptions: inmemory.NewSubscriptionRepo(),
		executors:     inmemory.NewExecutorRegistry(),
		navPolicy:     policy,
	}
	rt.wire()
	return rt, nil
}

func (rt *Runtime) wire() {
	invoker := &domain.InvocationService{
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
	rt.invoke = &application.InvokeCommandUseCase{Invoker: invoker}
	rt.inspect = &application.InspectCapabilitiesUseCase{Grants: rt.grants, Windows: rt.windows}
	rt.subscribe = &application.SubscribeEventUseCase{Windows: rt.windows, Subscriptions: rt.subscriptions}
}

// AppID returns the application id.
func (rt *Runtime) AppID() domain.AppID { return rt.appID }

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

// NavigationPolicy returns the runtime navigation policy.
func (rt *Runtime) NavigationPolicy() *domain.NavigationPolicy { return rt.navPolicy }

// Invoke runs a frontend command through the capability gateway.
func (rt *Runtime) Invoke(ctx context.Context, req domain.InvocationRequest) (*domain.InvocationResult, error) {
	return rt.invoke.Execute(ctx, req)
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
	return out
}
