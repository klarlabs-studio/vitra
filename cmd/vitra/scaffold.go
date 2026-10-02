package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official"
)

// templateFS holds the starter app. main.go.tmpl is the shared Go program;
// starter.html is the page served before any frontend build; each
// <template>/ directory holds that template's static frontend files.
//
//go:embed all:templates
var templateFS embed.FS

// scaffoldTemplate describes one `vitra new --template` choice.
type scaffoldTemplate struct {
	name  string
	label string
	// vite templates embed frontend/dist and ship sources to build it.
	vite bool
}

// scaffoldTemplates are the supported starters. Other frameworks work the
// same way as the vite template: build to frontend/dist and call
// window.vitra.invoke (or the generated client) from any framework.
var scaffoldTemplates = []scaffoldTemplate{
	{name: "vanilla", label: "HTML"},
	{name: "vite", label: "Vite + TypeScript", vite: true},
	{name: "react", label: "Vite + React", vite: true},
	{name: "svelte", label: "Vite + Svelte", vite: true},
	{name: "vue", label: "Vite + Vue", vite: true},
}

func templateNames() string {
	names := make([]string, len(scaffoldTemplates))
	for i, t := range scaffoldTemplates {
		names[i] = t.name
	}
	return strings.Join(names, "|")
}

func lookupTemplate(name string) (scaffoldTemplate, error) {
	for _, t := range scaffoldTemplates {
		if t.name == name {
			return t, nil
		}
	}
	return scaffoldTemplate{}, fmt.Errorf("unknown template %q (want %s); for other frameworks start from vite and build to frontend/dist", name, templateNames())
}

// scaffoldPlugin is one `vitra new --with` choice: an official plugin the
// starter registers and grants as narrowly as a starter can (see
// templates/main.go.tmpl for the grants).
type scaffoldPlugin struct {
	name string
	ctor string // Go expression that constructs the plugin
	new  func() plugin.Plugin
}

// scaffoldPlugins are the plugins `vitra new --with` supports, in the order
// they are registered. They are the ones with a grant that is narrow and
// useful without app-specific choices. The others (path, browser, window,
// menu, tray, shortcut, dragdrop, app, deeplink) need scopes, ids, or
// accelerators only the app knows, so they are added by hand.
var scaffoldPlugins = []scaffoldPlugin{
	{name: "fs", ctor: "official.FS()", new: official.FS},
	{name: "dialog", ctor: "official.Dialog()", new: official.Dialog},
	{name: "clipboard", ctor: "official.Clipboard()", new: official.Clipboard},
	{name: "notification", ctor: "official.Notification()", new: official.Notification},
	{name: "os", ctor: "official.OS()", new: official.OS},
}

func pluginNames() string {
	names := make([]string, len(scaffoldPlugins))
	for i, p := range scaffoldPlugins {
		names[i] = p.name
	}
	return strings.Join(names, ", ")
}

// parseWith parses a comma-separated --with list into supported plugins in
// their canonical order, without duplicates.
func parseWith(list string) ([]scaffoldPlugin, error) {
	chosen := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !slices.ContainsFunc(scaffoldPlugins, func(p scaffoldPlugin) bool { return p.name == name }) {
			return nil, fmt.Errorf("unknown plugin %q for --with (want a comma-separated list of %s)", name, pluginNames())
		}
		chosen[name] = true
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("--with requires a comma-separated list of %s", pluginNames())
	}
	var out []scaffoldPlugin
	for _, p := range scaffoldPlugins {
		if chosen[p.name] {
			out = append(out, p)
		}
	}
	return out, nil
}

func scaffoldNew(args []string) error {
	usage := fmt.Errorf("usage: vitra new <dir> [--template %s] [--with %s]", templateNames(), strings.ReplaceAll(pluginNames(), ", ", ","))
	dir := ""
	name := "vanilla"
	var plugins []scaffoldPlugin
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--template":
			i++
			if i >= len(args) {
				return fmt.Errorf("--template requires one of %s", templateNames())
			}
			name = args[i]
		case "--with":
			i++
			if i >= len(args) {
				return fmt.Errorf("--with requires a comma-separated list of %s", pluginNames())
			}
			var err error
			if plugins, err = parseWith(args[i]); err != nil {
				return err
			}
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown new flag %q", args[i])
			}
			if dir != "" {
				return usage
			}
			dir = args[i]
		}
	}
	if dir == "" {
		return usage
	}
	tmpl, err := lookupTemplate(name)
	if err != nil {
		return err
	}
	modPath := "example.com/" + filepath.Base(dir)
	if modPath == "example.com/." {
		modPath = "example.com/vitra-app"
	}
	tsClient, err := scaffoldTypeScriptClient(plugins)
	if err != nil {
		return err
	}
	files, err := scaffoldFiles(tmpl, modPath, tsClient, plugins)
	if err != nil {
		return err
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	if len(plugins) == 0 {
		fmt.Printf("created %s (template=%s)\n", dir, tmpl.name)
	} else {
		fmt.Printf("created %s (template=%s, with=%s)\n", dir, tmpl.name, pluginList(plugins, ","))
	}
	if tmpl.vite {
		fmt.Println("next: cd", dir, "&& (optional: cd frontend && npm install && npm run build) && vitra dev")
	} else {
		fmt.Println("next: cd", dir, "&& vitra dev")
	}
	return nil
}

// scaffoldData is what the main.go, starter.html, and plugins.ts templates
// are rendered with.
type scaffoldData struct {
	Embed, Root string // embed pattern and asset root of the frontend

	Plugins     bool   // any --with plugin chosen
	PluginList  string // "fs, dialog"
	PluginNames string // "fs,dialog"
	PluginCalls string // "official.FS(), official.Dialog()"

	FS, Dialog, Clipboard, Notification, OS bool
}

func pluginList(plugins []scaffoldPlugin, sep string) string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.name
	}
	return strings.Join(names, sep)
}

func newScaffoldData(tmpl scaffoldTemplate, plugins []scaffoldPlugin) scaffoldData {
	d := scaffoldData{Embed: "frontend/*", Root: "frontend"}
	if tmpl.vite {
		d.Embed, d.Root = "all:frontend/dist", "frontend/dist"
	}
	calls := make([]string, len(plugins))
	for i, p := range plugins {
		calls[i] = p.ctor
		switch p.name {
		case "fs":
			d.FS = true
		case "dialog":
			d.Dialog = true
		case "clipboard":
			d.Clipboard = true
		case "notification":
			d.Notification = true
		case "os":
			d.OS = true
		}
	}
	d.Plugins = len(plugins) > 0
	d.PluginList = pluginList(plugins, ", ")
	d.PluginNames = pluginList(plugins, ",")
	d.PluginCalls = strings.Join(calls, ", ")
	return d
}

// renderTemplate renders an embedded template. Templates use [[ ]] as
// delimiters, because Go composite literals are full of {{ and }}.
func renderTemplate(name string, data scaffoldData) (string, error) {
	src, err := fs.ReadFile(templateFS, "templates/"+name)
	if err != nil {
		return "", err
	}
	t, err := template.New(name).Delims("[[", "]]").Option("missingkey=error").Parse(string(src))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// scaffoldFiles returns the starter's files keyed by slash-separated path.
// Without plugins the output is the plain greet starter.
func scaffoldFiles(tmpl scaffoldTemplate, modPath, tsClient string, plugins []scaffoldPlugin) (map[string]string, error) {
	data := newScaffoldData(tmpl, plugins)
	mainGo, err := renderTemplate("main.go.tmpl", data)
	if err != nil {
		return nil, err
	}
	starter, err := renderTemplate("starter.html", data)
	if err != nil {
		return nil, err
	}
	files := map[string]string{
		"go.mod":                   scaffoldGoMod(modPath),
		"main.go":                  mainGo,
		"frontend/vitra-client.ts": tsClient,
		"README.md":                scaffoldREADME(tmpl, plugins),
	}
	if !tmpl.vite {
		files["frontend/index.html"] = starter
		return files, nil
	}
	// A starter dist so `vitra dev` works before the first npm run build.
	files["frontend/dist/index.html"] = starter
	files[".gitignore"] = "frontend/node_modules/\nvitra-app\n"
	base := "templates/" + tmpl.name
	err = fs.WalkDir(templateFS, base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(templateFS, p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, base+"/")
		files[path.Clean(rel)] = string(body)
		return nil
	})
	if err != nil || !data.Plugins {
		return files, err
	}
	// Plugin demos are framework-agnostic DOM code, loaded by the entry.
	demos, err := renderTemplate("plugins.ts.tmpl", data)
	if err != nil {
		return nil, err
	}
	files["frontend/src/plugins.ts"] = demos
	entry := "frontend/src/main.ts"
	if _, ok := files[entry]; !ok {
		entry = "frontend/src/main.tsx"
	}
	const styleImport = "import \"./style.css\";\n"
	if !strings.HasPrefix(files[entry], styleImport) {
		return nil, fmt.Errorf("template %s: %s must start with %q", tmpl.name, entry, styleImport)
	}
	files[entry] = styleImport + "import \"./plugins\";\n" + strings.TrimPrefix(files[entry], styleImport)
	return files, nil
}

func scaffoldREADME(tmpl scaffoldTemplate, plugins []scaffoldPlugin) string {
	body := scaffoldBaseREADME(tmpl)
	if len(plugins) == 0 {
		return body
	}
	return body + `
## Plugins

This app was created with ` + "`--with " + pluginList(plugins, ",") + "`" + `. ` + "`main.go`" + ` registers
only those official plugins and grants each as narrowly as a starter can.
The comments next to the grant explain each permission and what to add, or
take away, when your app needs something else.
`
}

func scaffoldBaseREADME(tmpl scaffoldTemplate) string {
	body := `# Vitra app

` + "```bash" + `
# Native DesktopHost (Linux WebKitGTK / Darwin WKWebView / Windows WebView2)
vitra dev
# or
CGO_ENABLED=1 go run -tags vitra_native .

# Refresh the typed frontend client after changing commands/plugins
vitra generate typescript --app . --out frontend/vitra-client.ts

# Stage a package (optional)
vitra package --out dist/ --format dir
` + "```" + `

## Adding capabilities

The frontend can call only what ` + "`main.go`" + ` registers and grants. To add a
command, register it with ` + "`vitra.Register`" + ` and add its permission to the
grant. For files, dialogs, the clipboard, menus, and more, call
` + "`a.UseOfficialPlugins(ctx)`" + ` after ` + "`app.New`" + ` and grant the permissions
you need narrowly: path-scoped permissions such as ` + "`fs.read`" + ` take allow and
deny patterns. See the [desktop features guide](https://klarlabs-studio.github.io/vitra/guide/desktop)
and [the security model](https://klarlabs-studio.github.io/vitra/security).
`
	if !tmpl.vite {
		return body
	}
	return body + `
## ` + tmpl.label + ` frontend

Go embeds ` + "`frontend/dist`" + `. A starter ` + "`dist/index.html`" + ` is included so
` + "`vitra dev`" + ` works immediately. To rebuild from the sources:

` + "```bash" + `
cd frontend
npm install
npm run build
` + "```" + `

Then re-run ` + "`vitra generate typescript --app . --out frontend/vitra-client.ts`" + ` and
` + "`npm run build`" + ` after changing plugin commands.
`
}

// scaffoldGoVersion is the go directive of go.klarlabs.de/vitra's go.mod; a
// generated app may not declare an older one.
const scaffoldGoVersion = "1.26.2"

func scaffoldGoMod(modPath string) string {
	body := "module " + modPath + "\n\ngo " + scaffoldGoVersion + "\n\nrequire go.klarlabs.de/vitra v" + vitra.Version + "\n"
	if root := os.Getenv("VITRA_MODULE_PATH"); root != "" {
		body += "\nreplace go.klarlabs.de/vitra => " + root + "\n"
	}
	return body
}
