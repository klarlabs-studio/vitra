package bindings_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bindings"
)

func mustGenerate(t *testing.T, cmds []bindings.Command, events []domain.EventName) string {
	t.Helper()
	out, err := bindings.GenerateTypeScript("app", "0.3.0", cmds, events)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGenerateTypeScript(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("project.open", "Open a project", "fs.read")
	out := mustGenerate(t, bindings.Untyped(cmd), nil)
	for _, want := range []string{
		"kernel: 0.3.0",
		"projectOpen(input?: unknown, resourcePath?: string): Promise<unknown>;",
		"fs.read",
		`invoke("project.open"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "VitraEvents") {
		t.Fatal("expected no events section without events")
	}
}

func TestGenerateTypeScript_Events(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("fs.read", "Read a file", "fs.read")
	out := mustGenerate(t, bindings.Untyped(cmd), []domain.EventName{"fs.changed", "demo.tick", "fs.changed"})
	for _, want := range []string{
		"VitraEvents", "createEvents", "onFsChanged", "onDemoTick",
		`on("fs.changed"`, `on("demo.tick"`, "VitraSubscriber",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Count(out, `on("fs.changed"`) != 1 {
		t.Fatalf("expected deduped fs.changed:\n%s", out)
	}
}

type address struct {
	City string `json:"city"`
}

type Base struct {
	ID string `json:"id"`
}

type node struct {
	Base                     // embedded: fields flatten
	Label    string          `json:"label"`
	Count    int             `json:"count,omitempty"`
	Ratio    float64         `json:"ratio"`
	OK       bool            `json:"ok"`
	Tags     []string        `json:"tags"`
	Meta     map[string]int  `json:"meta"`
	Home     *address        `json:"home"`
	Work     address         `json:"work"`
	Children []*node         `json:"children,omitempty"` // recursive
	When     time.Time       `json:"when"`
	Blob     []byte          `json:"blob"`
	Raw      json.RawMessage `json:"raw"`
	Any      any             `json:"any"`
	Big      int64           `json:"big,string"`
	Hidden   string          `json:"-"`
	unexport string          //nolint:unused // must not appear
	NoTag    string
	Inline   struct{ X int } `json:"inline"`
	Opt      *string         `json:"opt,omitempty"`
	Arr      [2]uint8        `json:"arr"`
}

func TestGenerateTypeScript_TypedShapes(t *testing.T) {
	out := mustGenerate(t, []bindings.Command{{
		Name: "graph.get", Description: "Get a node", Permission: "graph.read",
		Input: reflect.TypeOf(address{}), Output: reflect.TypeOf(&node{}),
	}}, nil)
	for _, want := range []string{
		"export interface Address {\n  city: string;\n}",
		"export interface Node {",
		"  id: string;",
		"  label: string;",
		"  count?: number;",
		"  ratio: number;",
		"  ok: boolean;",
		"  tags: string[] | null;",
		"  meta: Record<string, number> | null;",
		"  home: Address | null;",
		"  work: Address;",
		"  children?: (Node | null)[] | null;",
		"  when: string;",
		"  blob: string | null;",
		"  raw: unknown;",
		"  any: unknown;",
		"  big: string;",
		"  NoTag: string;",
		"  inline: {\n    X: number;\n  };",
		"  opt?: string | null;",
		"  arr: number[];",
		"graphGet(input: Address, resourcePath?: string): Promise<Node | null>;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, unwanted := range []string{"Hidden", "unexport", "Base"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("unexpected %q", unwanted)
		}
	}
	if t.Failed() {
		t.Log(out)
	}
	// Deterministic output.
	again := mustGenerate(t, []bindings.Command{{
		Name: "graph.get", Description: "Get a node", Permission: "graph.read",
		Input: reflect.TypeOf(address{}), Output: reflect.TypeOf(&node{}),
	}}, nil)
	if again != out {
		t.Fatal("output is not deterministic")
	}
}

type pickOptions struct {
	Title   string   `json:"title,omitempty"`
	Filters []string `json:"filters,omitzero"`
	_       int      // unexported fields are not encoded, so they don't count
}

// An input whose fields are all optional may be left out: null decodes to
// its zero value.
func TestGenerateTypeScript_OptionalInputWhenEveryFieldIsOptional(t *testing.T) {
	out := mustGenerate(t, []bindings.Command{
		{Name: "pick", Permission: "p", Input: reflect.TypeOf(pickOptions{}), Output: reflect.TypeOf("")},
		{Name: "graph.get", Permission: "p", Input: reflect.TypeOf(address{}), Output: reflect.TypeOf("")},
	}, nil)
	for _, want := range []string{
		"pick(input?: PickOptions, resourcePath?: string): Promise<string>;",
		"graphGet(input: Address, resourcePath?: string): Promise<string>;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGenerateTypeScript_VoidOutput(t *testing.T) {
	out := mustGenerate(t, []bindings.Command{{
		Name: "app.quit", Permission: "app.quit", Input: reflect.TypeOf(struct{}{}), Void: true,
	}}, nil)
	for _, want := range []string{
		"appQuit(input?: {}, resourcePath?: string): Promise<void>;",
		`appQuit: (input, resourcePath) => invoke("app.quit", input, resourcePath) as Promise<void>,`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGenerateTypeScript_RejectsMethodNameCollisions(t *testing.T) {
	a, _ := domain.NewCommandDefinition("fs.read", "a", "fs.read")
	b, _ := domain.NewCommandDefinition("fs_read", "b", "fs.read")
	if _, err := bindings.GenerateTypeScript("app", "0.3.0", bindings.Untyped(a, b), nil); err == nil {
		t.Fatal("fs.read and fs_read both became fsRead without an error")
	}
}

// Descriptions and names come from app code, but generated output must stay
// valid TypeScript whatever they contain.
func TestGenerateTypeScript_EscapesUntrustedText(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("x.run", "ends */ the comment\nand   more", "p")
	out := mustGenerate(t, bindings.Untyped(cmd), nil)
	if strings.Contains(out, "ends */") {
		t.Fatalf("description closed the doc comment:\n%s", out)
	}
	cmd2, _ := domain.NewCommandDefinition("emoji.😀", "d", "p")
	out = mustGenerate(t, bindings.Untyped(cmd2), nil)
	if strings.Contains(out, `\U0001`) {
		t.Fatalf("Go-only escape in JS string literal:\n%s", out)
	}
}
