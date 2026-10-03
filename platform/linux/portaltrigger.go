//go:build linux

package linux

import (
	"fmt"
	"strings"
	"unicode"
)

// portalTrigger translates a Vitra accelerator ("Ctrl+Shift+Y", or the GTK
// form "<Control><Shift>y") into the XDG shortcuts specification's trigger
// format used by the GlobalShortcuts portal's preferred_trigger: upper-case
// modifiers (CTRL, ALT, SHIFT, LOGO) joined by "+" in a fixed order, then an
// xkb keysym name ("CTRL+SHIFT+y", "LOGO+F5", "ALT+space").
//
// As with XGrabKey on X11, at least one modifier is required: a bare key is
// not a reasonable OS-wide shortcut.
func portalTrigger(accelerator string) (string, error) {
	mods, key, err := splitAccelerator(accelerator)
	if err != nil {
		return "", err
	}
	if len(mods) == 0 {
		return "", fmt.Errorf("global shortcut %q needs at least one modifier", accelerator)
	}
	sym, ok := keysymName(key)
	if !ok {
		return "", fmt.Errorf("global shortcut %q: unknown key %q", accelerator, key)
	}
	var b strings.Builder
	for _, m := range []string{"CTRL", "ALT", "SHIFT", "LOGO"} {
		if mods[m] {
			b.WriteString(m)
			b.WriteByte('+')
		}
	}
	b.WriteString(sym)
	return b.String(), nil
}

// splitAccelerator returns the set of portal modifier names and the key of an
// accelerator in either "Mod+Mod+Key" or GTK "<Mod><Mod>key" form.
func splitAccelerator(accelerator string) (map[string]bool, string, error) {
	s := strings.TrimSpace(accelerator)
	if s == "" {
		return nil, "", fmt.Errorf("empty accelerator")
	}
	var modNames []string
	var key string
	if strings.HasPrefix(s, "<") {
		for strings.HasPrefix(s, "<") {
			end := strings.IndexByte(s, '>')
			if end < 0 {
				return nil, "", fmt.Errorf("accelerator %q: unclosed modifier", accelerator)
			}
			modNames = append(modNames, s[1:end])
			s = s[end+1:]
		}
		key = strings.TrimSpace(s)
	} else {
		parts := strings.Split(s, "+")
		// "Ctrl++" names the plus key: Split yields two trailing empty parts.
		if strings.HasSuffix(s, "++") {
			parts = append(parts[:len(parts)-2], "+")
		}
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if i == len(parts)-1 {
				key = p
				break
			}
			if p == "" {
				return nil, "", fmt.Errorf("accelerator %q: empty modifier", accelerator)
			}
			modNames = append(modNames, p)
		}
	}
	if key == "" {
		return nil, "", fmt.Errorf("accelerator %q has no key", accelerator)
	}
	mods := make(map[string]bool, len(modNames))
	for _, m := range modNames {
		name, ok := portalModifier(m)
		if !ok {
			return nil, "", fmt.Errorf("accelerator %q: unknown modifier %q", accelerator, m)
		}
		mods[name] = true
	}
	return mods, key, nil
}

func portalModifier(m string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "ctrl", "control", "primary", "cmdorctrl", "commandorcontrol":
		return "CTRL", true
	case "alt", "mod1", "option":
		return "ALT", true
	case "shift":
		return "SHIFT", true
	case "super", "meta", "win", "logo", "cmd", "command", "mod4", "hyper":
		return "LOGO", true
	}
	return "", false
}

// namedKeys maps accelerator key names (lower-cased) to xkb keysym names.
var namedKeys = map[string]string{
	"space": "space", "tab": "Tab", "enter": "Return", "return": "Return",
	"esc": "Escape", "escape": "Escape", "backspace": "BackSpace",
	"delete": "Delete", "del": "Delete", "insert": "Insert", "ins": "Insert",
	"home": "Home", "end": "End", "pageup": "Page_Up", "page_up": "Page_Up",
	"pagedown": "Page_Down", "page_down": "Page_Down",
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
	"print": "Print", "printscreen": "Print", "pause": "Pause",
	"plus": "plus", "minus": "minus", "equal": "equal", "comma": "comma",
	"period": "period", "slash": "slash", "backslash": "backslash",
	"semicolon": "semicolon", "apostrophe": "apostrophe", "grave": "grave",
	"bracketleft": "bracketleft", "bracketright": "bracketright",
}

// punctuation maps single printable characters to their xkb keysym names.
var punctuation = map[rune]string{
	'+': "plus", '-': "minus", '=': "equal", ',': "comma", '.': "period",
	'/': "slash", '\\': "backslash", ';': "semicolon", '\'': "apostrophe",
	'`': "grave", '[': "bracketleft", ']': "bracketright",
}

func keysymName(key string) (string, bool) {
	if r := []rune(key); len(r) == 1 {
		c := r[0]
		switch {
		case c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c)):
			return string(unicode.ToLower(c)), true
		case punctuation[c] != "":
			return punctuation[c], true
		}
		return "", false
	}
	lower := strings.ToLower(key)
	if name, ok := namedKeys[lower]; ok {
		return name, true
	}
	if len(lower) >= 2 && lower[0] == 'f' {
		n := 0
		for _, c := range lower[1:] {
			if c < '0' || c > '9' {
				return "", false
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 24 {
			return fmt.Sprintf("F%d", n), true
		}
	}
	return "", false
}
