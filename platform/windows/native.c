//go:build windows && cgo && vitra_native

#include "native.h"
#include <ctype.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>
#include <commdlg.h>
#include <shellapi.h>

extern void goVitraIdle(void *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);
extern void goVitraAction(char *);

#define VITRA_MAX_ACTIONS 256
#define VITRA_CMD_BASE 1000
#define VITRA_TRAY_CMD_BASE 5000
#define WM_TRAYICON (WM_USER + 42)

typedef struct {
	UINT id;
	char *action;
} ActionEntry;

struct VitraWin {
	HWND hwnd;
	char *id;
	char *pending_uri;
	char *icon_path;
	int maximized;
	int fullscreen;
	int above;
	int minimized;
	int hidden;
	int req_width;
	int req_height;
	WINDOWPLACEMENT saved_placement;
	LONG_PTR saved_style;
	HMENU menubar;
	ActionEntry actions[VITRA_MAX_ACTIONS];
	int n_actions;
	ACCEL accels[VITRA_MAX_ACTIONS];
	char *accel_actions[VITRA_MAX_ACTIONS];
	int n_accels;
	HACCEL accel;
	UINT next_cmd;
};

static const char *kClassName = "VitraWinClass";
static const char *kTrayClassName = "VitraTrayClass";
static int g_class_registered = 0;
static int g_tray_class_registered = 0;
static int g_quit = 0;

static HWND g_tray_hwnd = NULL;
static NOTIFYICONDATAA g_nid;
static int g_tray_added = 0;
static HMENU g_tray_menu = NULL;
static ActionEntry g_tray_actions[VITRA_MAX_ACTIONS];
static int g_tray_n_actions = 0;
static UINT g_tray_next_cmd = VITRA_TRAY_CMD_BASE;

static void free_actions(ActionEntry *entries, int n) {
	for (int i = 0; i < n; i++) {
		free(entries[i].action);
		entries[i].action = NULL;
		entries[i].id = 0;
	}
}

static const char *lookup_action(ActionEntry *entries, int n, UINT id) {
	for (int i = 0; i < n; i++) {
		if (entries[i].id == id) {
			return entries[i].action;
		}
	}
	return NULL;
}

#ifndef NIN_SELECT
#define NIN_SELECT (WM_USER + 0)
#endif

static int parse_shortcut(const char *shortcut, ACCEL *out) {
	if (!shortcut || shortcut[0] == '\0' || !out) {
		return 0;
	}
	memset(out, 0, sizeof(*out));
	out->fVirt = FVIRTKEY;
	char buf[128];
	strncpy(buf, shortcut, sizeof(buf) - 1);
	buf[sizeof(buf) - 1] = '\0';
	/* Accept GTK form "<Control>q" by normalizing brackets. */
	for (char *p = buf; *p; p++) {
		if (*p == '<' || *p == '>') {
			*p = '+';
		}
	}
	char key = 0;
	char *cursor = buf;
	while (*cursor) {
		while (*cursor == '+') {
			cursor++;
		}
		if (*cursor == '\0') {
			break;
		}
		char *tok = cursor;
		while (*cursor && *cursor != '+') {
			cursor++;
		}
		if (*cursor == '+') {
			*cursor++ = '\0';
		}
		while (*tok && isspace((unsigned char)*tok)) {
			tok++;
		}
		char *end = tok + strlen(tok);
		while (end > tok && isspace((unsigned char)end[-1])) {
			*--end = '\0';
		}
		char lower[64];
		size_t n = strlen(tok);
		if (n >= sizeof(lower)) {
			n = sizeof(lower) - 1;
		}
		for (size_t i = 0; i < n; i++) {
			lower[i] = (char)tolower((unsigned char)tok[i]);
		}
		lower[n] = '\0';
		if (strcmp(lower, "ctrl") == 0 || strcmp(lower, "control") == 0) {
			out->fVirt |= FCONTROL;
		} else if (strcmp(lower, "shift") == 0) {
			out->fVirt |= FSHIFT;
		} else if (strcmp(lower, "alt") == 0 || strcmp(lower, "option") == 0) {
			out->fVirt |= FALT;
		} else if (strcmp(lower, "win") == 0 || strcmp(lower, "meta") == 0 || strcmp(lower, "super") == 0) {
			/* Win key not representable in ACCEL; ignore modifier. */
		} else if (n == 1) {
			key = (char)toupper((unsigned char)lower[0]);
		}
	}
	if (key == 0) {
		return 0;
	}
	out->key = (WORD)key;
	return 1;
}

static HMENU find_or_create_popup(HMENU menubar, const char *menu_label) {
	int count = GetMenuItemCount(menubar);
	for (int i = 0; i < count; i++) {
		char buf[256];
		if (GetMenuStringA(menubar, (UINT)i, buf, (int)sizeof(buf), MF_BYPOSITION) > 0) {
			if (strcmp(buf, menu_label) == 0) {
				HMENU sub = GetSubMenu(menubar, i);
				if (sub) {
					return sub;
				}
			}
		}
	}
	HMENU popup = CreatePopupMenu();
	AppendMenuA(menubar, MF_POPUP, (UINT_PTR)popup, menu_label);
	return popup;
}

static void dispatch_command(VitraWin *w, UINT cmd) {
	if (w) {
		const char *action = lookup_action(w->actions, w->n_actions, cmd);
		if (action) {
			goVitraAction((char *)action);
			return;
		}
	}
	const char *tray = lookup_action(g_tray_actions, g_tray_n_actions, cmd);
	if (tray) {
		goVitraAction((char *)tray);
	}
}

static void show_tray_menu(HWND hwnd) {
	if (!g_tray_menu) {
		return;
	}
	POINT pt;
	GetCursorPos(&pt);
	SetForegroundWindow(hwnd);
	TrackPopupMenu(g_tray_menu, TPM_RIGHTBUTTON | TPM_BOTTOMALIGN | TPM_LEFTALIGN,
		pt.x, pt.y, 0, hwnd, NULL);
	PostMessage(hwnd, WM_NULL, 0, 0);
}

static LRESULT CALLBACK vitra_wnd_proc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
	VitraWin *w = (VitraWin *)GetWindowLongPtr(hwnd, GWLP_USERDATA);
	switch (msg) {
	case WM_COMMAND:
		dispatch_command(w, LOWORD(wParam));
		return 0;
	case WM_DESTROY:
		if (w && w->id) {
			goVitraDestroy(w->id);
		}
		return 0;
	case WM_CLOSE:
		DestroyWindow(hwnd);
		return 0;
	default:
		return DefWindowProc(hwnd, msg, wParam, lParam);
	}
}

static LRESULT CALLBACK tray_wnd_proc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
	switch (msg) {
	case WM_TRAYICON:
		if (lParam == WM_RBUTTONUP || lParam == WM_CONTEXTMENU) {
			show_tray_menu(hwnd);
		} else if (lParam == WM_LBUTTONUP || lParam == NIN_SELECT) {
			goVitraAction("tray.activate");
		}
		return 0;
	case WM_COMMAND:
		dispatch_command(NULL, LOWORD(wParam));
		return 0;
	default:
		return DefWindowProc(hwnd, msg, wParam, lParam);
	}
}

static void ensure_tray_window(void) {
	if (g_tray_hwnd) {
		return;
	}
	if (!g_tray_class_registered) {
		WNDCLASSEXA wc;
		memset(&wc, 0, sizeof(wc));
		wc.cbSize = sizeof(wc);
		wc.lpfnWndProc = tray_wnd_proc;
		wc.hInstance = GetModuleHandle(NULL);
		wc.lpszClassName = kTrayClassName;
		RegisterClassExA(&wc);
		g_tray_class_registered = 1;
	}
	g_tray_hwnd = CreateWindowExA(0, kTrayClassName, "", 0, 0, 0, 0, 0,
		HWND_MESSAGE, NULL, GetModuleHandle(NULL), NULL);
}

void vitra_win32_init(void) {
	if (g_class_registered) {
		return;
	}
	WNDCLASSEXA wc;
	memset(&wc, 0, sizeof(wc));
	wc.cbSize = sizeof(wc);
	wc.lpfnWndProc = vitra_wnd_proc;
	wc.hInstance = GetModuleHandle(NULL);
	wc.lpszClassName = kClassName;
	wc.hCursor = LoadCursor(NULL, IDC_ARROW);
	wc.hbrBackground = (HBRUSH)(COLOR_WINDOW + 1);
	RegisterClassExA(&wc);
	g_class_registered = 1;
}

void vitra_win32_main(void) {
	MSG msg;
	g_quit = 0;
	while (!g_quit && GetMessage(&msg, NULL, 0, 0) > 0) {
		VitraWin *w = NULL;
		if (msg.hwnd) {
			w = (VitraWin *)GetWindowLongPtr(msg.hwnd, GWLP_USERDATA);
		}
		if (w && w->accel && TranslateAccelerator(w->hwnd, w->accel, &msg)) {
			continue;
		}
		TranslateMessage(&msg);
		DispatchMessage(&msg);
	}
}

void vitra_win32_quit(void) {
	g_quit = 1;
	PostQuitMessage(0);
}

void vitra_idle_add(void *data) {
	/* Queue onto the UI thread via a custom message on a hidden helper — for
	   the first scaffold, invoke immediately when already on the UI thread. */
	goVitraIdle(data);
}

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload) {
	(void)preload;
	vitra_win32_init();
	VitraWin *w = (VitraWin *)calloc(1, sizeof(VitraWin));
	if (!w) {
		return NULL;
	}
	w->id = _strdup(id ? id : "");
	w->next_cmd = VITRA_CMD_BASE;
	if (uri && uri[0] != '\0') {
		w->pending_uri = _strdup(uri);
	}
	int wpx = width > 0 ? width : 1024;
	int hpx = height > 0 ? height : 768;
	w->req_width = wpx;
	w->req_height = hpx;
	w->hwnd = CreateWindowExA(
		0, kClassName, title ? title : "",
		WS_OVERLAPPEDWINDOW | WS_VISIBLE,
		CW_USEDEFAULT, CW_USEDEFAULT, wpx, hpx,
		NULL, NULL, GetModuleHandle(NULL), NULL);
	if (!w->hwnd) {
		free(w->pending_uri);
		free(w->id);
		free(w);
		return NULL;
	}
	SetWindowLongPtr(w->hwnd, GWLP_USERDATA, (LONG_PTR)w);
	ShowWindow(w->hwnd, SW_SHOW);
	UpdateWindow(w->hwnd);
	if (w->pending_uri) {
		/* WebView2 navigation lands in a follow-up slice; still consult nav policy. */
		(void)goVitraNav(w->id, w->pending_uri);
	}
	return w;
}

void vitra_win_navigate(VitraWin *w, const char *uri) {
	if (!w || !uri) {
		return;
	}
	free(w->pending_uri);
	w->pending_uri = _strdup(uri);
	(void)goVitraNav(w->id, w->pending_uri);
}

int vitra_win_eval(VitraWin *w, const char *js) {
	(void)w;
	(void)js;
	/* WebView2 EvaluateScript lands with the WebView2 SDK slice. */
	return 0;
}

void vitra_win_close(VitraWin *w) {
	if (!w || !w->hwnd) {
		return;
	}
	DestroyWindow(w->hwnd);
	w->hwnd = NULL;
}

void vitra_win_clear_menu(VitraWin *w) {
	if (!w) {
		return;
	}
	if (w->hwnd && w->menubar) {
		SetMenu(w->hwnd, NULL);
		DrawMenuBar(w->hwnd);
	}
	if (w->menubar) {
		DestroyMenu(w->menubar);
		w->menubar = NULL;
	}
	free_actions(w->actions, w->n_actions);
	w->n_actions = 0;
	for (int i = 0; i < w->n_accels; i++) {
		free(w->accel_actions[i]);
		w->accel_actions[i] = NULL;
	}
	w->n_accels = 0;
	if (w->accel) {
		DestroyAcceleratorTable(w->accel);
		w->accel = NULL;
	}
	w->next_cmd = VITRA_CMD_BASE;
}

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut) {
	if (!w || !w->hwnd || !menu_label || !item_id || !item_label) {
		return;
	}
	if (w->n_actions >= VITRA_MAX_ACTIONS) {
		return;
	}
	if (!w->menubar) {
		w->menubar = CreateMenu();
		SetMenu(w->hwnd, w->menubar);
	}
	HMENU popup = find_or_create_popup(w->menubar, menu_label);
	if (!popup) {
		return;
	}
	UINT cmd = w->next_cmd++;
	AppendMenuA(popup, MF_STRING, cmd, item_label);
	w->actions[w->n_actions].id = cmd;
	w->actions[w->n_actions].action = _strdup(item_id);
	w->n_actions++;
	if (shortcut && shortcut[0] != '\0' && w->n_accels < VITRA_MAX_ACTIONS) {
		ACCEL a;
		if (parse_shortcut(shortcut, &a)) {
			a.cmd = (WORD)cmd;
			w->accels[w->n_accels] = a;
			w->accel_actions[w->n_accels] = _strdup(item_id);
			w->n_accels++;
			if (w->accel) {
				DestroyAcceleratorTable(w->accel);
			}
			w->accel = CreateAcceleratorTable(w->accels, w->n_accels);
		}
	}
	DrawMenuBar(w->hwnd);
}

int vitra_win_activate_accel(VitraWin *w, const char *shortcut) {
	if (!w || !shortcut || shortcut[0] == '\0') {
		return 0;
	}
	ACCEL want;
	if (!parse_shortcut(shortcut, &want)) {
		return 0;
	}
	for (int i = 0; i < w->n_accels; i++) {
		if (w->accels[i].key == want.key &&
			(w->accels[i].fVirt & (FCONTROL | FSHIFT | FALT | FVIRTKEY)) ==
				(want.fVirt & (FCONTROL | FSHIFT | FALT | FVIRTKEY))) {
			if (w->accel_actions[i]) {
				goVitraAction(w->accel_actions[i]);
				return 1;
			}
		}
	}
	return 0;
}

void vitra_win_free(VitraWin *w) {
	if (!w) {
		return;
	}
	vitra_win_clear_menu(w);
	if (w->hwnd) {
		SetWindowLongPtr(w->hwnd, GWLP_USERDATA, 0);
		DestroyWindow(w->hwnd);
		w->hwnd = NULL;
	}
	free(w->pending_uri);
	free(w->icon_path);
	free(w->id);
	free(w);
}

void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path) {
	if (!w || !w->hwnd) {
		return;
	}
	HWND hwnd = w->hwnd;
	if (title) {
		SetWindowTextA(hwnd, title);
	}
	if (width > 0 && height > 0) {
		w->req_width = width;
		w->req_height = height;
		RECT rc;
		GetWindowRect(hwnd, &rc);
		MoveWindow(hwnd, rc.left, rc.top, width, height, TRUE);
	}
	w->maximized = maximized ? 1 : 0;
	w->fullscreen = fullscreen ? 1 : 0;
	w->above = above ? 1 : 0;
	w->minimized = minimized ? 1 : 0;
	w->hidden = hidden ? 1 : 0;

	if (fullscreen) {
		if (!w->saved_style) {
			w->saved_placement.length = sizeof(WINDOWPLACEMENT);
			GetWindowPlacement(hwnd, &w->saved_placement);
			w->saved_style = GetWindowLongPtr(hwnd, GWL_STYLE);
		}
		SetWindowLongPtr(hwnd, GWL_STYLE, WS_POPUP | WS_VISIBLE);
		MONITORINFO mi;
		mi.cbSize = sizeof(mi);
		HMONITOR mon = MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST);
		GetMonitorInfo(mon, &mi);
		SetWindowPos(hwnd, HWND_TOP, mi.rcMonitor.left, mi.rcMonitor.top,
			mi.rcMonitor.right - mi.rcMonitor.left, mi.rcMonitor.bottom - mi.rcMonitor.top,
			SWP_FRAMECHANGED);
	} else if (w->saved_style) {
		SetWindowLongPtr(hwnd, GWL_STYLE, w->saved_style);
		SetWindowPlacement(hwnd, &w->saved_placement);
		w->saved_style = 0;
	}

	if (!fullscreen) {
		if (maximized) {
			ShowWindow(hwnd, SW_MAXIMIZE);
		} else if (minimized) {
			ShowWindow(hwnd, SW_MINIMIZE);
		} else if (hidden) {
			ShowWindow(hwnd, SW_HIDE);
		} else {
			ShowWindow(hwnd, SW_RESTORE);
		}
	}

	SetWindowPos(hwnd, above ? HWND_TOPMOST : HWND_NOTOPMOST, 0, 0, 0, 0,
		SWP_NOMOVE | SWP_NOSIZE);

	if (icon_path && icon_path[0] != '\0') {
		HICON icon = (HICON)LoadImageA(NULL, icon_path, IMAGE_ICON, 0, 0, LR_LOADFROMFILE | LR_DEFAULTSIZE);
		if (icon) {
			SendMessage(hwnd, WM_SETICON, ICON_BIG, (LPARAM)icon);
			SendMessage(hwnd, WM_SETICON, ICON_SMALL, (LPARAM)icon);
			free(w->icon_path);
			w->icon_path = _strdup(icon_path);
		}
	}
}

VitraChrome vitra_win_chrome(VitraWin *w) {
	VitraChrome c;
	memset(&c, 0, sizeof(c));
	c.title = _strdup("");
	c.icon_path = _strdup("");
	if (!w || !w->hwnd) {
		return c;
	}
	char title[512];
	GetWindowTextA(w->hwnd, title, sizeof(title));
	free(c.title);
	c.title = _strdup(title);
	free(c.icon_path);
	c.icon_path = _strdup(w->icon_path ? w->icon_path : "");
	RECT rc;
	GetClientRect(w->hwnd, &rc);
	c.width = (int)(rc.right - rc.left);
	c.height = (int)(rc.bottom - rc.top);
	if (c.width <= 0) {
		c.width = w->req_width;
	}
	if (c.height <= 0) {
		c.height = w->req_height;
	}
	c.maximized = IsZoomed(w->hwnd) ? 1 : 0;
	c.fullscreen = w->fullscreen;
	c.above = w->above;
	c.minimized = IsIconic(w->hwnd) ? 1 : 0;
	c.hidden = !IsWindowVisible(w->hwnd) ? 1 : 0;
	return c;
}

char *vitra_open_dialog(void) {
	char path[MAX_PATH];
	path[0] = '\0';
	OPENFILENAMEA ofn;
	memset(&ofn, 0, sizeof(ofn));
	ofn.lStructSize = sizeof(ofn);
	ofn.lpstrFile = path;
	ofn.nMaxFile = (DWORD)sizeof(path);
	ofn.lpstrFilter = "All Files\0*.*\0";
	ofn.nFilterIndex = 1;
	ofn.Flags = OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR;
	if (!GetOpenFileNameA(&ofn)) {
		return NULL;
	}
	return _strdup(path);
}

char *vitra_save_dialog(void) {
	char path[MAX_PATH];
	path[0] = '\0';
	OPENFILENAMEA ofn;
	memset(&ofn, 0, sizeof(ofn));
	ofn.lStructSize = sizeof(ofn);
	ofn.lpstrFile = path;
	ofn.nMaxFile = (DWORD)sizeof(path);
	ofn.lpstrFilter = "All Files\0*.*\0";
	ofn.nFilterIndex = 1;
	ofn.Flags = OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST | OFN_NOCHANGEDIR;
	if (!GetSaveFileNameA(&ofn)) {
		return NULL;
	}
	return _strdup(path);
}

void vitra_tray_set(const char *tooltip) {
	ensure_tray_window();
	if (!g_tray_hwnd) {
		return;
	}
	memset(&g_nid, 0, sizeof(g_nid));
	g_nid.cbSize = sizeof(g_nid);
	g_nid.hWnd = g_tray_hwnd;
	g_nid.uID = 1;
	g_nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
	g_nid.uCallbackMessage = WM_TRAYICON;
	g_nid.hIcon = LoadIcon(NULL, IDI_APPLICATION);
	if (tooltip) {
		strncpy(g_nid.szTip, tooltip, sizeof(g_nid.szTip) - 1);
	} else {
		g_nid.szTip[0] = '\0';
	}
	if (!g_tray_added) {
		if (Shell_NotifyIconA(NIM_ADD, &g_nid)) {
			g_tray_added = 1;
		}
	} else {
		Shell_NotifyIconA(NIM_MODIFY, &g_nid);
	}
}

void vitra_tray_clear_menu(void) {
	if (g_tray_menu) {
		DestroyMenu(g_tray_menu);
		g_tray_menu = NULL;
	}
	free_actions(g_tray_actions, g_tray_n_actions);
	g_tray_n_actions = 0;
	g_tray_next_cmd = VITRA_TRAY_CMD_BASE;
}

void vitra_tray_add_menu_item(const char *item_id, const char *item_label) {
	if (!item_id || !item_label || g_tray_n_actions >= VITRA_MAX_ACTIONS) {
		return;
	}
	if (!g_tray_menu) {
		g_tray_menu = CreatePopupMenu();
	}
	UINT cmd = g_tray_next_cmd++;
	AppendMenuA(g_tray_menu, MF_STRING, cmd, item_label);
	g_tray_actions[g_tray_n_actions].id = cmd;
	g_tray_actions[g_tray_n_actions].action = _strdup(item_id);
	g_tray_n_actions++;
}

void vitra_tray_clear(void) {
	vitra_tray_clear_menu();
	if (g_tray_added) {
		Shell_NotifyIconA(NIM_DELETE, &g_nid);
		g_tray_added = 0;
	}
}
