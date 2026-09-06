//go:build linux && cgo && vitra_native

#include "native.h"
#include <stdlib.h>
#include <string.h>

extern void goVitraIdle(void *);
extern void goVitraMessage(char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);
extern void goVitraAction(char *);

static GtkStatusIcon *g_tray = NULL;

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

static void on_action(GtkMenuItem *item, gpointer user_data) {
	(void)item;
	goVitraAction((char *)user_data);
}

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload) {
	VitraWin *w = g_new0(VitraWin, 1);
	w->id = g_strdup(id);
	w->window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_title(GTK_WINDOW(w->window), title);
	gtk_window_set_default_size(GTK_WINDOW(w->window), width, height);

	w->vbox = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
	gtk_container_add(GTK_CONTAINER(w->window), w->vbox);

	w->menubar = gtk_menu_bar_new();
	gtk_box_pack_start(GTK_BOX(w->vbox), w->menubar, FALSE, FALSE, 0);

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
	gtk_box_pack_start(GTK_BOX(w->vbox), GTK_WIDGET(w->view), TRUE, TRUE, 0);
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

void vitra_win_clear_menu(VitraWin *w) {
	if (!w || !w->menubar) {
		return;
	}
	GList *children = gtk_container_get_children(GTK_CONTAINER(w->menubar));
	for (GList *l = children; l != NULL; l = l->next) {
		gtk_widget_destroy(GTK_WIDGET(l->data));
	}
	g_list_free(children);
}

static GtkWidget *find_or_create_menu(GtkWidget *menubar, const char *menu_label) {
	GList *children = gtk_container_get_children(GTK_CONTAINER(menubar));
	for (GList *l = children; l != NULL; l = l->next) {
		GtkWidget *child = GTK_WIDGET(l->data);
		const gchar *lbl = gtk_menu_item_get_label(GTK_MENU_ITEM(child));
		if (lbl && strcmp(lbl, menu_label) == 0) {
			g_list_free(children);
			GtkWidget *submenu = gtk_menu_item_get_submenu(GTK_MENU_ITEM(child));
			if (!submenu) {
				submenu = gtk_menu_new();
				gtk_menu_item_set_submenu(GTK_MENU_ITEM(child), submenu);
			}
			return child;
		}
	}
	g_list_free(children);
	GtkWidget *top = gtk_menu_item_new_with_label(menu_label);
	GtkWidget *submenu = gtk_menu_new();
	gtk_menu_item_set_submenu(GTK_MENU_ITEM(top), submenu);
	gtk_menu_shell_append(GTK_MENU_SHELL(menubar), top);
	gtk_widget_show_all(top);
	return top;
}

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label) {
	if (!w || !w->menubar || !menu_label || !item_id || !item_label) {
		return;
	}
	GtkWidget *top = find_or_create_menu(w->menubar, menu_label);
	GtkWidget *submenu = gtk_menu_item_get_submenu(GTK_MENU_ITEM(top));
	GtkWidget *item = gtk_menu_item_new_with_label(item_label);
	char *id_copy = g_strdup(item_id);
	g_signal_connect_data(item, "activate", G_CALLBACK(on_action), id_copy, (GClosureNotify)g_free, 0);
	gtk_menu_shell_append(GTK_MENU_SHELL(submenu), item);
	gtk_widget_show_all(item);
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

static void on_tray_activate(GtkStatusIcon *icon, gpointer user_data) {
	(void)icon;
	(void)user_data;
	goVitraAction("tray.activate");
}

void vitra_tray_set(const char *tooltip) {
	if (!g_tray) {
		g_tray = gtk_status_icon_new_from_icon_name("application-x-executable");
		g_signal_connect(g_tray, "activate", G_CALLBACK(on_tray_activate), NULL);
	}
	gtk_status_icon_set_visible(g_tray, TRUE);
	if (tooltip) {
		gtk_status_icon_set_tooltip_text(g_tray, tooltip);
	}
}

void vitra_tray_clear(void) {
	if (g_tray) {
		gtk_status_icon_set_visible(g_tray, FALSE);
	}
}
