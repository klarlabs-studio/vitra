package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
)

const (
	// mainWindow is the only window, and the only one the grant names.
	mainWindow domain.WindowID = "main"
	// auditEvent carries each new audit.Event to subscribed windows.
	auditEvent domain.EventName = "audit.event"
)

// VaultInfo tells the frontend where the vault is, so it can name paths.
type VaultInfo struct {
	Root string `json:"root"`
}

// Entry is a note or folder in the vault.
type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
}

// ListRequest lists the entries of one vault folder.
type ListRequest struct {
	Dir string `json:"dir"`
}

// ResourcePath binds the folder to the path the gateway authorized.
func (r ListRequest) ResourcePath() string { return r.Dir }

// ReadRequest reads one note.
type ReadRequest struct {
	Path string `json:"path"`
}

// ResourcePath binds the note to the path the gateway authorized.
func (r ReadRequest) ResourcePath() string { return r.Path }

// WriteRequest saves one note.
type WriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ResourcePath binds the note to the path the gateway authorized.
func (r WriteRequest) ResourcePath() string { return r.Path }

// maxNoteBytes bounds a saved note; the IPC bridge caps messages at 1 MiB.
const maxNoteBytes = 512 << 10

// grantVault gives the main window, at the packaged origin only:
//   - fs.read on everything in the vault except .private/,
//   - fs.write on Markdown files only, so the page cannot drop a script,
//   - vault.info, which returns the vault's location,
//   - audit.read, which streams the audit log to the window.
//
// root must be absolute with symlinks resolved, because path scopes compare
// strings.
func grantVault(rt *vitra.Runtime, root string) error {
	scope := func(allow string) *domain.PathScope {
		return &domain.PathScope{
			Allow: []string{allow},
			Deny:  []string{root + "/.private/**"},
		}
	}
	grant, err := domain.NewCapabilityGrant(
		"notes-vault", "read the vault, write Markdown notes",
		[]domain.WindowID{mainWindow},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "vault.info"},
			{Name: "audit.read"},
			{Name: desktop.PermFSRead, PathScope: scope(root + "/**")},
			{Name: desktop.PermFSWrite, PathScope: scope(root + "/**/*.md")},
		},
	)
	if err != nil {
		return err
	}
	return rt.RegisterGrant(grant)
}

// registerCommands registers the commands the frontend may call. Each one
// names the permission the gateway checks before the handler runs; file
// access also goes through desktop.FileService, which resolves symlinks and
// authorizes the real target.
func registerCommands(rt *vitra.Runtime, root string, log *liveAudit) error {
	files := &desktop.FileService{Gateway: rt}
	return errors.Join(
		vitra.Register(rt, vitra.Command[struct{}, VaultInfo]{
			Name:        "vault.info",
			Description: "Where the vault is",
			Permission:  "vault.info",
			Handler: func(context.Context, domain.Invocation, struct{}) (VaultInfo, error) {
				return VaultInfo{Root: root}, nil
			},
		}),
		vitra.Register(rt, vitra.Command[struct{}, []audit.Event]{
			Name:        "audit.follow",
			Description: "Stream audit events to the calling window; returns the log so far",
			Permission:  "audit.read",
			Handler: func(_ context.Context, inv domain.Invocation, _ struct{}) ([]audit.Event, error) {
				// Subscribing from the page, rather than when the app starts,
				// guarantees the window exists. Re-subscribing is a no-op.
				id := domain.SubscriptionID("audit-" + string(inv.Caller.Window))
				if _, err := rt.SubscribeEvent(id, auditEvent, inv.Caller.Window); err != nil {
					return nil, err
				}
				return log.List(), nil
			},
		}),
		vitra.Register(rt, vitra.Command[ListRequest, []Entry]{
			Name:        "notes.list",
			Description: "List a vault folder",
			Permission:  desktop.PermFSRead,
			Handler: func(_ context.Context, inv domain.Invocation, req ListRequest) ([]Entry, error) {
				return listDir(rt, inv, req.Dir)
			},
		}),
		vitra.Register(rt, vitra.Command[ReadRequest, string]{
			Name:        "notes.read",
			Description: "Read a note",
			Permission:  desktop.PermFSRead,
			Handler: func(ctx context.Context, inv domain.Invocation, req ReadRequest) (string, error) {
				b, err := files.Read(ctx, inv.Caller, req.Path)
				return string(b), err
			},
		}),
		vitra.Register(rt, vitra.Command[WriteRequest, struct{}]{
			Name:        "notes.write",
			Description: "Save a Markdown note",
			Permission:  desktop.PermFSWrite,
			Handler: func(ctx context.Context, inv domain.Invocation, req WriteRequest) (struct{}, error) {
				if len(req.Content) > maxNoteBytes {
					return struct{}{}, errors.New("note is too large")
				}
				return struct{}{}, files.Write(ctx, inv.Caller, req.Path, []byte(req.Content))
			},
		}),
	)
}

// listDir lists dir after authorizing its real location, so a symlinked
// folder cannot list something outside the vault.
func listDir(rt *vitra.Runtime, inv domain.Invocation, dir string) ([]Entry, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	if d := rt.Authorize(inv.Caller, desktop.PermFSRead, filepath.ToSlash(real)); !d.Allowed {
		return nil, &domain.ErrDenied{Permission: desktop.PermFSRead, Window: inv.Caller.Window, Origin: inv.Caller.Origin, Code: d.Code, Reason: d.Reason}
	}
	des, err := os.ReadDir(real)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if !de.IsDir() && !strings.HasSuffix(name, ".md") {
			continue
		}
		entries = append(entries, Entry{Name: name, Path: dir + "/" + name, Dir: de.IsDir()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}
