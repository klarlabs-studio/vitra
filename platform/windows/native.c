//go:build windows && cgo && vitra_native

#include "native.h"
#include <stdlib.h>
#include <string.h>
#include <windows.h>

extern void goVitraIdle(void *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);

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
};

static const char *kClassName = "VitraWinClass";
static int g_class_registered = 0;
static int g_quit = 0;

static LRESULT CALLBACK vitra_wnd_proc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
	VitraWin *w = (VitraWin *)GetWindowLongPtr(hwnd, GWLP_USERDATA);
	switch (msg) {
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

void vitra_win_free(VitraWin *w) {
	if (!w) {
		return;
	}
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
