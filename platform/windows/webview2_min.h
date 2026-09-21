/* Minimal WebView2 COM surface for Vitra — no NuGet SDK required.
   Vtable layouts match Microsoft.Web.WebView2 Win32 IDL (stable ICoreWebView2). */
#ifndef VITRA_WEBVIEW2_MIN_H
#define VITRA_WEBVIEW2_MIN_H

#include <windows.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct ICoreWebView2 ICoreWebView2;
typedef struct ICoreWebView2Controller ICoreWebView2Controller;
typedef struct ICoreWebView2Environment ICoreWebView2Environment;
typedef struct ICoreWebView2Settings ICoreWebView2Settings;
typedef struct ICoreWebView2WebMessageReceivedEventArgs ICoreWebView2WebMessageReceivedEventArgs;
typedef struct ICoreWebView2NavigationStartingEventArgs ICoreWebView2NavigationStartingEventArgs;
typedef struct ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler;
typedef struct ICoreWebView2CreateCoreWebView2ControllerCompletedHandler ICoreWebView2CreateCoreWebView2ControllerCompletedHandler;
typedef struct ICoreWebView2WebMessageReceivedEventHandler ICoreWebView2WebMessageReceivedEventHandler;
typedef struct ICoreWebView2NavigationStartingEventHandler ICoreWebView2NavigationStartingEventHandler;
typedef struct ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler;
typedef struct ICoreWebView2ExecuteScriptCompletedHandler ICoreWebView2ExecuteScriptCompletedHandler;
typedef struct ICoreWebView2EnvironmentOptions ICoreWebView2EnvironmentOptions;

typedef struct EventRegistrationToken {
	int64_t value;
} EventRegistrationToken;

/* --- handler vtables we implement --- */

typedef struct ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *, HRESULT, ICoreWebView2Environment *);
} ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl;

struct ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler {
	ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerVtbl *lpVtbl;
};

typedef struct ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *, HRESULT, ICoreWebView2Controller *);
} ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl;

struct ICoreWebView2CreateCoreWebView2ControllerCompletedHandler {
	ICoreWebView2CreateCoreWebView2ControllerCompletedHandlerVtbl *lpVtbl;
};

typedef struct ICoreWebView2WebMessageReceivedEventHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2WebMessageReceivedEventHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2WebMessageReceivedEventHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2WebMessageReceivedEventHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2WebMessageReceivedEventHandler *, ICoreWebView2 *, ICoreWebView2WebMessageReceivedEventArgs *);
} ICoreWebView2WebMessageReceivedEventHandlerVtbl;

struct ICoreWebView2WebMessageReceivedEventHandler {
	ICoreWebView2WebMessageReceivedEventHandlerVtbl *lpVtbl;
};

typedef struct ICoreWebView2NavigationStartingEventHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2NavigationStartingEventHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2NavigationStartingEventHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2NavigationStartingEventHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2NavigationStartingEventHandler *, ICoreWebView2 *, ICoreWebView2NavigationStartingEventArgs *);
} ICoreWebView2NavigationStartingEventHandlerVtbl;

struct ICoreWebView2NavigationStartingEventHandler {
	ICoreWebView2NavigationStartingEventHandlerVtbl *lpVtbl;
};

typedef struct ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *, HRESULT, LPCWSTR);
} ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandlerVtbl;

struct ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler {
	ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandlerVtbl *lpVtbl;
};

typedef struct ICoreWebView2ExecuteScriptCompletedHandlerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2ExecuteScriptCompletedHandler *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2ExecuteScriptCompletedHandler *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2ExecuteScriptCompletedHandler *);
	HRESULT(STDMETHODCALLTYPE *Invoke)(ICoreWebView2ExecuteScriptCompletedHandler *, HRESULT, LPCWSTR);
} ICoreWebView2ExecuteScriptCompletedHandlerVtbl;

struct ICoreWebView2ExecuteScriptCompletedHandler {
	ICoreWebView2ExecuteScriptCompletedHandlerVtbl *lpVtbl;
};

/* --- SDK object vtables (only slots we call; trailing entries keep offsets correct) --- */

typedef struct ICoreWebView2EnvironmentVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2Environment *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2Environment *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2Environment *);
	HRESULT(STDMETHODCALLTYPE *CreateCoreWebView2Controller)(ICoreWebView2Environment *, HWND, ICoreWebView2CreateCoreWebView2ControllerCompletedHandler *);
	void *CreateWebResourceResponse;
	void *get_BrowserVersionString;
	void *add_NewBrowserVersionAvailable;
	void *remove_NewBrowserVersionAvailable;
} ICoreWebView2EnvironmentVtbl;

struct ICoreWebView2Environment {
	ICoreWebView2EnvironmentVtbl *lpVtbl;
};

typedef struct ICoreWebView2ControllerVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2Controller *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2Controller *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2Controller *);
	void *get_IsVisible;
	void *put_IsVisible;
	HRESULT(STDMETHODCALLTYPE *get_Bounds)(ICoreWebView2Controller *, RECT *);
	HRESULT(STDMETHODCALLTYPE *put_Bounds)(ICoreWebView2Controller *, RECT);
	void *get_ZoomFactor;
	void *put_ZoomFactor;
	void *add_ZoomFactorChanged;
	void *remove_ZoomFactorChanged;
	void *SetBoundsAndZoomFactor;
	void *MoveFocus;
	void *add_MoveFocusRequested;
	void *remove_MoveFocusRequested;
	void *add_GotFocus;
	void *remove_GotFocus;
	void *add_LostFocus;
	void *remove_LostFocus;
	void *add_AcceleratorKeyPressed;
	void *remove_AcceleratorKeyPressed;
	void *get_ParentWindow;
	void *put_ParentWindow;
	void *NotifyParentWindowPositionChanged;
	HRESULT(STDMETHODCALLTYPE *Close)(ICoreWebView2Controller *);
	HRESULT(STDMETHODCALLTYPE *get_CoreWebView2)(ICoreWebView2Controller *, ICoreWebView2 **);
} ICoreWebView2ControllerVtbl;

struct ICoreWebView2Controller {
	ICoreWebView2ControllerVtbl *lpVtbl;
};

typedef struct ICoreWebView2Vtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2 *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2 *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2 *);
	HRESULT(STDMETHODCALLTYPE *get_Settings)(ICoreWebView2 *, ICoreWebView2Settings **);
	void *get_Source;
	HRESULT(STDMETHODCALLTYPE *Navigate)(ICoreWebView2 *, LPCWSTR);
	void *NavigateToString;
	HRESULT(STDMETHODCALLTYPE *add_NavigationStarting)(ICoreWebView2 *, ICoreWebView2NavigationStartingEventHandler *, EventRegistrationToken *);
	void *remove_NavigationStarting;
	void *add_ContentLoading;
	void *remove_ContentLoading;
	void *add_SourceChanged;
	void *remove_SourceChanged;
	void *add_HistoryChanged;
	void *remove_HistoryChanged;
	void *add_NavigationCompleted;
	void *remove_NavigationCompleted;
	void *add_FrameNavigationStarting;
	void *remove_FrameNavigationStarting;
	void *add_FrameNavigationCompleted;
	void *remove_FrameNavigationCompleted;
	void *add_ScriptDialogOpening;
	void *remove_ScriptDialogOpening;
	void *add_PermissionRequested;
	void *remove_PermissionRequested;
	void *add_ProcessFailed;
	void *remove_ProcessFailed;
	HRESULT(STDMETHODCALLTYPE *AddScriptToExecuteOnDocumentCreated)(ICoreWebView2 *, LPCWSTR, ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler *);
	void *RemoveScriptToExecuteOnDocumentCreated;
	HRESULT(STDMETHODCALLTYPE *ExecuteScript)(ICoreWebView2 *, LPCWSTR, ICoreWebView2ExecuteScriptCompletedHandler *);
	void *CapturePreview;
	void *Reload;
	void *PostWebMessageAsJSON;
	void *PostWebMessageAsString;
	HRESULT(STDMETHODCALLTYPE *add_WebMessageReceived)(ICoreWebView2 *, ICoreWebView2WebMessageReceivedEventHandler *, EventRegistrationToken *);
	void *remove_WebMessageReceived;
	/* remaining slots unused */
} ICoreWebView2Vtbl;

struct ICoreWebView2 {
	ICoreWebView2Vtbl *lpVtbl;
};

typedef struct ICoreWebView2SettingsVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2Settings *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2Settings *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2Settings *);
	void *get_IsScriptEnabled;
	void *put_IsScriptEnabled;
	void *get_IsWebMessageEnabled;
	HRESULT(STDMETHODCALLTYPE *put_IsWebMessageEnabled)(ICoreWebView2Settings *, BOOL);
	void *get_AreDefaultScriptDialogsEnabled;
	void *put_AreDefaultScriptDialogsEnabled;
	void *get_IsStatusBarEnabled;
	void *put_IsStatusBarEnabled;
	void *get_AreDevToolsEnabled;
	void *put_AreDevToolsEnabled;
	void *get_AreDefaultContextMenusEnabled;
	void *put_AreDefaultContextMenusEnabled;
	void *get_AreHostObjectsAllowed;
	void *put_AreHostObjectsAllowed;
	void *get_IsZoomControlEnabled;
	void *put_IsZoomControlEnabled;
	void *get_IsBuiltInErrorPageEnabled;
	void *put_IsBuiltInErrorPageEnabled;
} ICoreWebView2SettingsVtbl;

struct ICoreWebView2Settings {
	ICoreWebView2SettingsVtbl *lpVtbl;
};

typedef struct ICoreWebView2WebMessageReceivedEventArgsVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2WebMessageReceivedEventArgs *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2WebMessageReceivedEventArgs *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2WebMessageReceivedEventArgs *);
	void *get_Source;
	HRESULT(STDMETHODCALLTYPE *get_WebMessageAsJson)(ICoreWebView2WebMessageReceivedEventArgs *, LPWSTR *);
	HRESULT(STDMETHODCALLTYPE *TryGetWebMessageAsString)(ICoreWebView2WebMessageReceivedEventArgs *, LPWSTR *);
} ICoreWebView2WebMessageReceivedEventArgsVtbl;

struct ICoreWebView2WebMessageReceivedEventArgs {
	ICoreWebView2WebMessageReceivedEventArgsVtbl *lpVtbl;
};

typedef struct ICoreWebView2NavigationStartingEventArgsVtbl {
	HRESULT(STDMETHODCALLTYPE *QueryInterface)(ICoreWebView2NavigationStartingEventArgs *, REFIID, void **);
	ULONG(STDMETHODCALLTYPE *AddRef)(ICoreWebView2NavigationStartingEventArgs *);
	ULONG(STDMETHODCALLTYPE *Release)(ICoreWebView2NavigationStartingEventArgs *);
	HRESULT(STDMETHODCALLTYPE *get_Uri)(ICoreWebView2NavigationStartingEventArgs *, LPWSTR *);
	void *get_IsUserInitiated;
	void *get_IsRedirected;
	void *get_RequestHeaders;
	void *get_Cancel;
	HRESULT(STDMETHODCALLTYPE *put_Cancel)(ICoreWebView2NavigationStartingEventArgs *, BOOL);
	void *get_NavigationId;
} ICoreWebView2NavigationStartingEventArgsVtbl;

struct ICoreWebView2NavigationStartingEventArgs {
	ICoreWebView2NavigationStartingEventArgsVtbl *lpVtbl;
};

typedef HRESULT(STDMETHODCALLTYPE *CreateCoreWebView2EnvironmentWithOptionsFn)(
	PCWSTR browserExecutableFolder,
	PCWSTR userDataFolder,
	ICoreWebView2EnvironmentOptions *options,
	ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *handler);

typedef HRESULT(STDMETHODCALLTYPE *GetAvailableCoreWebView2BrowserVersionStringFn)(
	PCWSTR browserExecutableFolder,
	LPWSTR *versionInfo);

#ifdef __cplusplus
}
#endif

#endif /* VITRA_WEBVIEW2_MIN_H */
