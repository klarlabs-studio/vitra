//go:build windows && cgo && vitra_native

#include "webview2_min.h"
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <objbase.h>

extern void goVitraMessage(char *, char *);
extern int goVitraNav(char *, char *);

typedef struct VitraWV2 VitraWV2;

struct VitraWV2 {
	HWND hwnd;
	char *id;
	char *pending_uri;
	char *preload;
	ICoreWebView2Controller *controller;
	ICoreWebView2 *webview;
	ICoreWebView2Environment *env;
	volatile int ready;
	volatile int failed;

	ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler env_handler;
	ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl env_vtbl;
	ICoreWebView2CreateCoreWebView2ControllerCompletedHandler ctrl_handler;
	ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl ctrl_vtbl;
	ICoreWebView2WebMessageReceivedEventHandler msg_handler;
	ICoreWebView2WebMessageReceivedEventHandlerVtbl msg_vtbl;
	ICoreWebView2NavigationStartingEventHandler nav_handler;
	ICoreWebView2NavigationStartingEventHandlerVtbl nav_vtbl;
	ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler script_handler;
	ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandlerVtbl script_vtbl;
	ICoreWebView2ExecuteScriptCompletedHandler eval_handler;
	ICoreWebView2ExecuteScriptCompletedHandlerVtbl eval_vtbl;
};

static CreateCoreWebView2EnvironmentWithOptionsFn g_create_env = NULL;
static GetAvailableCoreWebView2BrowserVersionStringFn g_get_version = NULL;
static int g_loader_tried = 0;
static int g_com_inited = 0;

#define WV2_FROM(ptr, field) ((VitraWV2 *)((char *)(ptr) - offsetof(VitraWV2, field)))

static wchar_t *utf8_to_wide(const char *s) {
	if (!s) {
		return NULL;
	}
	int n = MultiByteToWideChar(CP_UTF8, 0, s, -1, NULL, 0);
	if (n <= 0) {
		return NULL;
	}
	wchar_t *w = (wchar_t *)malloc((size_t)n * sizeof(wchar_t));
	if (!w) {
		return NULL;
	}
	MultiByteToWideChar(CP_UTF8, 0, s, -1, w, n);
	return w;
}

static char *wide_to_utf8(LPCWSTR w) {
	if (!w) {
		return NULL;
	}
	int n = WideCharToMultiByte(CP_UTF8, 0, w, -1, NULL, 0, NULL, NULL);
	if (n <= 0) {
		return NULL;
	}
	char *s = (char *)malloc((size_t)n);
	if (!s) {
		return NULL;
	}
	WideCharToMultiByte(CP_UTF8, 0, w, -1, s, n, NULL, NULL);
	return s;
}

static HRESULT STDMETHODCALLTYPE noop_qi(void *This, REFIID riid, void **ppv) {
	(void)This;
	(void)riid;
	if (ppv) {
		*ppv = This;
	}
	return S_OK;
}
static ULONG STDMETHODCALLTYPE noop_addref(void *This) {
	(void)This;
	return 1;
}
static ULONG STDMETHODCALLTYPE noop_release(void *This) {
	(void)This;
	return 1;
}

static HRESULT STDMETHODCALLTYPE on_script_created(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *This, HRESULT errorCode, LPCWSTR id) {
	(void)errorCode;
	(void)id;
	VitraWV2 *wv = WV2_FROM(This, script_handler);
	wv->ready = 1;
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE on_eval_done(ICoreWebView2ExecuteScriptCompletedHandler *This, HRESULT errorCode, LPCWSTR result) {
	(void)This;
	(void)errorCode;
	(void)result;
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE on_web_message(ICoreWebView2WebMessageReceivedEventHandler *This, ICoreWebView2 *sender, ICoreWebView2WebMessageReceivedEventArgs *args) {
	(void)sender;
	VitraWV2 *wv = WV2_FROM(This, msg_handler);
	LPWSTR wmsg = NULL;
	HRESULT hr = args->lpVtbl->TryGetWebMessageAsString(args, &wmsg);
	if (FAILED(hr) || !wmsg) {
		hr = args->lpVtbl->get_WebMessageAsJson(args, &wmsg);
	}
	if (SUCCEEDED(hr) && wmsg) {
		char *msg = wide_to_utf8(wmsg);
		CoTaskMemFree(wmsg);
		if (msg && wv->id) {
			goVitraMessage(wv->id, msg);
		}
		free(msg);
	}
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE on_nav_starting(ICoreWebView2NavigationStartingEventHandler *This, ICoreWebView2 *sender, ICoreWebView2NavigationStartingEventArgs *args) {
	(void)sender;
	VitraWV2 *wv = WV2_FROM(This, nav_handler);
	LPWSTR wuri = NULL;
	if (FAILED(args->lpVtbl->get_Uri(args, &wuri)) || !wuri) {
		return S_OK;
	}
	char *uri = wide_to_utf8(wuri);
	CoTaskMemFree(wuri);
	if (!uri) {
		return S_OK;
	}
	if (!goVitraNav(wv->id, uri)) {
		args->lpVtbl->put_Cancel(args, TRUE);
	}
	free(uri);
	return S_OK;
}

static void wv2_navigate_pending(VitraWV2 *wv) {
	if (!wv || !wv->webview || !wv->pending_uri || wv->pending_uri[0] == '\0') {
		return;
	}
	wchar_t *wuri = utf8_to_wide(wv->pending_uri);
	if (!wuri) {
		return;
	}
	wv->webview->lpVtbl->Navigate(wv->webview, wuri);
	free(wuri);
}

static HRESULT STDMETHODCALLTYPE on_controller_created(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *This, HRESULT result, ICoreWebView2Controller *controller) {
	VitraWV2 *wv = WV2_FROM(This, ctrl_handler);
	if (FAILED(result) || !controller) {
		wv->failed = 1;
		return E_FAIL;
	}
	controller->lpVtbl->AddRef(controller);
	wv->controller = controller;
	ICoreWebView2 *webview = NULL;
	if (FAILED(controller->lpVtbl->get_CoreWebView2(controller, &webview)) || !webview) {
		wv->failed = 1;
		return E_FAIL;
	}
	wv->webview = webview;

	ICoreWebView2Settings *settings = NULL;
	if (SUCCEEDED(webview->lpVtbl->get_Settings(webview, &settings)) && settings) {
		settings->lpVtbl->put_IsWebMessageEnabled(settings, TRUE);
		settings->lpVtbl->Release(settings);
	}

	RECT bounds;
	GetClientRect(wv->hwnd, &bounds);
	controller->lpVtbl->put_Bounds(controller, bounds);

	EventRegistrationToken token;
	webview->lpVtbl->add_WebMessageReceived(webview, &wv->msg_handler, &token);
	webview->lpVtbl->add_NavigationStarting(webview, &wv->nav_handler, &token);

	const char *preload = (wv->preload && wv->preload[0] != '\0') ? wv->preload : "/* vitra */";
	wchar_t *wpreload = utf8_to_wide(preload);
	if (wpreload) {
		webview->lpVtbl->AddScriptToExecuteOnDocumentCreated(webview, wpreload, &wv->script_handler);
		free(wpreload);
	} else {
		wv->ready = 1;
	}
	wv2_navigate_pending(wv);
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE on_env_created(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *This, HRESULT result, ICoreWebView2Environment *env) {
	VitraWV2 *wv = WV2_FROM(This, env_handler);
	if (FAILED(result) || !env) {
		wv->failed = 1;
		return E_FAIL;
	}
	env->lpVtbl->AddRef(env);
	wv->env = env;
	HRESULT hr = env->lpVtbl->CreateCoreWebView2Controller(env, wv->hwnd, &wv->ctrl_handler);
	if (FAILED(hr)) {
		wv->failed = 1;
		return hr;
	}
	return S_OK;
}

static void init_handlers(VitraWV2 *wv) {
	wv->env_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *, REFIID, void **))noop_qi;
	wv->env_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *))noop_addref;
	wv->env_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *))noop_release;
	wv->env_vtbl.Invoke = on_env_created;
	wv->env_handler.lpVtbl = &wv->env_vtbl;

	wv->ctrl_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *, REFIID, void **))noop_qi;
	wv->ctrl_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *))noop_addref;
	wv->ctrl_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *))noop_release;
	wv->ctrl_vtbl.Invoke = on_controller_created;
	wv->ctrl_handler.lpVtbl = &wv->ctrl_vtbl;

	wv->msg_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2WebMessageReceivedEventHandler *, REFIID, void **))noop_qi;
	wv->msg_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2WebMessageReceivedEventHandler *))noop_addref;
	wv->msg_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2WebMessageReceivedEventHandler *))noop_release;
	wv->msg_vtbl.Invoke = on_web_message;
	wv->msg_handler.lpVtbl = &wv->msg_vtbl;

	wv->nav_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2NavigationStartingEventHandler *, REFIID, void **))noop_qi;
	wv->nav_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2NavigationStartingEventHandler *))noop_addref;
	wv->nav_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2NavigationStartingEventHandler *))noop_release;
	wv->nav_vtbl.Invoke = on_nav_starting;
	wv->nav_handler.lpVtbl = &wv->nav_vtbl;

	wv->script_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *, REFIID, void **))noop_qi;
	wv->script_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *))noop_addref;
	wv->script_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *))noop_release;
	wv->script_vtbl.Invoke = on_script_created;
	wv->script_handler.lpVtbl = &wv->script_vtbl;

	wv->eval_vtbl.QueryInterface = (HRESULT(STDMETHODCALLTYPE *)(ICoreWebView2ExecuteScriptCompletedHandler *, REFIID, void **))noop_qi;
	wv->eval_vtbl.AddRef = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2ExecuteScriptCompletedHandler *))noop_addref;
	wv->eval_vtbl.Release = (ULONG(STDMETHODCALLTYPE *)(ICoreWebView2ExecuteScriptCompletedHandler *))noop_release;
	wv->eval_vtbl.Invoke = on_eval_done;
	wv->eval_handler.lpVtbl = &wv->eval_vtbl;
}

static int ensure_loader(void) {
	if (g_loader_tried) {
		return g_create_env != NULL;
	}
	g_loader_tried = 1;
	if (!g_com_inited) {
		HRESULT hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
		if (SUCCEEDED(hr) || hr == S_FALSE || hr == RPC_E_CHANGED_MODE) {
			g_com_inited = 1;
		}
	}
	HMODULE mod = LoadLibraryA("WebView2Loader.dll");
	if (!mod) {
		return 0;
	}
	g_create_env = (CreateCoreWebView2EnvironmentWithOptionsFn)GetProcAddress(mod, "CreateCoreWebView2EnvironmentWithOptions");
	g_get_version = (GetAvailableCoreWebView2BrowserVersionStringFn)GetProcAddress(mod, "GetAvailableCoreWebView2BrowserVersionString");
	if (!g_create_env) {
		return 0;
	}
	if (g_get_version) {
		LPWSTR ver = NULL;
		if (FAILED(g_get_version(NULL, &ver)) || !ver) {
			return 0;
		}
		CoTaskMemFree(ver);
	}
	return 1;
}

static void pump_until_ready(VitraWV2 *wv, DWORD timeout_ms) {
	DWORD start = GetTickCount();
	MSG msg;
	while (!wv->ready && !wv->failed) {
		DWORD elapsed = GetTickCount() - start;
		if (elapsed >= timeout_ms) {
			wv->failed = 1;
			break;
		}
		DWORD wait = MsgWaitForMultipleObjects(0, NULL, FALSE, 50, QS_ALLINPUT);
		if (wait == WAIT_OBJECT_0) {
			while (PeekMessage(&msg, NULL, 0, 0, PM_REMOVE)) {
				TranslateMessage(&msg);
				DispatchMessage(&msg);
			}
		}
	}
}

static wchar_t *user_data_folder(wchar_t *buf, size_t n) {
	const wchar_t *local = _wgetenv(L"LOCALAPPDATA");
	if (!local) {
		return NULL;
	}
	_snwprintf(buf, n, L"%ls\\vitra-webview2", local);
	return buf;
}

VitraWV2 *vitra_wv2_attach(HWND hwnd, const char *id, const char *uri, const char *preload) {
	if (!hwnd || !ensure_loader()) {
		return NULL;
	}
	VitraWV2 *wv = (VitraWV2 *)calloc(1, sizeof(VitraWV2));
	if (!wv) {
		return NULL;
	}
	init_handlers(wv);
	wv->hwnd = hwnd;
	wv->id = _strdup(id ? id : "");
	if (uri && uri[0] != '\0') {
		wv->pending_uri = _strdup(uri);
	}
	if (preload && preload[0] != '\0') {
		wv->preload = _strdup(preload);
	}
	wchar_t folder[MAX_PATH];
	PCWSTR userData = user_data_folder(folder, MAX_PATH);
	HRESULT hr = g_create_env(NULL, userData, NULL, &wv->env_handler);
	if (FAILED(hr)) {
		free(wv->preload);
		free(wv->pending_uri);
		free(wv->id);
		free(wv);
		return NULL;
	}
	pump_until_ready(wv, 15000);
	if (wv->failed || !wv->webview) {
		/* Keep HWND shell usable; caller treats NULL attach as shell-only. */
		if (wv->controller) {
			wv->controller->lpVtbl->Close(wv->controller);
			wv->controller->lpVtbl->Release(wv->controller);
		}
		if (wv->webview) {
			wv->webview->lpVtbl->Release(wv->webview);
		}
		if (wv->env) {
			wv->env->lpVtbl->Release(wv->env);
		}
		free(wv->preload);
		free(wv->pending_uri);
		free(wv->id);
		free(wv);
		return NULL;
	}
	return wv;
}

void vitra_wv2_navigate(VitraWV2 *wv, const char *uri) {
	if (!wv || !uri) {
		return;
	}
	free(wv->pending_uri);
	wv->pending_uri = _strdup(uri);
	if (wv->webview) {
		wv2_navigate_pending(wv);
	}
}

int vitra_wv2_eval(VitraWV2 *wv, const char *js) {
	if (!wv || !wv->webview || !js) {
		return 0;
	}
	wchar_t *wjs = utf8_to_wide(js);
	if (!wjs) {
		return 0;
	}
	HRESULT hr = wv->webview->lpVtbl->ExecuteScript(wv->webview, wjs, &wv->eval_handler);
	free(wjs);
	return SUCCEEDED(hr) ? 1 : 0;
}

void vitra_wv2_resize(VitraWV2 *wv) {
	if (!wv || !wv->controller || !wv->hwnd) {
		return;
	}
	RECT bounds;
	GetClientRect(wv->hwnd, &bounds);
	wv->controller->lpVtbl->put_Bounds(wv->controller, bounds);
}

void vitra_wv2_free(VitraWV2 *wv) {
	if (!wv) {
		return;
	}
	if (wv->controller) {
		wv->controller->lpVtbl->Close(wv->controller);
		wv->controller->lpVtbl->Release(wv->controller);
		wv->controller = NULL;
	}
	if (wv->webview) {
		wv->webview->lpVtbl->Release(wv->webview);
		wv->webview = NULL;
	}
	if (wv->env) {
		wv->env->lpVtbl->Release(wv->env);
		wv->env = NULL;
	}
	free(wv->preload);
	free(wv->pending_uri);
	free(wv->id);
	free(wv);
}

int vitra_wv2_loader_available(void) {
	return ensure_loader();
}
