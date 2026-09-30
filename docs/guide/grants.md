# Grants and path scopes

A grant is the only way a window gets authority.

```go
grant, err := domain.NewCapabilityGrant(
    "editor",                        // name, in audits and inspect
    "the editor window edits notes", // description
    []domain.WindowID{"main"},       // windows
    []domain.Origin{domain.OriginPackagedLocal}, // origins
    []domain.PermissionSpec{
        {Name: "notes.list"},
        {Name: "fs.read", PathScope: &domain.PathScope{
            Allow: []string{"/Users/me/Notes/**"},
            Deny:  []string{"/Users/me/Notes/.private/**"},
        }},
    },
)
if err != nil {
    return err
}
if err := rt.RegisterGrant(grant); err != nil {
    return err
}
```

A call is allowed only if some grant names the calling **window**, its current **origin**, and the command's **permission**. For a path-scoped permission, the `resourcePath` of the call must also match the scope.

## Origins

Windows opened by `app.App` show your embedded assets from a local server and are recorded with the origin `domain.OriginPackagedLocal` (`app://local`). Grant that origin for your own UI. Navigation away from the asset server is blocked, and a window at another origin would not match these grants anyway.

## One grant per role

Give each window only what it needs, and prefer several small grants to one big one:

```go
// The settings window can read preferences, nothing else.
settings, _ := domain.NewCapabilityGrant("settings", "read preferences",
    []domain.WindowID{"settings"}, []domain.Origin{domain.OriginPackagedLocal},
    []domain.PermissionSpec{{Name: "prefs.read"}})
```

A window you open later (`app.OpenWindow`) has no authority until a grant names its ID.

## Path scopes

Permissions that act on files (`fs.read`, `fs.write`, `path.open`, and your own commands that use them) take a `PathScope`:

- **Allow** patterns: the path must match at least one.
- **Deny** patterns: the path must match none. **Deny always wins.**

### Pattern syntax

| Pattern | Matches |
|---|---|
| `/data/**` | everything under `/data`, at any depth, and `/data` itself |
| `/data/*` | direct children of `/data` |
| `/data/**/*.md` | Markdown files anywhere under `/data` |
| `/data/report-??.csv` | `path.Match` globs within one segment |
| `C:/Users/me/Notes/**` | Windows paths, with `/` or `\` |

Patterns must be absolute. `NewCapabilityGrant` rejects invalid ones.

### What is rejected before matching

Paths containing `..` segments, NUL bytes, relative or drive-relative paths, UNC paths (`\\host\share`), and device paths (`\\?\`) are refused outright.

### Case

Deny patterns ignore letter case, so a `.private` deny also covers `.PRIVATE` on case-insensitive filesystems (APFS, NTFS). Allow patterns are case-sensitive. Either way, an ambiguous path fails closed.

### Symlinks

Scopes compare strings, so a symlink inside an allowed folder could point anywhere. The desktop `FileService` and `PathService` resolve the real target first. It must stay under the real root of the allow pattern that matched, and it is checked against the deny patterns again. If your own commands touch the filesystem, go through `desktop.FileService`, or resolve and re-authorize the real path with `rt.Authorize`.

### Write scopes with the real path

Scopes are strings, and a directory can have several names: on macOS `/var` is `/private/var`, and `$TMPDIR` lives there. Resolve the directory before building the grant:

```go
root, err := filepath.EvalSymlinks(dir)
root = filepath.ToSlash(root)
scope := &domain.PathScope{Allow: []string{root + "/**"}}
```

### Never grant `fs.write` and `path.open` on the same folder

`path.open` opens a file with its default application, which runs scripts and executables. Together with `fs.write`, a page could write a program and launch it. If you need both, restrict writes to harmless types (`/**/*.md`).

## Inspecting what a window can do

```go
surface, _ := rt.InspectCapabilities("main")
fmt.Println(vitra.FormatInspect(rt.AppID(), surface))
```

`vitra inspect capabilities` prints the same report for a demo runtime.

## Enterprise policy

A `policy.Engine` can deny permissions for a whole deployment, even when the app grants them. Policy only ever tightens. See [audit and policy](/guide/audit-policy).
