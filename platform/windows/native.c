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
	free(w->id);
	free(w);
}
