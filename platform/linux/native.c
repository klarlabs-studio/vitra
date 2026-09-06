//go:build linux && cgo && vitra_native

#include "native.h"
#include <stdlib.h>

extern void goVitraIdle(void *);
extern void goVitraMessage(char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);

static gboolean idle_cb(gpointer data) {
	goVitraIdle(data);
	return G_SOURCE_REMOVE;
}

void vitra_gtk_init(void) { gtk_init(NULL, NULL); }
void vitra_gtk_main(void) { gtk_main(); }
void vitra_gtk_quit(void) { gtk_main_quit(); }
void vitra_idle_add(void *data) { g_idle_add(idle_cb, data); }

static void on_message(WebKitUserContentManager *mgr, WebKitJavascriptResult *js_result, gpointer user_data) {
	(void)mgr;
	JSCValue *value = webkit_javascript_result_get_js_value(js_result);
	gchar *msg = jsc_value_to_string(value);
	goVitraMessage((char *)user_data, msg);
	g_free(msg);
}

static gboolean on_policy(WebKitWebView *view, WebKitPolicyDecision *decision, WebKitPolicyDecisionType type, gpointer user_data) {
	(void)view;
	if (type != WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION) {
		return FALSE;
	}
	WebKitNavigationPolicyDecision *nav = WEBKIT_NAVIGATION_POLICY_DECISION(decision);
	WebKitNavigationAction *action = webkit_navigation_policy_decision_get_navigation_action(nav);
	WebKitURIRequest *req = webkit_navigation_action_get_request(action);
	const gchar *uri = webkit_uri_request_get_uri(req);
	if (goVitraNav((char *)user_data, (char *)uri)) {
		webkit_policy_decision_use(decision);
	} else {
		webkit_policy_decision_ignore(decision);
	}
	return TRUE;
}

static void on_destroy(GtkWidget *widget, gpointer user_data) {
	(void)widget;
	goVitraDestroy((char *)user_data);
}

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload) {
	VitraWin *w = g_new0(VitraWin, 1);
	w->id = g_strdup(id);
	w->window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_title(GTK_WINDOW(w->window), title);
	gtk_window_set_default_size(GTK_WINDOW(w->window), width, height);

	WebKitUserContentManager *ucm = webkit_user_content_manager_new();
	webkit_user_content_manager_register_script_message_handler(ucm, "vitra");
	g_signal_connect(ucm, "script-message-received::vitra", G_CALLBACK(on_message), w->id);

	if (preload && preload[0] != '\0') {
		WebKitUserScript *script = webkit_user_script_new(
			preload,
			WEBKIT_USER_CONTENT_INJECT_TOP_FRAME,
			WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START,
			NULL, NULL);
		webkit_user_content_manager_add_script(ucm, script);
		webkit_user_script_unref(script);
	}

	w->view = WEBKIT_WEB_VIEW(webkit_web_view_new_with_user_content_manager(ucm));
	webkit_settings_set_enable_developer_extras(webkit_web_view_get_settings(w->view), TRUE);
	g_signal_connect(w->view, "decide-policy", G_CALLBACK(on_policy), w->id);
	g_signal_connect(w->window, "destroy", G_CALLBACK(on_destroy), w->id);
	gtk_container_add(GTK_CONTAINER(w->window), GTK_WIDGET(w->view));
	if (uri && uri[0] != '\0') {
		webkit_web_view_load_uri(w->view, uri);
	}
	gtk_widget_show_all(w->window);
	return w;
}

void vitra_win_navigate(VitraWin *w, const char *uri) {
	if (w && w->view && uri) {
		webkit_web_view_load_uri(w->view, uri);
	}
}

void vitra_win_eval(VitraWin *w, const char *js) {
	if (w && w->view && js) {
		webkit_web_view_evaluate_javascript(w->view, js, -1, NULL, NULL, NULL, NULL, NULL);
	}
}

void vitra_win_close(VitraWin *w) {
	if (w && w->window) {
		gtk_widget_destroy(w->window);
	}
}

void vitra_win_free(VitraWin *w) {
	if (!w) {
		return;
	}
	g_free(w->id);
	g_free(w);
}

char *vitra_clip_get(void) {
	return gtk_clipboard_wait_for_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD));
}

void vitra_clip_set(const char *text) {
	gtk_clipboard_set_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), text, -1);
}

char *vitra_open_dialog(void) {
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		"Open File", NULL, GTK_FILE_CHOOSER_ACTION_OPEN,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Open", GTK_RESPONSE_ACCEPT, NULL);
	char *path = NULL;
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
		path = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dialog));
	}
	gtk_widget_destroy(dialog);
	return path;
}
