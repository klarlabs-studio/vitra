package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.klarlabs.de/vitra"
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

func scaffoldNew(args []string) error {
	usage := fmt.Errorf("usage: vitra new <dir> [--template %s]", templateNames())
	dir := ""
	name := "vanilla"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--template":
			i++
			if i >= len(args) {
				return fmt.Errorf("--template requires one of %s", templateNames())
			}
			name = args[i]
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
	tsClient, err := scaffoldTypeScriptClient()
	if err != nil {
		return err
	}
	files, err := scaffoldFiles(tmpl, modPath, tsClient)
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
	fmt.Printf("created %s (template=%s)\n", dir, tmpl.name)
	if tmpl.vite {
		fmt.Println("next: cd", dir, "&& (optional: cd frontend && npm install && npm run build) && vitra dev")
	} else {
		fmt.Println("next: cd", dir, "&& vitra dev")
	}
	return nil
}

// scaffoldFiles returns the starter's files keyed by slash-separated path.
func scaffoldFiles(tmpl scaffoldTemplate, modPath, tsClient string) (map[string]string, error) {
	mainTmpl, err := fs.ReadFile(templateFS, "templates/main.go.tmpl")
	if err != nil {
		return nil, err
	}
	starter, err := fs.ReadFile(templateFS, "templates/starter.html")
	if err != nil {
		return nil, err
	}
	embedPattern, root := "frontend/*", "frontend"
	if tmpl.vite {
		embedPattern, root = "all:frontend/dist", "frontend/dist"
	}
	files := map[string]string{
		"go.mod":                   scaffoldGoMod(modPath),
		"main.go":                  strings.NewReplacer("__VITRA_EMBED__", embedPattern, "__VITRA_ROOT__", root).Replace(string(mainTmpl)),
		"frontend/vitra-client.ts": tsClient,
		"README.md":                scaffoldREADME(tmpl),
	}
	if !tmpl.vite {
		files["frontend/index.html"] = string(starter)
		return files, nil
	}
	// A starter dist so `vitra dev` works before the first npm run build.
	files["frontend/dist/index.html"] = string(starter)
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
	return files, err
}

func scaffoldREADME(tmpl scaffoldTemplate) string {
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
grant. For files, dialogs, the clipboard, menus, and more, register the
official plugins (` + "`go.klarlabs.de/vitra/plugin/official`" + `), bind their
services, and grant their permissions narrowly: path-scoped permissions such
as ` + "`fs.read`" + ` take allow and deny patterns. See ` + "`example/notes`" + ` and
` + "`example/competitive`" + ` in the Vitra repository, and
[the security model](https://github.com/klarlabs-studio/vitra/blob/main/docs/security.md).
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
