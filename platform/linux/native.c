//go:build linux && cgo && vitra_native

#include "native.h"
#include <gio/gio.h>
#include <stdlib.h>
#include <string.h>

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xlib.h>
#endif

/* Web inspector for newly created webviews; off unless the app opts in. */
static int vitra_devtools = 0;

void vitra_set_devtools(int enabled) { vitra_devtools = enabled; }

extern void goVitraIdle(unsigned long long);
extern void goVitraMessage(char *, char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);
extern void goVitraAction(char *);
extern void goVitraTrayClick(int, int, int, int, int);
extern void goVitraDrop(char *, char *);

static GtkStatusIcon *g_tray = NULL;
static GtkWidget *g_tray_menu = NULL;

static gboolean idle_cb(gpointer data) {
	goVitraIdle((unsigned long long)(guintptr)data);
	return G_SOURCE_REMOVE;
}

void vitra_gtk_init(const char *prgname) {
	if (prgname != NULL && prgname[0] != '\0') {
		g_set_prgname(prgname);
		gdk_set_program_class(prgname);
	}
	gtk_init(NULL, NULL);
}

const char *vitra_get_prgname(void) {
	return g_get_prgname();
}

void vitra_gtk_main(void) { gtk_main(); }
void vitra_gtk_quit(void) { gtk_main_quit(); }
void vitra_idle_add(unsigned long long id) { g_idle_add(idle_cb, (gpointer)(guintptr)id); }

static void on_message(WebKitUserContentManager *mgr, WebKitJavascriptResult *js_result, gpointer user_data) {
	(void)mgr;
	VitraWin *w = (VitraWin *)user_data;
	JSCValue *value = webkit_javascript_result_get_js_value(js_result);
	gchar *msg = jsc_value_to_string(value);
	/* The page that sent the message: the app checks it on every message. */
	const gchar *sender = w->view ? webkit_web_view_get_uri(w->view) : NULL;
	goVitraMessage(w->id, msg, (char *)(sender ? sender : ""));
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

void vitra_win_set_skip_taskbar(VitraWin *w, int skip) {
	if (!w || !w->window) {
		return;
	}
	gtk_window_set_skip_taskbar_hint(GTK_WINDOW(w->window), skip ? TRUE : FALSE);
	gtk_window_set_skip_pager_hint(GTK_WINDOW(w->window), skip ? TRUE : FALSE);
}

int vitra_win_skips_taskbar(VitraWin *w) {
	if (!w || !w->window) {
		return 0;
	}
	return gtk_window_get_skip_taskbar_hint(GTK_WINDOW(w->window)) && gtk_window_get_skip_pager_hint(GTK_WINDOW(w->window));
}

/* GTK toggles a check item when it is activated. The app owns the checked
 * state (it sets the menu again), so put it back before reporting. */
static void on_check_action(GtkMenuItem *item, gpointer user_data) {
	gboolean checked = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(item), "vitra-checked"));
	gtk_check_menu_item_set_active(GTK_CHECK_MENU_ITEM(item), checked);
	goVitraAction((char *)user_data);
}

/* new_menu_widget builds a menu item (or a separator) with flags applied. */
static GtkWidget *new_menu_widget(const char *item_id, const char *item_label, int flags) {
	if (flags & VITRA_MENU_SEPARATOR) {
		return gtk_separator_menu_item_new();
	}
	GtkWidget *item = NULL;
	GCallback cb = G_CALLBACK(on_action);
	if (flags & VITRA_MENU_CHECKED) {
		item = gtk_check_menu_item_new_with_label(item_label);
		gtk_check_menu_item_set_active(GTK_CHECK_MENU_ITEM(item), TRUE);
		g_object_set_data(G_OBJECT(item), "vitra-checked", GINT_TO_POINTER(1));
		cb = G_CALLBACK(on_check_action);
	} else {
		item = gtk_menu_item_new_with_label(item_label);
	}
	gtk_widget_set_sensitive(item, (flags & VITRA_MENU_DISABLED) ? FALSE : TRUE);
	g_signal_connect_data(item, "activate", cb, g_strdup(item_id), (GClosureNotify)g_free, 0);
	return item;
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
	w->accels = gtk_accel_group_new();
	gtk_window_add_accel_group(GTK_WINDOW(w->window), w->accels);

	WebKitUserContentManager *ucm = webkit_user_content_manager_new();
	webkit_user_content_manager_register_script_message_handler(ucm, "vitra");
	g_signal_connect(ucm, "script-message-received::vitra", G_CALLBACK(on_message), w);

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
	webkit_settings_set_enable_developer_extras(webkit_web_view_get_settings(w->view), vitra_devtools ? TRUE : FALSE);
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
	if (!w || !w->window) {
		return;
	}
	/* Destroy first so menu items disconnect from the accel group while it
	   is still alive; then drop our remaining reference. */
	gtk_widget_destroy(w->window);
	w->window = NULL;
	if (w->accels) {
		g_object_unref(w->accels);
		w->accels = NULL;
	}
}

void vitra_win_free(VitraWin *w) {
	if (!w) {
		return;
	}
	if (w->accels) {
		/* Window may already be destroyed (UI close); drop only our ref. */
		g_object_unref(w->accels);
		w->accels = NULL;
	}
	g_free(w->id);
	g_free(w->icon_path);
	g_free(w);
}

void vitra_win_flush(void) {
	/* Bound the pump: WebKit keeps the queue non-empty while a page loads. */
	for (int i = 0; i < 32 && gtk_events_pending(); i++) {
		gtk_main_iteration_do(FALSE);
	}
}

void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path) {
	if (!w || !w->window) {
		return;
	}
	GtkWindow *win = GTK_WINDOW(w->window);
	if (title) {
		gtk_window_set_title(win, title);
	}
	if (width > 0 && height > 0) {
		gtk_window_set_default_size(win, width, height);
		gtk_window_resize(win, width, height);
		w->req_width = width;
		w->req_height = height;
	}
	w->maximized = maximized ? 1 : 0;
	w->fullscreen = fullscreen ? 1 : 0;
	w->above = above ? 1 : 0;
	w->minimized = minimized ? 1 : 0;
	w->hidden = hidden ? 1 : 0;
	if (maximized) {
		gtk_window_maximize(win);
	} else {
		gtk_window_unmaximize(win);
	}
	if (fullscreen) {
		gtk_window_fullscreen(win);
	} else {
		gtk_window_unfullscreen(win);
	}
	gtk_window_set_keep_above(win, above ? TRUE : FALSE);
	if (minimized) {
		gtk_window_iconify(win);
	} else {
		gtk_window_deiconify(win);
	}
	if (hidden) {
		gtk_widget_hide(w->window);
	} else {
		gtk_widget_show(w->window);
	}
	if (icon_path && icon_path[0] != '\0') {
		GError *err = NULL;
		if (!gtk_window_set_icon_from_file(win, icon_path, &err)) {
			if (err) {
				g_error_free(err);
			}
		} else {
			g_free(w->icon_path);
			w->icon_path = g_strdup(icon_path);
		}
	}
	vitra_win_flush();
}

VitraChrome vitra_win_chrome(VitraWin *w) {
	VitraChrome c;
	memset(&c, 0, sizeof(c));
	c.title = g_strdup("");
	c.icon_path = g_strdup("");
	if (!w || !w->window) {
		return c;
	}
	GtkWindow *win = GTK_WINDOW(w->window);
	const gchar *t = gtk_window_get_title(win);
	g_free(c.title);
	c.title = g_strdup(t ? t : "");
	g_free(c.icon_path);
	c.icon_path = g_strdup(w->icon_path ? w->icon_path : "");
	int dw = 0;
	int dh = 0;
	gtk_window_get_default_size(win, &dw, &dh);
	if (dw > 0 && dh > 0) {
		c.width = dw;
		c.height = dh;
	} else {
		c.width = w->req_width;
		c.height = w->req_height;
	}
	c.maximized = gtk_window_is_maximized(win) ? 1 : 0;
	GdkWindow *gw = gtk_widget_get_window(w->window);
	int gdk_full = 0;
	int gdk_above = 0;
	int gdk_icon = 0;
	if (gw) {
		GdkWindowState st = gdk_window_get_state(gw);
		gdk_full = (st & GDK_WINDOW_STATE_FULLSCREEN) ? 1 : 0;
		gdk_above = (st & GDK_WINDOW_STATE_ABOVE) ? 1 : 0;
		gdk_icon = (st & GDK_WINDOW_STATE_ICONIFIED) ? 1 : 0;
		if (st & GDK_WINDOW_STATE_MAXIMIZED) {
			c.maximized = 1;
		}
	}
	/* Xvfb has no window manager, so GDK may not echo these hints.
	   Fall back to the last request, which was still applied via GTK. */
	if (!c.maximized) {
		c.maximized = w->maximized;
	}
	c.fullscreen = gdk_full || w->fullscreen;
	c.above = gdk_above || w->above;
	c.minimized = gdk_icon || w->minimized;
	c.hidden = (!gtk_widget_get_visible(w->window)) || w->hidden;
	return c;
}

void vitra_win_focus(VitraWin *w) {
	if (!w || !w->window) {
		return;
	}
	GtkWindow *win = GTK_WINDOW(w->window);
	w->hidden = 0;
	w->minimized = 0;
	gtk_window_deiconify(win);
	gtk_widget_show(w->window);
	gtk_window_present(win);
	vitra_win_flush();
}

void vitra_win_blur(VitraWin *w) {
	if (!w || !w->window) {
		return;
	}
	GtkWindow *win = GTK_WINDOW(w->window);
	gtk_window_set_focus(win, NULL);
	GdkWindow *gdk = gtk_widget_get_window(w->window);
	if (gdk) {
		gdk_window_lower(gdk);
	}
	vitra_win_flush();
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
	/* Rebuild the accel group so prior shortcuts cannot fire after clear.
	   Destroying menu items above already disconnected them from the old group. */
	if (w->window && w->accels) {
		gtk_window_remove_accel_group(GTK_WINDOW(w->window), w->accels);
		g_object_unref(w->accels);
		w->accels = gtk_accel_group_new();
		gtk_window_add_accel_group(GTK_WINDOW(w->window), w->accels);
	}
}

static char *normalize_accel(const char *shortcut) {
	if (!shortcut || shortcut[0] == '\0') {
		return NULL;
	}
	if (shortcut[0] == '<') {
		return g_strdup(shortcut);
	}
	GString *out = g_string_new(NULL);
	gchar **parts = g_strsplit(shortcut, "+", -1);
	for (int i = 0; parts[i] != NULL; i++) {
		gchar *p = g_strstrip(parts[i]);
		if (p[0] == '\0') {
			continue;
		}
		gchar *lower = g_ascii_strdown(p, -1);
		if (g_strcmp0(lower, "ctrl") == 0 || g_strcmp0(lower, "control") == 0) {
			g_string_append(out, "<Control>");
		} else if (g_strcmp0(lower, "shift") == 0) {
			g_string_append(out, "<Shift>");
		} else if (g_strcmp0(lower, "alt") == 0 || g_strcmp0(lower, "mod1") == 0) {
			g_string_append(out, "<Alt>");
		} else if (g_strcmp0(lower, "meta") == 0 || g_strcmp0(lower, "super") == 0 || g_strcmp0(lower, "win") == 0) {
			g_string_append(out, "<Super>");
		} else if (strlen(lower) == 1) {
			g_string_append_c(out, lower[0]);
		} else {
			g_string_append(out, lower);
		}
		g_free(lower);
	}
	g_strfreev(parts);
	return g_string_free(out, FALSE);
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

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut, int flags) {
	int separator = (flags & VITRA_MENU_SEPARATOR) != 0;
	if (!w || !w->menubar || !menu_label || (!separator && (!item_id || !item_label))) {
		return;
	}
	GtkWidget *top = find_or_create_menu(w->menubar, menu_label);
	GtkWidget *submenu = gtk_menu_item_get_submenu(GTK_MENU_ITEM(top));
	GtkWidget *item = new_menu_widget(item_id, item_label, flags);
	if (!separator && shortcut && shortcut[0] != '\0' && w->accels) {
		char *accel = normalize_accel(shortcut);
		if (accel) {
			guint key = 0;
			GdkModifierType mods = 0;
			gtk_accelerator_parse(accel, &key, &mods);
			if (key != 0) {
				gtk_widget_add_accelerator(item, "activate", w->accels, key, mods, GTK_ACCEL_VISIBLE);
			}
			g_free(accel);
		}
	}
	gtk_menu_shell_append(GTK_MENU_SHELL(submenu), item);
	gtk_widget_show_all(item);
}

int vitra_win_activate_accel(VitraWin *w, const char *shortcut) {
	if (!w || !w->window || !shortcut || shortcut[0] == '\0') {
		return 0;
	}
	char *accel = normalize_accel(shortcut);
	if (!accel) {
		return 0;
	}
	guint key = 0;
	GdkModifierType mods = 0;
	gtk_accelerator_parse(accel, &key, &mods);
	g_free(accel);
	if (key == 0) {
		return 0;
	}
	gboolean ok = gtk_accel_groups_activate(G_OBJECT(w->window), key, mods);
	vitra_win_flush();
	return ok ? 1 : 0;
}

char *vitra_clip_get(void) {
	return gtk_clipboard_wait_for_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD));
}

void vitra_clip_set(const char *text) {
	gtk_clipboard_set_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), text, -1);
}

static void vitra_apply_file_filters(GtkFileChooser *chooser, const char *filters) {
	if (!filters || !filters[0]) {
		return;
	}
	char *copy = g_strdup(filters);
	char *saveptr = NULL;
	for (char *group = strtok_r(copy, ";", &saveptr); group; group = strtok_r(NULL, ";", &saveptr)) {
		char *colon = strchr(group, ':');
		if (!colon) {
			continue;
		}
		*colon = '\0';
		const char *name = group;
		char *exts = colon + 1;
		if (!exts[0]) {
			continue;
		}
		GtkFileFilter *filter = gtk_file_filter_new();
		gtk_file_filter_set_name(filter, name[0] ? name : exts);
		char *esave = NULL;
		for (char *ext = strtok_r(exts, ",", &esave); ext; ext = strtok_r(NULL, ",", &esave)) {
			if (!ext[0]) {
				continue;
			}
			char *pattern = g_strdup_printf("*.%s", ext);
			gtk_file_filter_add_pattern(filter, pattern);
			g_free(pattern);
		}
		gtk_file_chooser_add_filter(chooser, filter);
	}
	g_free(copy);
	GtkFileFilter *all = gtk_file_filter_new();
	gtk_file_filter_set_name(all, "All Files");
	gtk_file_filter_add_pattern(all, "*");
	gtk_file_chooser_add_filter(chooser, all);
}

static void vitra_apply_dialog_defaults(GtkFileChooser *chooser, const char *title, const char *default_path, GtkWidget *dialog) {
	if (title && title[0]) {
		gtk_window_set_title(GTK_WINDOW(dialog), title);
	}
	if (default_path && default_path[0]) {
		gtk_file_chooser_set_filename(chooser, default_path);
		gtk_file_chooser_set_current_folder(chooser, default_path);
	}
}

char *vitra_open_dialog(const char *title, const char *default_path, const char *filters) {
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		title && title[0] ? title : "Open File", NULL, GTK_FILE_CHOOSER_ACTION_OPEN,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Open", GTK_RESPONSE_ACCEPT, NULL);
	GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
	vitra_apply_dialog_defaults(chooser, title, default_path, dialog);
	vitra_apply_file_filters(chooser, filters);
	char *path = NULL;
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
		path = gtk_file_chooser_get_filename(chooser);
	}
	gtk_widget_destroy(dialog);
	return path;
}

/* vitra_open_dialog_multi runs a multi-select open dialog. It returns the
 * selected paths as one g_malloc'd buffer of NUL-terminated paths (NUL is the
 * only byte a path cannot contain), its byte length in *out_len, or NULL when
 * the user cancels or picks nothing. Free it with g_free. */
char *vitra_open_dialog_multi(const char *title, const char *default_path, const char *filters, int *out_len) {
	*out_len = 0;
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		title && title[0] ? title : "Open Files", NULL, GTK_FILE_CHOOSER_ACTION_OPEN,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Open", GTK_RESPONSE_ACCEPT, NULL);
	GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
	gtk_file_chooser_set_select_multiple(chooser, TRUE);
	vitra_apply_dialog_defaults(chooser, title, default_path, dialog);
	vitra_apply_file_filters(chooser, filters);
	GString *buf = NULL;
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
		GSList *names = gtk_file_chooser_get_filenames(chooser);
		for (GSList *it = names; it; it = it->next) {
			const char *name = (const char *)it->data;
			if (!name || !name[0]) {
				continue;
			}
			if (!buf) {
				buf = g_string_new(NULL);
			}
			g_string_append_len(buf, name, (gssize)strlen(name) + 1);
		}
		g_slist_free_full(names, g_free);
	}
	gtk_widget_destroy(dialog);
	if (!buf) {
		return NULL;
	}
	if (buf->len > G_MAXINT) {
		g_string_free(buf, TRUE);
		return NULL;
	}
	*out_len = (int)buf->len;
	return g_string_free(buf, FALSE);
}

char *vitra_open_directory_dialog(const char *title, const char *default_path) {
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		title && title[0] ? title : "Open Folder", NULL, GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Open", GTK_RESPONSE_ACCEPT, NULL);
	GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
	vitra_apply_dialog_defaults(chooser, title, default_path, dialog);
	char *path = NULL;
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
		path = gtk_file_chooser_get_filename(chooser);
	}
	gtk_widget_destroy(dialog);
	return path;
}

char *vitra_save_dialog(const char *title, const char *default_path, const char *filters) {
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		title && title[0] ? title : "Save File", NULL, GTK_FILE_CHOOSER_ACTION_SAVE,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Save", GTK_RESPONSE_ACCEPT, NULL);
	GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
	gtk_file_chooser_set_do_overwrite_confirmation(chooser, TRUE);
	vitra_apply_dialog_defaults(chooser, title, default_path, dialog);
	vitra_apply_file_filters(chooser, filters);
	char *path = NULL;
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
		path = gtk_file_chooser_get_filename(chooser);
	}
	gtk_widget_destroy(dialog);
	return path;
}

int vitra_message_dialog(const char *title, const char *message, int confirm) {
	GtkMessageType type = confirm ? GTK_MESSAGE_QUESTION : GTK_MESSAGE_INFO;
	GtkButtonsType buttons = confirm ? GTK_BUTTONS_YES_NO : GTK_BUTTONS_OK;
	GtkWidget *dialog = gtk_message_dialog_new(
		NULL, GTK_DIALOG_MODAL, type, buttons, "%s", message ? message : "");
	if (title && title[0]) {
		gtk_window_set_title(GTK_WINDOW(dialog), title);
	}
	gint response = gtk_dialog_run(GTK_DIALOG(dialog));
	gtk_widget_destroy(dialog);
	if (confirm) {
		return response == GTK_RESPONSE_YES ? 1 : 0;
	}
	return 1;
}

int vitra_show_notification(const char *title, const char *body) {
	GError *err = NULL;
	GDBusProxy *proxy = g_dbus_proxy_new_for_bus_sync(
		G_BUS_TYPE_SESSION,
		G_DBUS_PROXY_FLAGS_DO_NOT_LOAD_PROPERTIES,
		NULL,
		"org.freedesktop.Notifications",
		"/org/freedesktop/Notifications",
		"org.freedesktop.Notifications",
		NULL,
		&err);
	if (!proxy) {
		if (err) {
			g_error_free(err);
		}
		return 0;
	}
	const char *app = vitra_get_prgname();
	if (!app || !app[0]) {
		app = "vitra";
	}
	GVariantBuilder actions;
	GVariantBuilder hints;
	g_variant_builder_init(&actions, G_VARIANT_TYPE("as"));
	g_variant_builder_init(&hints, G_VARIANT_TYPE("a{sv}"));
	GVariant *result = g_dbus_proxy_call_sync(
		proxy,
		"Notify",
		g_variant_new("(susssasa{sv}i)",
			app,
			(guint32)0,
			"",
			title ? title : "",
			body ? body : "",
			&actions,
			&hints,
			-1),
		G_DBUS_CALL_FLAGS_NONE,
		-1,
		NULL,
		&err);
	g_object_unref(proxy);
	if (!result) {
		if (err) {
			g_error_free(err);
		}
		return 0;
	}
	g_variant_unref(result);
	return 1;
}

/* GtkStatusIcon (XEmbed) is the fallback tray, used only when no
 * StatusNotifierWatcher is on the session bus (see the StatusNotifierItem
 * section below). GTK3 has no tray API that is not deprecated, so keep it
 * without warning every build about it. */
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
static int g_tray_click_activates = 0;

int vitra_tray_anchor(int *x, int *y, int *w, int *h) {
	GdkRectangle area;
	if (!g_tray || !gtk_status_icon_get_visible(g_tray) || !gtk_status_icon_get_geometry(g_tray, NULL, &area, NULL)) {
		return 0;
	}
	*x = area.x;
	*y = area.y;
	*w = area.width;
	*h = area.height;
	return 1;
}

static void on_tray_activate(GtkStatusIcon *icon, gpointer user_data) {
	(void)icon;
	(void)user_data;
	if (!g_tray_click_activates) {
		goVitraAction("tray.activate");
		return;
	}
	int x = 0, y = 0, w = 0, h = 0;
	int has = vitra_tray_anchor(&x, &y, &w, &h);
	goVitraTrayClick(x, y, w, h, has);
}

static void on_tray_popup(GtkStatusIcon *icon, guint button, guint32 activate_time, gpointer user_data) {
	(void)user_data;
	if (!g_tray_menu) {
		return;
	}
	gtk_widget_show_all(g_tray_menu);
	gtk_menu_popup(GTK_MENU(g_tray_menu), NULL, NULL, gtk_status_icon_position_menu, icon, button, activate_time);
}

/* vitra_tray_set shows the GtkStatusIcon. rgba is width x height
 * non-premultiplied RGBA pixels, or NULL for the default icon. The status
 * icon has no text: title is its accessible title only. */
void vitra_tray_set(const char *tooltip, const char *title, const unsigned char *rgba, int width, int height, int click_activates) {
	g_tray_click_activates = click_activates;
	if (!g_tray) {
		g_tray = gtk_status_icon_new_from_icon_name("application-x-executable");
		g_signal_connect(g_tray, "activate", G_CALLBACK(on_tray_activate), NULL);
		g_signal_connect(g_tray, "popup-menu", G_CALLBACK(on_tray_popup), NULL);
	}
	if (rgba && width > 0 && height > 0) {
		GBytes *pixels = g_bytes_new(rgba, (gsize)width * (gsize)height * 4);
		GdkPixbuf *pb = gdk_pixbuf_new_from_bytes(pixels, GDK_COLORSPACE_RGB, TRUE, 8, width, height, width * 4);
		gtk_status_icon_set_from_pixbuf(g_tray, pb);
		g_object_unref(pb);
		g_bytes_unref(pixels);
	} else {
		gtk_status_icon_set_from_icon_name(g_tray, "application-x-executable");
	}
	gtk_status_icon_set_title(g_tray, title ? title : "");
	gtk_status_icon_set_visible(g_tray, TRUE);
	if (tooltip) {
		gtk_status_icon_set_tooltip_text(g_tray, tooltip);
	}
}

void vitra_tray_clear_menu(void) {
	if (g_tray_menu) {
		gtk_widget_destroy(g_tray_menu);
		g_tray_menu = NULL;
	}
}

void vitra_tray_add_menu_item(const char *item_id, const char *item_label, int flags) {
	if (!(flags & VITRA_MENU_SEPARATOR) && (!item_id || !item_label)) {
		return;
	}
	if (!g_tray_menu) {
		g_tray_menu = gtk_menu_new();
	}
	GtkWidget *item = new_menu_widget(item_id, item_label, flags);
	gtk_menu_shell_append(GTK_MENU_SHELL(g_tray_menu), item);
	gtk_widget_show_all(item);
}

void vitra_tray_clear(void) {
	vitra_tray_clear_menu();
	if (g_tray) {
		gtk_status_icon_set_visible(g_tray, FALSE);
	}
}
#pragma GCC diagnostic pop

/* ===================================================================== *
 * Linux tray: StatusNotifierItem + com.canonical.dbusmenu over GDBus.
 *
 * KDE Plasma, GNOME with the AppIndicator extension, and most other current
 * panels (XFCE, Cinnamon, MATE, LXQt, Budgie, waybar, …) show tray items
 * through the StatusNotifierItem protocol rather than XEmbed, and on Wayland
 * XEmbed does not exist. This is a plain-GDBus implementation (no
 * libayatana-appindicator): it owns org.kde.StatusNotifierItem-PID-N,
 * exports the item at /StatusNotifierItem and its menu at /MenuBar, and
 * registers with org.kde.StatusNotifierWatcher. Without a watcher, the
 * caller (webview.go) falls back to GtkStatusIcon above.
 *
 * Both objects are served on a dedicated thread with its own main context,
 * so the tray answers the panel even while the GTK thread is busy. State is
 * shared with the Go-facing setters under g_sni_mu. Menu clicks map the
 * dbusmenu item id back to the action id the app set; a D-Bus peer can
 * never name an action id itself.
 * ===================================================================== */

#include <unistd.h>

#define VITRA_SNI_WATCHER "org.kde.StatusNotifierWatcher"
#define VITRA_SNI_WATCHER_PATH "/StatusNotifierWatcher"
#define VITRA_SNI_PATH "/StatusNotifierItem"
#define VITRA_SNI_IFACE "org.kde.StatusNotifierItem"
#define VITRA_SNI_MENU_PATH "/MenuBar"
#define VITRA_DBUSMENU_IFACE "com.canonical.dbusmenu"
#define VITRA_SNI_ICON "application-x-executable"
#define VITRA_SNI_CALL_TIMEOUT_MS 5000

static const char vitra_sni_xml[] =
	"<node>"
	" <interface name='org.kde.StatusNotifierItem'>"
	"  <property name='Category' type='s' access='read'/>"
	"  <property name='Id' type='s' access='read'/>"
	"  <property name='Title' type='s' access='read'/>"
	"  <property name='Status' type='s' access='read'/>"
	"  <property name='WindowId' type='i' access='read'/>"
	"  <property name='IconName' type='s' access='read'/>"
	"  <property name='IconPixmap' type='a(iiay)' access='read'/>"
	"  <property name='OverlayIconName' type='s' access='read'/>"
	"  <property name='AttentionIconName' type='s' access='read'/>"
	"  <property name='ToolTip' type='(sa(iiay)ss)' access='read'/>"
	"  <property name='ItemIsMenu' type='b' access='read'/>"
	"  <property name='Menu' type='o' access='read'/>"
	"  <property name='XAyatanaLabel' type='s' access='read'/>"
	"  <property name='XAyatanaLabelGuide' type='s' access='read'/>"
	"  <method name='ContextMenu'><arg name='x' type='i' direction='in'/><arg name='y' type='i' direction='in'/></method>"
	"  <method name='Activate'><arg name='x' type='i' direction='in'/><arg name='y' type='i' direction='in'/></method>"
	"  <method name='SecondaryActivate'><arg name='x' type='i' direction='in'/><arg name='y' type='i' direction='in'/></method>"
	"  <method name='Scroll'><arg name='delta' type='i' direction='in'/><arg name='orientation' type='s' direction='in'/></method>"
	"  <signal name='NewTitle'/>"
	"  <signal name='NewIcon'/>"
	"  <signal name='NewAttentionIcon'/>"
	"  <signal name='NewOverlayIcon'/>"
	"  <signal name='NewToolTip'/>"
	"  <signal name='NewStatus'><arg name='status' type='s'/></signal>"
	"  <signal name='XAyatanaNewLabel'><arg name='label' type='s'/><arg name='guide' type='s'/></signal>"
	" </interface>"
	" <interface name='com.canonical.dbusmenu'>"
	"  <property name='Version' type='u' access='read'/>"
	"  <property name='TextDirection' type='s' access='read'/>"
	"  <property name='Status' type='s' access='read'/>"
	"  <property name='IconThemePath' type='as' access='read'/>"
	"  <method name='GetLayout'>"
	"   <arg name='parentId' type='i' direction='in'/><arg name='recursionDepth' type='i' direction='in'/>"
	"   <arg name='propertyNames' type='as' direction='in'/>"
	"   <arg name='revision' type='u' direction='out'/><arg name='layout' type='(ia{sv}av)' direction='out'/>"
	"  </method>"
	"  <method name='GetGroupProperties'>"
	"   <arg name='ids' type='ai' direction='in'/><arg name='propertyNames' type='as' direction='in'/>"
	"   <arg name='properties' type='a(ia{sv})' direction='out'/>"
	"  </method>"
	"  <method name='GetProperty'>"
	"   <arg name='id' type='i' direction='in'/><arg name='name' type='s' direction='in'/>"
	"   <arg name='value' type='v' direction='out'/>"
	"  </method>"
	"  <method name='Event'>"
	"   <arg name='id' type='i' direction='in'/><arg name='eventId' type='s' direction='in'/>"
	"   <arg name='data' type='v' direction='in'/><arg name='timestamp' type='u' direction='in'/>"
	"  </method>"
	"  <method name='EventGroup'>"
	"   <arg name='events' type='a(isvu)' direction='in'/><arg name='idErrors' type='ai' direction='out'/>"
	"  </method>"
	"  <method name='AboutToShow'>"
	"   <arg name='id' type='i' direction='in'/><arg name='needUpdate' type='b' direction='out'/>"
	"  </method>"
	"  <method name='AboutToShowGroup'>"
	"   <arg name='ids' type='ai' direction='in'/>"
	"   <arg name='updatesNeeded' type='ai' direction='out'/><arg name='idErrors' type='ai' direction='out'/>"
	"  </method>"
	"  <signal name='ItemsPropertiesUpdated'><arg type='a(ia{sv})'/><arg type='a(ias)'/></signal>"
	"  <signal name='LayoutUpdated'><arg name='revision' type='u'/><arg name='parent' type='i'/></signal>"
	"  <signal name='ItemActivationRequested'><arg name='id' type='i'/><arg name='timestamp' type='u'/></signal>"
	" </interface>"
	"</node>";

static GMutex g_sni_mu;
static GCond g_sni_cond;
static GThread *g_sni_thread = NULL;
static int g_sni_ready = 0;       /* 1 objects exported, -1 failed (under g_sni_mu) */
static int g_sni_active = 0;      /* item registered with the watcher */
static int g_sni_watcher_lost = 0;
static char *g_sni_name = NULL;   /* our org.kde.StatusNotifierItem-PID-N name */
static char *g_sni_tooltip = NULL; /* tooltip text */
static char *g_sni_title = NULL;   /* status text next to the icon */
static GVariant *g_sni_pixmap = NULL; /* a(iiay) IconPixmap; empty for the default icon */
static GPtrArray *g_sni_ids = NULL;    /* menu action ids; dbusmenu id = index + 1 */
static GPtrArray *g_sni_labels = NULL; /* menu labels, parallel to g_sni_ids */
static GArray *g_sni_flags = NULL;     /* VITRA_MENU_* flags, parallel to g_sni_ids */
static int g_sni_click_activates = 0;  /* Activate is a tray click, not "tray.activate" */
static int g_sni_has_point = 0;        /* g_sni_x/y hold the last Activate position */
static int g_sni_x = 0;
static int g_sni_y = 0;
static guint32 g_sni_revision = 1;
static guint g_sni_seq = 0;

static GDBusConnection *sni_bus(void) {
	/* Process-wide shared connection: GIO keeps it alive; held, never freed. */
	static GDBusConnection *conn = NULL;
	if (conn && !g_dbus_connection_is_closed(conn)) {
		return conn;
	}
	GError *err = NULL;
	GDBusConnection *c = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &err);
	if (!c) {
		if (err) {
			g_error_free(err);
		}
		return NULL;
	}
	conn = c;
	return conn;
}

static GVariant *sni_call(GDBusConnection *conn, const char *dest, const char *path, const char *iface,
	const char *method, GVariant *args, const GVariantType *reply) {
	return g_dbus_connection_call_sync(conn, dest, path, iface, method, args, reply,
		G_DBUS_CALL_FLAGS_NONE, VITRA_SNI_CALL_TIMEOUT_MS, NULL, NULL);
}

/* vitra_sni_available reports whether a StatusNotifierWatcher with a
 * registered host (a panel that will display items) is on the session bus. */
int vitra_sni_available(void) {
	GDBusConnection *conn = sni_bus();
	if (!conn) {
		return 0;
	}
	GVariant *ret = sni_call(conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
		"NameHasOwner", g_variant_new("(s)", VITRA_SNI_WATCHER), G_VARIANT_TYPE("(b)"));
	if (!ret) {
		return 0;
	}
	gboolean owned = FALSE;
	g_variant_get(ret, "(b)", &owned);
	g_variant_unref(ret);
	if (!owned) {
		return 0;
	}
	/* A watcher with no host shows nothing; treat it as absent. Watchers
	 * that do not expose the property are trusted. */
	ret = sni_call(conn, VITRA_SNI_WATCHER, VITRA_SNI_WATCHER_PATH, "org.freedesktop.DBus.Properties", "Get",
		g_variant_new("(ss)", VITRA_SNI_WATCHER, "IsStatusNotifierHostRegistered"), G_VARIANT_TYPE("(v)"));
	if (!ret) {
		return 1;
	}
	GVariant *v = NULL;
	g_variant_get(ret, "(v)", &v);
	int hosted = !(v && g_variant_is_of_type(v, G_VARIANT_TYPE_BOOLEAN) && !g_variant_get_boolean(v));
	if (v) {
		g_variant_unref(v);
	}
	g_variant_unref(ret);
	return hosted;
}

/* dbusmenu labels treat '_' as a mnemonic marker; Vitra labels are literal. */
static GVariant *sni_label(const char *label) {
	GString *s = g_string_new(NULL);
	for (const char *p = label ? label : ""; *p; p++) {
		if (*p == '_') {
			g_string_append_c(s, '_');
		}
		g_string_append_c(s, *p);
	}
	return g_variant_new_take_string(g_string_free(s, FALSE));
}

static int sni_wants(const gchar *const *names, const char *prop) {
	if (!names || !names[0]) {
		return 1;
	}
	return g_strv_contains(names, prop);
}

static int sni_valid_id(gint32 id) {
	return id == 0 || (g_sni_ids && id > 0 && (guint)id <= g_sni_ids->len);
}

/* Callers hold g_sni_mu and pass a valid id. */
static GVariant *sni_item_props(gint32 id, const gchar *const *names) {
	GVariantBuilder b;
	g_variant_builder_init(&b, G_VARIANT_TYPE_VARDICT);
	if (id == 0) {
		if (sni_wants(names, "children-display")) {
			g_variant_builder_add(&b, "{sv}", "children-display", g_variant_new_string("submenu"));
		}
		return g_variant_builder_end(&b);
	}
	int flags = g_array_index(g_sni_flags, int, id - 1);
	if (flags & VITRA_MENU_SEPARATOR) {
		if (sni_wants(names, "type")) {
			g_variant_builder_add(&b, "{sv}", "type", g_variant_new_string("separator"));
		}
	} else if (sni_wants(names, "label")) {
		g_variant_builder_add(&b, "{sv}", "label", sni_label(g_ptr_array_index(g_sni_labels, id - 1)));
	}
	if (sni_wants(names, "enabled")) {
		g_variant_builder_add(&b, "{sv}", "enabled", g_variant_new_boolean((flags & VITRA_MENU_DISABLED) ? FALSE : TRUE));
	}
	if (flags & VITRA_MENU_CHECKED) {
		if (sni_wants(names, "toggle-type")) {
			g_variant_builder_add(&b, "{sv}", "toggle-type", g_variant_new_string("checkmark"));
		}
		if (sni_wants(names, "toggle-state")) {
			g_variant_builder_add(&b, "{sv}", "toggle-state", g_variant_new_int32(1));
		}
	}
	if (sni_wants(names, "visible")) {
		g_variant_builder_add(&b, "{sv}", "visible", g_variant_new_boolean(TRUE));
	}
	return g_variant_builder_end(&b);
}

static GVariant *sni_layout_node(gint32 id, gint32 depth, const gchar *const *names) {
	GVariantBuilder children;
	g_variant_builder_init(&children, G_VARIANT_TYPE("av"));
	if (id == 0 && depth != 0 && g_sni_ids) {
		for (guint i = 0; i < g_sni_ids->len; i++) {
			g_variant_builder_add(&children, "v", sni_layout_node((gint32)i + 1, depth - 1, names));
		}
	}
	return g_variant_new("(i@a{sv}av)", id, sni_item_props(id, names), &children);
}

/* sni_event handles one dbusmenu event; returns 0 for an unknown id. */
static int sni_event(gint32 id, const char *event_id) {
	g_mutex_lock(&g_sni_mu);
	if (!sni_valid_id(id)) {
		g_mutex_unlock(&g_sni_mu);
		return 0;
	}
	char *action = NULL;
	/* Separators and disabled items never fire, whatever the panel sends. */
	if (id > 0 && g_strcmp0(event_id, "clicked") == 0 &&
		!(g_array_index(g_sni_flags, int, id - 1) & (VITRA_MENU_SEPARATOR | VITRA_MENU_DISABLED))) {
		action = g_strdup(g_ptr_array_index(g_sni_ids, id - 1));
	}
	g_mutex_unlock(&g_sni_mu);
	if (action) {
		goVitraAction(action);
		g_free(action);
	}
	return 1;
}

static void sni_menu_call(GDBusMethodInvocation *inv, const gchar *method, GVariant *params) {
	if (g_strcmp0(method, "GetLayout") == 0) {
		gint32 parent = 0;
		gint32 depth = -1;
		const gchar **names = NULL;
		g_variant_get(params, "(ii^a&s)", &parent, &depth, &names);
		g_mutex_lock(&g_sni_mu);
		if (!sni_valid_id(parent)) {
			g_mutex_unlock(&g_sni_mu);
			g_free(names);
			g_dbus_method_invocation_return_dbus_error(inv, "org.freedesktop.DBus.Error.InvalidArgs", "unknown menu item");
			return;
		}
		GVariant *node = sni_layout_node(parent, depth, names);
		guint32 rev = g_sni_revision;
		g_mutex_unlock(&g_sni_mu);
		g_free(names);
		g_dbus_method_invocation_return_value(inv, g_variant_new("(u@(ia{sv}av))", rev, node));
	} else if (g_strcmp0(method, "GetGroupProperties") == 0) {
		GVariant *ids = NULL;
		const gchar **names = NULL;
		g_variant_get(params, "(@ai^a&s)", &ids, &names);
		GVariantBuilder out;
		g_variant_builder_init(&out, G_VARIANT_TYPE("a(ia{sv})"));
		g_mutex_lock(&g_sni_mu);
		gsize n = g_variant_n_children(ids);
		if (n == 0) {
			guint count = g_sni_ids ? g_sni_ids->len : 0;
			for (guint i = 0; i <= count; i++) {
				g_variant_builder_add(&out, "(i@a{sv})", (gint32)i, sni_item_props((gint32)i, names));
			}
		}
		for (gsize i = 0; i < n; i++) {
			gint32 id = 0;
			g_variant_get_child(ids, i, "i", &id);
			if (sni_valid_id(id)) {
				g_variant_builder_add(&out, "(i@a{sv})", id, sni_item_props(id, names));
			}
		}
		g_mutex_unlock(&g_sni_mu);
		g_variant_unref(ids);
		g_free(names);
		g_dbus_method_invocation_return_value(inv, g_variant_new("(a(ia{sv}))", &out));
	} else if (g_strcmp0(method, "GetProperty") == 0) {
		gint32 id = 0;
		const gchar *name = NULL;
		g_variant_get(params, "(i&s)", &id, &name);
		const gchar *only[] = {name, NULL};
		GVariant *value = NULL;
		g_mutex_lock(&g_sni_mu);
		if (sni_valid_id(id)) {
			GVariant *props = sni_item_props(id, only);
			value = g_variant_lookup_value(props, name, NULL);
			g_variant_unref(g_variant_ref_sink(props));
		}
		g_mutex_unlock(&g_sni_mu);
		if (!value) {
			g_dbus_method_invocation_return_dbus_error(inv, "org.freedesktop.DBus.Error.InvalidArgs", "unknown menu item or property");
			return;
		}
		g_dbus_method_invocation_return_value(inv, g_variant_new("(v)", value));
		g_variant_unref(value);
	} else if (g_strcmp0(method, "Event") == 0) {
		gint32 id = 0;
		const gchar *event_id = NULL;
		g_variant_get(params, "(i&svu)", &id, &event_id, NULL, NULL);
		if (!sni_event(id, event_id)) {
			g_dbus_method_invocation_return_dbus_error(inv, "org.freedesktop.DBus.Error.InvalidArgs", "unknown menu item");
			return;
		}
		g_dbus_method_invocation_return_value(inv, NULL);
	} else if (g_strcmp0(method, "EventGroup") == 0) {
		GVariantIter *it = NULL;
		g_variant_get(params, "(a(isvu))", &it);
		GVariantBuilder errs;
		g_variant_builder_init(&errs, G_VARIANT_TYPE("ai"));
		gint32 id = 0;
		const gchar *event_id = NULL;
		while (g_variant_iter_loop(it, "(i&svu)", &id, &event_id, NULL, NULL)) {
			if (!sni_event(id, event_id)) {
				g_variant_builder_add(&errs, "i", id);
			}
		}
		g_variant_iter_free(it);
		g_dbus_method_invocation_return_value(inv, g_variant_new("(ai)", &errs));
	} else if (g_strcmp0(method, "AboutToShow") == 0) {
		g_dbus_method_invocation_return_value(inv, g_variant_new("(b)", FALSE));
	} else if (g_strcmp0(method, "AboutToShowGroup") == 0) {
		GVariantBuilder none;
		GVariantBuilder errs;
		g_variant_builder_init(&none, G_VARIANT_TYPE("ai"));
		g_variant_builder_init(&errs, G_VARIANT_TYPE("ai"));
		g_dbus_method_invocation_return_value(inv, g_variant_new("(aiai)", &none, &errs));
	} else {
		g_dbus_method_invocation_return_dbus_error(inv, "org.freedesktop.DBus.Error.UnknownMethod", method);
	}
}

static void sni_method_call(GDBusConnection *conn, const gchar *sender, const gchar *path, const gchar *iface,
	const gchar *method, GVariant *params, GDBusMethodInvocation *inv, gpointer user_data) {
	(void)conn;
	(void)sender;
	(void)path;
	(void)user_data;
	if (g_strcmp0(iface, VITRA_DBUSMENU_IFACE) == 0) {
		sni_menu_call(inv, method, params);
		return;
	}
	if (g_strcmp0(method, "Activate") == 0) {
		gint32 x = 0, y = 0;
		g_variant_get(params, "(ii)", &x, &y);
		/* Panels that cannot tell where the icon is (some Wayland ones) send 0,0. */
		int has = x != 0 || y != 0;
		g_mutex_lock(&g_sni_mu);
		int clicks = g_sni_click_activates;
		if (has) {
			g_sni_has_point = 1;
			g_sni_x = x;
			g_sni_y = y;
		}
		g_mutex_unlock(&g_sni_mu);
		if (clicks) {
			goVitraTrayClick(x, y, 0, 0, has);
		} else {
			goVitraAction("tray.activate");
		}
	}
	/* ContextMenu: the host shows our Menu itself. SecondaryActivate, Scroll:
	 * no Vitra event. */
	g_dbus_method_invocation_return_value(inv, NULL);
}

static GVariant *sni_get_property(GDBusConnection *conn, const gchar *sender, const gchar *path,
	const gchar *iface, const gchar *prop, GError **error, gpointer user_data) {
	(void)conn;
	(void)sender;
	(void)path;
	(void)user_data;
	if (g_strcmp0(iface, VITRA_DBUSMENU_IFACE) == 0) {
		if (g_strcmp0(prop, "Version") == 0) {
			return g_variant_new_uint32(3);
		}
		if (g_strcmp0(prop, "TextDirection") == 0) {
			return g_variant_new_string("ltr");
		}
		if (g_strcmp0(prop, "Status") == 0) {
			return g_variant_new_string("normal");
		}
		if (g_strcmp0(prop, "IconThemePath") == 0) {
			return g_variant_new_strv(NULL, 0);
		}
	} else {
		const char *app = g_get_prgname();
		if (!app || !app[0]) {
			app = "vitra";
		}
		GVariant *v = NULL;
		g_mutex_lock(&g_sni_mu);
		const char *tooltip = g_sni_tooltip && g_sni_tooltip[0] ? g_sni_tooltip : NULL;
		const char *label = g_sni_title ? g_sni_title : "";
		/* Title names the item; panels without labels show it on hover. */
		const char *title = label[0] ? label : tooltip ? tooltip : app;
		int has_pixmap = g_sni_pixmap && g_variant_n_children(g_sni_pixmap) > 0;
		if (g_strcmp0(prop, "Category") == 0) {
			v = g_variant_new_string("ApplicationStatus");
		} else if (g_strcmp0(prop, "Id") == 0) {
			v = g_variant_new_string(app);
		} else if (g_strcmp0(prop, "Title") == 0) {
			v = g_variant_new_string(title);
		} else if (g_strcmp0(prop, "Status") == 0) {
			v = g_variant_new_string(g_sni_active ? "Active" : "Passive");
		} else if (g_strcmp0(prop, "WindowId") == 0) {
			v = g_variant_new_int32(0);
		} else if (g_strcmp0(prop, "IconName") == 0) {
			/* Hosts prefer IconName over IconPixmap: leave it empty for an app icon. */
			v = g_variant_new_string(has_pixmap ? "" : VITRA_SNI_ICON);
		} else if (g_strcmp0(prop, "IconPixmap") == 0) {
			v = has_pixmap ? g_variant_ref(g_sni_pixmap) : g_variant_new_array(G_VARIANT_TYPE("(iiay)"), NULL, 0);
		} else if (g_strcmp0(prop, "XAyatanaLabel") == 0 || g_strcmp0(prop, "XAyatanaLabelGuide") == 0) {
			v = g_variant_new_string(label);
		} else if (g_strcmp0(prop, "OverlayIconName") == 0 || g_strcmp0(prop, "AttentionIconName") == 0) {
			v = g_variant_new_string("");
		} else if (g_strcmp0(prop, "ToolTip") == 0) {
			v = g_variant_new("(s@a(iiay)ss)", VITRA_SNI_ICON, g_variant_new_array(G_VARIANT_TYPE("(iiay)"), NULL, 0),
				tooltip ? tooltip : title, "");
		} else if (g_strcmp0(prop, "ItemIsMenu") == 0) {
			v = g_variant_new_boolean(FALSE);
		} else if (g_strcmp0(prop, "Menu") == 0) {
			v = g_variant_new_object_path(VITRA_SNI_MENU_PATH);
		}
		g_mutex_unlock(&g_sni_mu);
		if (v) {
			return v;
		}
	}
	g_set_error(error, G_DBUS_ERROR, G_DBUS_ERROR_UNKNOWN_PROPERTY, "unknown property %s", prop);
	return NULL;
}

static const GDBusInterfaceVTable sni_vtable = {sni_method_call, sni_get_property, NULL, {0}};

static void sni_register_with_watcher(GDBusConnection *conn, const char *name) {
	GVariant *ret = sni_call(conn, VITRA_SNI_WATCHER, VITRA_SNI_WATCHER_PATH, VITRA_SNI_WATCHER,
		"RegisterStatusNotifierItem", g_variant_new("(s)", name), NULL);
	if (ret) {
		g_variant_unref(ret);
	}
}

/* A restarted panel (new watcher) forgets items: register again. */
static void sni_watcher_appeared(GDBusConnection *conn, const gchar *name, const gchar *owner, gpointer user_data) {
	(void)name;
	(void)owner;
	(void)user_data;
	g_mutex_lock(&g_sni_mu);
	char *item = g_sni_watcher_lost && g_sni_active ? g_strdup(g_sni_name) : NULL;
	g_sni_watcher_lost = 0;
	g_mutex_unlock(&g_sni_mu);
	if (item) {
		sni_register_with_watcher(conn, item);
		g_free(item);
	}
}

static void sni_watcher_vanished(GDBusConnection *conn, const gchar *name, gpointer user_data) {
	(void)conn;
	(void)name;
	(void)user_data;
	g_mutex_lock(&g_sni_mu);
	g_sni_watcher_lost = 1;
	g_mutex_unlock(&g_sni_mu);
}

static gpointer sni_thread_main(gpointer data) {
	GDBusConnection *conn = G_DBUS_CONNECTION(data);
	GMainContext *ctx = g_main_context_new();
	g_main_context_push_thread_default(ctx);
	int ok = 0;
	GDBusNodeInfo *info = g_dbus_node_info_new_for_xml(vitra_sni_xml, NULL);
	if (info) {
		GDBusInterfaceInfo *item = g_dbus_node_info_lookup_interface(info, VITRA_SNI_IFACE);
		GDBusInterfaceInfo *menu = g_dbus_node_info_lookup_interface(info, VITRA_DBUSMENU_IFACE);
		ok = g_dbus_connection_register_object(conn, VITRA_SNI_PATH, item, &sni_vtable, NULL, NULL, NULL) != 0 &&
			g_dbus_connection_register_object(conn, VITRA_SNI_MENU_PATH, menu, &sni_vtable, NULL, NULL, NULL) != 0;
	}
	if (ok) {
		g_bus_watch_name_on_connection(conn, VITRA_SNI_WATCHER, G_BUS_NAME_WATCHER_FLAGS_NONE,
			sni_watcher_appeared, sni_watcher_vanished, NULL, NULL);
	}
	g_mutex_lock(&g_sni_mu);
	g_sni_ready = ok ? 1 : -1;
	g_cond_broadcast(&g_sni_cond);
	g_mutex_unlock(&g_sni_mu);
	if (ok) {
		GMainLoop *loop = g_main_loop_new(ctx, FALSE);
		g_main_loop_run(loop); /* for the life of the process */
		g_main_loop_unref(loop);
	}
	if (info) {
		g_dbus_node_info_unref(info);
	}
	g_main_context_pop_thread_default(ctx);
	g_main_context_unref(ctx);
	g_object_unref(conn);
	return NULL;
}

/* sni_ensure_exported starts the tray thread once and waits until the item
 * and menu objects are exported. */
static int sni_ensure_exported(GDBusConnection *conn) {
	g_mutex_lock(&g_sni_mu);
	if (!g_sni_thread) {
		g_sni_thread = g_thread_new("vitra-tray", sni_thread_main, g_object_ref(conn));
	}
	while (g_sni_ready == 0) {
		g_cond_wait(&g_sni_cond, &g_sni_mu);
	}
	int ok = g_sni_ready > 0;
	g_mutex_unlock(&g_sni_mu);
	return ok;
}

static void sni_emit(GDBusConnection *conn, const char *path, const char *iface, const char *signal, GVariant *args) {
	g_dbus_connection_emit_signal(conn, NULL, path, iface, signal, args, NULL);
}

/* vitra_sni_pixmap_new builds an IconPixmap value from width x height
 * ARGB32 pixels in network byte order; NULL argb gives an empty array. */
static GVariant *vitra_sni_pixmap_new(const unsigned char *argb, int width, int height) {
	GVariantBuilder b;
	g_variant_builder_init(&b, G_VARIANT_TYPE("a(iiay)"));
	if (argb && width > 0 && height > 0) {
		GVariant *data = g_variant_new_fixed_array(G_VARIANT_TYPE_BYTE, argb, (gsize)width * (gsize)height * 4, 1);
		g_variant_builder_add(&b, "(ii@ay)", width, height, data);
	}
	return g_variant_ref_sink(g_variant_builder_end(&b));
}

/* vitra_sni_set shows (or updates) the StatusNotifierItem: tooltip, the
 * status title (also the XAyatanaLabel shown next to the icon), an ARGB32
 * icon (NULL for the default) and a flat menu of ids[i] / labels[i] with
 * VITRA_MENU_* flags[i]. Returns 1 when the item is registered with a
 * StatusNotifierWatcher, 0 when there is none (the caller falls back to
 * GtkStatusIcon). Safe from any thread. */
int vitra_sni_set(const char *tooltip, const char *title, const unsigned char *argb, int width, int height,
	const char *const *ids, const char *const *labels, const int *flags, int n, int click_activates) {
	GDBusConnection *conn = sni_bus();
	if (!conn || !vitra_sni_available() || !sni_ensure_exported(conn)) {
		return 0;
	}
	GPtrArray *new_ids = g_ptr_array_new_with_free_func(g_free);
	GPtrArray *new_labels = g_ptr_array_new_with_free_func(g_free);
	GArray *new_flags = g_array_new(FALSE, TRUE, sizeof(int));
	for (int i = 0; i < n; i++) {
		g_ptr_array_add(new_ids, g_strdup(ids[i] ? ids[i] : ""));
		g_ptr_array_add(new_labels, g_strdup(labels[i] ? labels[i] : ""));
		int f = flags ? flags[i] : 0;
		g_array_append_val(new_flags, f);
	}
	GVariant *pixmap = vitra_sni_pixmap_new(argb, width, height);
	g_mutex_lock(&g_sni_mu);
	if (g_sni_ids) {
		g_ptr_array_unref(g_sni_ids);
		g_ptr_array_unref(g_sni_labels);
		g_array_unref(g_sni_flags);
	}
	g_sni_ids = new_ids;
	g_sni_labels = new_labels;
	g_sni_flags = new_flags;
	g_free(g_sni_tooltip);
	g_sni_tooltip = g_strdup(tooltip ? tooltip : "");
	g_free(g_sni_title);
	g_sni_title = g_strdup(title ? title : "");
	if (g_sni_pixmap) {
		g_variant_unref(g_sni_pixmap);
	}
	g_sni_pixmap = pixmap;
	g_sni_click_activates = click_activates;
	char *label = g_strdup(g_sni_title);
	guint32 rev = ++g_sni_revision;
	int was_active = g_sni_active;
	char *name = NULL;
	if (!was_active) {
		name = g_strdup_printf("org.kde.StatusNotifierItem-%d-%u", (int)getpid(), ++g_sni_seq);
		g_sni_active = 1;
		g_free(g_sni_name);
		g_sni_name = g_strdup(name);
	}
	g_mutex_unlock(&g_sni_mu);

	if (was_active) {
		sni_emit(conn, VITRA_SNI_PATH, VITRA_SNI_IFACE, "NewTitle", NULL);
		sni_emit(conn, VITRA_SNI_PATH, VITRA_SNI_IFACE, "NewToolTip", NULL);
		sni_emit(conn, VITRA_SNI_PATH, VITRA_SNI_IFACE, "NewIcon", NULL);
		sni_emit(conn, VITRA_SNI_PATH, VITRA_SNI_IFACE, "XAyatanaNewLabel", g_variant_new("(ss)", label, label));
		sni_emit(conn, VITRA_SNI_MENU_PATH, VITRA_DBUSMENU_IFACE, "LayoutUpdated", g_variant_new("(ui)", rev, 0));
		g_free(label);
		return 1;
	}
	g_free(label);
	/* 4 = DBUS_NAME_FLAG_DO_NOT_QUEUE; reply 1 = primary owner. */
	guint32 owner = 0;
	GVariant *ret = sni_call(conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
		"RequestName", g_variant_new("(su)", name, 4u), G_VARIANT_TYPE("(u)"));
	if (ret) {
		g_variant_get(ret, "(u)", &owner);
		g_variant_unref(ret);
	}
	GVariant *reg = NULL;
	if (owner == 1) {
		reg = sni_call(conn, VITRA_SNI_WATCHER, VITRA_SNI_WATCHER_PATH, VITRA_SNI_WATCHER,
			"RegisterStatusNotifierItem", g_variant_new("(s)", name), NULL);
	}
	if (!reg) {
		g_mutex_lock(&g_sni_mu);
		g_sni_active = 0;
		g_free(g_sni_name);
		g_sni_name = NULL;
		g_mutex_unlock(&g_sni_mu);
		if (owner == 1) {
			GVariant *rel = sni_call(conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
				"ReleaseName", g_variant_new("(s)", name), NULL);
			if (rel) {
				g_variant_unref(rel);
			}
		}
		g_free(name);
		return 0;
	}
	g_variant_unref(reg);
	g_free(name);
	return 1;
}

int vitra_sni_anchor(int *x, int *y) {
	g_mutex_lock(&g_sni_mu);
	int has = g_sni_active && g_sni_has_point;
	*x = g_sni_x;
	*y = g_sni_y;
	g_mutex_unlock(&g_sni_mu);
	return has;
}

/* vitra_sni_clear removes the item: releasing its bus name makes the
 * watcher drop it. No-op when the item is not shown. */
void vitra_sni_clear(void) {
	GDBusConnection *conn = sni_bus();
	g_mutex_lock(&g_sni_mu);
	char *name = g_sni_active ? g_sni_name : NULL;
	if (g_sni_active) {
		g_sni_active = 0;
		g_sni_name = NULL;
	}
	g_mutex_unlock(&g_sni_mu);
	if (!name) {
		return;
	}
	if (conn) {
		sni_emit(conn, VITRA_SNI_PATH, VITRA_SNI_IFACE, "NewStatus", g_variant_new("(s)", "Passive"));
		GVariant *rel = sni_call(conn, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
			"ReleaseName", g_variant_new("(s)", name), NULL);
		if (rel) {
			g_variant_unref(rel);
		}
	}
	g_free(name);
}
/* ======================= end StatusNotifierItem tray ===================== */

static void on_drag_data(GtkWidget *widget, GdkDragContext *ctx, gint x, gint y,
	GtkSelectionData *data, guint info, guint time, gpointer user_data) {
	(void)widget;
	(void)x;
	(void)y;
	(void)info;
	gchar **uris = gtk_selection_data_get_uris(data);
	if (!uris) {
		gtk_drag_finish(ctx, FALSE, FALSE, time);
		return;
	}
	GString *paths = g_string_new(NULL);
	for (int i = 0; uris[i] != NULL; i++) {
		gchar *path = g_filename_from_uri(uris[i], NULL, NULL);
		if (!path) {
			continue;
		}
		if (paths->len > 0) {
			g_string_append_c(paths, '\n');
		}
		g_string_append(paths, path);
		g_free(path);
	}
	g_strfreev(uris);
	if (paths->len > 0) {
		goVitraDrop((char *)user_data, paths->str);
		gtk_drag_finish(ctx, TRUE, FALSE, time);
	} else {
		gtk_drag_finish(ctx, FALSE, FALSE, time);
	}
	g_string_free(paths, TRUE);
}

void vitra_win_set_drag_drop(VitraWin *w, int enabled) {
	if (!w || !w->view) {
		return;
	}
	GtkWidget *view = GTK_WIDGET(w->view);
	g_signal_handlers_disconnect_by_func(view, G_CALLBACK(on_drag_data), w->id);
	if (!enabled) {
		gtk_drag_dest_unset(view);
		return;
	}
	gtk_drag_dest_set(view, GTK_DEST_DEFAULT_ALL, NULL, 0, GDK_ACTION_COPY);
	gtk_drag_dest_add_uri_targets(view);
	g_signal_connect(view, "drag-data-received", G_CALLBACK(on_drag_data), w->id);
}

#define VITRA_MAX_HOTKEYS 64

#ifdef GDK_WINDOWING_X11

#define VITRA_IGNORED_MODS (LockMask | Mod2Mask)

typedef struct {
	char *accel;
	char *action;
	KeyCode keycode;
	guint mods; /* X11 modifier mask */
} HotkeyEntry;

static HotkeyEntry g_hotkeys[VITRA_MAX_HOTKEYS];
static int g_hotkey_n = 0;
static int g_hotkey_filter = 0;

static guint gdk_mods_to_x(GdkModifierType mods) {
	guint x = 0;
	if (mods & GDK_CONTROL_MASK) {
		x |= ControlMask;
	}
	if (mods & GDK_SHIFT_MASK) {
		x |= ShiftMask;
	}
	if (mods & GDK_MOD1_MASK) {
		x |= Mod1Mask;
	}
	if (mods & (GDK_SUPER_MASK | GDK_META_MASK | GDK_MOD4_MASK)) {
		x |= Mod4Mask;
	}
	return x;
}

static void x_ungrab_key(Display *dpy, Window root, KeyCode keycode, guint mods) {
	guint masks[] = {0, LockMask, Mod2Mask, LockMask | Mod2Mask};
	for (size_t i = 0; i < sizeof(masks) / sizeof(masks[0]); i++) {
		XUngrabKey(dpy, keycode, mods | masks[i], root);
	}
}

static int x_grab_key(Display *dpy, Window root, KeyCode keycode, guint mods) {
	guint masks[] = {0, LockMask, Mod2Mask, LockMask | Mod2Mask};
	gdk_x11_display_error_trap_push(gdk_display_get_default());
	for (size_t i = 0; i < sizeof(masks) / sizeof(masks[0]); i++) {
		XGrabKey(dpy, keycode, mods | masks[i], root, True, GrabModeAsync, GrabModeAsync);
	}
	XSync(dpy, False);
	return gdk_x11_display_error_trap_pop(gdk_display_get_default()) == 0;
}

static GdkFilterReturn hotkey_filter(GdkXEvent *xevent, GdkEvent *event, gpointer data) {
	(void)event;
	(void)data;
	XEvent *ev = (XEvent *)xevent;
	if (!ev || ev->type != KeyPress) {
		return GDK_FILTER_CONTINUE;
	}
	guint state = (guint)(ev->xkey.state & ~(VITRA_IGNORED_MODS));
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].keycode == ev->xkey.keycode && g_hotkeys[i].mods == state && g_hotkeys[i].action) {
			goVitraAction(g_hotkeys[i].action);
			return GDK_FILTER_REMOVE;
		}
	}
	return GDK_FILTER_CONTINUE;
}

static void ensure_hotkey_filter(void) {
	if (g_hotkey_filter) {
		return;
	}
	gdk_window_add_filter(NULL, hotkey_filter, NULL);
	g_hotkey_filter = 1;
}

#endif /* GDK_WINDOWING_X11 */

int vitra_hotkey_supported(void) {
	GdkDisplay *d = gdk_display_get_default();
	if (!d) {
		return 0;
	}
#ifdef GDK_WINDOWING_X11
	return GDK_IS_X11_DISPLAY(d) ? 1 : 0;
#else
	(void)d;
	return 0;
#endif
}

int vitra_register_hotkey(const char *accelerator, const char *action_id) {
#ifndef GDK_WINDOWING_X11
	(void)accelerator;
	(void)action_id;
	return 0;
#else
	if (!accelerator || !action_id || accelerator[0] == '\0' || action_id[0] == '\0') {
		return 0;
	}
	if (!vitra_hotkey_supported()) {
		return 0;
	}
	char *norm = normalize_accel(accelerator);
	if (!norm) {
		return 0;
	}
	guint keyval = 0;
	GdkModifierType gmods = 0;
	gtk_accelerator_parse(norm, &keyval, &gmods);
	g_free(norm);
	if (keyval == 0 || gmods == 0) {
		/* Require at least one modifier for global hotkeys. */
		return 0;
	}
	guint xmods = gdk_mods_to_x(gmods);
	if (xmods == 0) {
		return 0;
	}
	Display *dpy = GDK_DISPLAY_XDISPLAY(gdk_display_get_default());
	Window root = DefaultRootWindow(dpy);
	KeyCode keycode = XKeysymToKeycode(dpy, (KeySym)keyval);
	if (keycode == 0) {
		return 0;
	}

	/* Replace existing binding for the same accelerator string. */
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].accel && strcmp(g_hotkeys[i].accel, accelerator) == 0) {
			x_ungrab_key(dpy, root, g_hotkeys[i].keycode, g_hotkeys[i].mods);
			g_free(g_hotkeys[i].action);
			g_hotkeys[i].action = g_strdup(action_id);
			g_hotkeys[i].keycode = keycode;
			g_hotkeys[i].mods = xmods;
			if (!x_grab_key(dpy, root, keycode, xmods)) {
				return 0;
			}
			ensure_hotkey_filter();
			return 1;
		}
	}
	if (g_hotkey_n >= VITRA_MAX_HOTKEYS) {
		return 0;
	}
	if (!x_grab_key(dpy, root, keycode, xmods)) {
		return 0;
	}
	g_hotkeys[g_hotkey_n].accel = g_strdup(accelerator);
	g_hotkeys[g_hotkey_n].action = g_strdup(action_id);
	g_hotkeys[g_hotkey_n].keycode = keycode;
	g_hotkeys[g_hotkey_n].mods = xmods;
	g_hotkey_n++;
	ensure_hotkey_filter();
	return 1;
#endif
}

int vitra_unregister_hotkey(const char *accelerator) {
#ifndef GDK_WINDOWING_X11
	(void)accelerator;
	return 0;
#else
	if (!accelerator || !vitra_hotkey_supported()) {
		return 0;
	}
	Display *dpy = GDK_DISPLAY_XDISPLAY(gdk_display_get_default());
	Window root = DefaultRootWindow(dpy);
	for (int i = 0; i < g_hotkey_n; i++) {
		if (g_hotkeys[i].accel && strcmp(g_hotkeys[i].accel, accelerator) == 0) {
			x_ungrab_key(dpy, root, g_hotkeys[i].keycode, g_hotkeys[i].mods);
			g_free(g_hotkeys[i].accel);
			g_free(g_hotkeys[i].action);
			g_hotkeys[i] = g_hotkeys[g_hotkey_n - 1];
			g_hotkeys[g_hotkey_n - 1].accel = NULL;
			g_hotkeys[g_hotkey_n - 1].action = NULL;
			g_hotkeys[g_hotkey_n - 1].keycode = 0;
			g_hotkeys[g_hotkey_n - 1].mods = 0;
			g_hotkey_n--;
			return 1;
		}
	}
	return 0;
#endif
}

/* ===================================================================== *
 * Wayland global shortcuts: org.freedesktop.portal.GlobalShortcuts.
 *
 * XGrabKey (above) needs an X11 display. On Wayland the compositor owns the
 * keyboard, and the GlobalShortcuts portal is the supported way to ask it
 * for OS-wide shortcuts. Everything here is plain GDBus, which GTK already
 * links.
 *
 * Flow: CreateSession → BindShortcuts, each answered asynchronously by a
 * Request.Response signal (0 = granted, 1 = cancelled by the user, 2 = other
 * failure). Binding may show the desktop's consent dialog, so the calls
 * block the calling Go goroutine (never the GTK thread) on a private main
 * context until the response arrives. Activated signals are received on a
 * dedicated listener thread and forwarded through goVitraAction.
 *
 * The portal has no "unbind": the Go side rebinds the remaining set on a
 * fresh session and closes the old one.
 * ===================================================================== */

#define VITRA_PORTAL_BUS "org.freedesktop.portal.Desktop"
#define VITRA_PORTAL_PATH "/org/freedesktop/portal/desktop"
#define VITRA_PORTAL_GS "org.freedesktop.portal.GlobalShortcuts"
/* Consent dialogs wait on a person; give them time, but never hang forever. */
#define VITRA_PORTAL_RESPONSE_TIMEOUT_S 300
#define VITRA_PORTAL_CALL_TIMEOUT_MS 5000

static GMutex g_gs_mu;
static char *g_gs_session = NULL; /* current session object path, or NULL */
static guint g_gs_token = 0;
static int g_gs_registered = 0;
static GThread *g_gs_listener = NULL;

static GDBusConnection *gs_bus(void) {
	/* Process-wide shared connection: GIO keeps it alive; held, never freed. */
	static GDBusConnection *conn = NULL;
	if (conn && !g_dbus_connection_is_closed(conn)) {
		return conn;
	}
	GError *err = NULL;
	GDBusConnection *c = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &err);
	if (!c) {
		if (err) {
			g_error_free(err);
		}
		return NULL;
	}
	conn = c;
	return conn;
}

/* vitra_gs_portal_version returns the GlobalShortcuts portal's interface
 * version, or 0 when no session bus or no such portal is present. */
int vitra_gs_portal_version(void) {
	GDBusConnection *conn = gs_bus();
	if (!conn) {
		return 0;
	}
	GError *err = NULL;
	GVariant *ret = g_dbus_connection_call_sync(conn, VITRA_PORTAL_BUS, VITRA_PORTAL_PATH,
		"org.freedesktop.DBus.Properties", "Get",
		g_variant_new("(ss)", VITRA_PORTAL_GS, "version"),
		G_VARIANT_TYPE("(v)"), G_DBUS_CALL_FLAGS_NONE, VITRA_PORTAL_CALL_TIMEOUT_MS, NULL, &err);
	if (!ret) {
		if (err) {
			g_error_free(err);
		}
		return 0;
	}
	GVariant *v = NULL;
	g_variant_get(ret, "(v)", &v);
	int version = 0;
	if (v && g_variant_is_of_type(v, G_VARIANT_TYPE_UINT32)) {
		version = (int)g_variant_get_uint32(v);
	}
	if (v) {
		g_variant_unref(v);
	}
	g_variant_unref(ret);
	return version;
}

static void gs_on_activated(GDBusConnection *conn, const gchar *sender, const gchar *path,
	const gchar *iface, const gchar *signal, GVariant *params, gpointer user_data) {
	(void)conn;
	(void)sender;
	(void)path;
	(void)iface;
	(void)signal;
	(void)user_data;
	if (!g_variant_is_of_type(params, G_VARIANT_TYPE("(osta{sv})"))) {
		return;
	}
	const gchar *session = NULL;
	const gchar *id = NULL;
	g_variant_get(params, "(&o&sta{sv})", &session, &id, NULL, NULL);
	g_mutex_lock(&g_gs_mu);
	int ours = g_gs_session != NULL && g_strcmp0(g_gs_session, session) == 0;
	g_mutex_unlock(&g_gs_mu);
	if (ours && id && id[0] != '\0') {
		/* Shortcut ids are Vitra action ids (see webview.go). */
		goVitraAction((char *)id);
	}
}

static gpointer gs_listener_main(gpointer data) {
	GDBusConnection *conn = G_DBUS_CONNECTION(data);
	GMainContext *ctx = g_main_context_new();
	g_main_context_push_thread_default(ctx);
	g_dbus_connection_signal_subscribe(conn, VITRA_PORTAL_BUS, VITRA_PORTAL_GS, "Activated",
		VITRA_PORTAL_PATH, NULL, G_DBUS_SIGNAL_FLAGS_NONE, gs_on_activated, NULL, NULL);
	GMainLoop *loop = g_main_loop_new(ctx, FALSE);
	g_main_loop_run(loop); /* for the life of the process */
	g_main_loop_unref(loop);
	g_main_context_pop_thread_default(ctx);
	g_main_context_unref(ctx);
	g_object_unref(conn);
	return NULL;
}

static void gs_ensure_listener(GDBusConnection *conn) {
	if (g_gs_listener) {
		return;
	}
	g_gs_listener = g_thread_new("vitra-portal-shortcuts", gs_listener_main, g_object_ref(conn));
}

typedef struct {
	int done;
	guint32 code;
	GVariant *results;
} GsResponse;

static void gs_on_response(GDBusConnection *conn, const gchar *sender, const gchar *path,
	const gchar *iface, const gchar *signal, GVariant *params, gpointer user_data) {
	(void)conn;
	(void)sender;
	(void)path;
	(void)iface;
	(void)signal;
	GsResponse *r = (GsResponse *)user_data;
	if (r->done || !g_variant_is_of_type(params, G_VARIANT_TYPE("(ua{sv})"))) {
		return;
	}
	g_variant_get(params, "(u@a{sv})", &r->code, &r->results);
	r->done = 1;
}

static gboolean gs_on_timeout(gpointer user_data) {
	int *expired = (int *)user_data;
	*expired = 1;
	return G_SOURCE_REMOVE;
}

/* gs_request calls a portal method that answers through a Request object and
 * waits for its Response. The request path is derived from handle_token and
 * subscribed before the call (the bus applies the match rule before it routes
 * the call), so a fast response cannot be missed. Returns the response code
 * (0 ok, 1 cancelled, 2 other), or 3 on a D-Bus error or timeout; *err_out
 * describes failures. *results is set (caller unrefs) on code 0. */
static int gs_request(GDBusConnection *conn, const char *method, GVariant *args, const char *token,
	GVariant **results, char **err_out) {
	*results = NULL;
	const gchar *unique = g_dbus_connection_get_unique_name(conn);
	if (!unique) {
		g_variant_unref(g_variant_ref_sink(args));
		*err_out = g_strdup("session bus connection has no unique name");
		return 3;
	}
	/* ":1.42" → "1_42" per the portal Request spec. */
	gchar *sender = g_strdup(unique[0] == ':' ? unique + 1 : unique);
	g_strdelimit(sender, ".", '_');
	gchar *req_path = g_strdup_printf(VITRA_PORTAL_PATH "/request/%s/%s", sender, token);
	g_free(sender);

	GMainContext *ctx = g_main_context_new();
	g_main_context_push_thread_default(ctx);
	GsResponse resp = {0, 2, NULL};
	guint sub = g_dbus_connection_signal_subscribe(conn, VITRA_PORTAL_BUS, "org.freedesktop.portal.Request",
		"Response", req_path, NULL, G_DBUS_SIGNAL_FLAGS_NONE, gs_on_response, &resp, NULL);

	int code = 3;
	GError *err = NULL;
	GVariant *ret = g_dbus_connection_call_sync(conn, VITRA_PORTAL_BUS, VITRA_PORTAL_PATH, VITRA_PORTAL_GS,
		method, args, G_VARIANT_TYPE("(o)"), G_DBUS_CALL_FLAGS_NONE, VITRA_PORTAL_CALL_TIMEOUT_MS, NULL, &err);
	if (!ret) {
		*err_out = g_strdup_printf("%s: %s", method, err ? err->message : "call failed");
		if (err) {
			g_error_free(err);
		}
	} else {
		const gchar *handle = NULL;
		g_variant_get(ret, "(&o)", &handle);
		if (g_strcmp0(handle, req_path) != 0) {
			/* Every GlobalShortcuts portal honours handle_token; anything
			 * else would answer on a path nobody listens to. */
			*err_out = g_strdup_printf("%s: unexpected request handle %s", method, handle);
		} else {
			int expired = 0;
			GSource *timer = g_timeout_source_new_seconds(VITRA_PORTAL_RESPONSE_TIMEOUT_S);
			g_source_set_callback(timer, gs_on_timeout, &expired, NULL);
			g_source_attach(timer, ctx);
			while (!resp.done && !expired) {
				g_main_context_iteration(ctx, TRUE);
			}
			g_source_destroy(timer);
			g_source_unref(timer);
			if (resp.done) {
				code = (int)resp.code;
				if (code == 0) {
					*results = resp.results;
					resp.results = NULL;
				}
			} else {
				*err_out = g_strdup_printf("%s: no response from the portal", method);
				/* Withdraw the pending request so no dialog outlives us. */
				GVariant *closed = g_dbus_connection_call_sync(conn, VITRA_PORTAL_BUS, req_path,
					"org.freedesktop.portal.Request", "Close", NULL, NULL, G_DBUS_CALL_FLAGS_NONE,
					VITRA_PORTAL_CALL_TIMEOUT_MS, NULL, NULL);
				if (closed) {
					g_variant_unref(closed);
				}
			}
		}
		g_variant_unref(ret);
	}
	g_dbus_connection_signal_unsubscribe(conn, sub);
	if (resp.results) {
		g_variant_unref(resp.results);
	}
	/* Drain what the unsubscribe queued before dropping the context. */
	while (g_main_context_iteration(ctx, FALSE)) {
	}
	g_main_context_pop_thread_default(ctx);
	g_main_context_unref(ctx);
	g_free(req_path);
	return code;
}

static void gs_close_session(GDBusConnection *conn, const char *session) {
	if (!conn || !session) {
		return;
	}
	GVariant *ret = g_dbus_connection_call_sync(conn, VITRA_PORTAL_BUS, session, "org.freedesktop.portal.Session",
		"Close", NULL, NULL, G_DBUS_CALL_FLAGS_NONE, VITRA_PORTAL_CALL_TIMEOUT_MS, NULL, NULL);
	if (ret) {
		g_variant_unref(ret);
	}
}

/* Tell xdg-desktop-portal which app this host (unsandboxed) process is:
 * portals key shortcut consent by app id. Only reverse-DNS program names
 * qualify. Optional interface (xdg-desktop-portal ≥ 1.19); errors ignored. */
static void gs_register_app(GDBusConnection *conn) {
	if (g_gs_registered) {
		return;
	}
	g_gs_registered = 1;
	const char *app = g_get_prgname();
	if (!app || !strchr(app, '.') || !g_dbus_is_name(app) || g_dbus_is_unique_name(app)) {
		return;
	}
	GVariantBuilder opts;
	g_variant_builder_init(&opts, G_VARIANT_TYPE_VARDICT);
	GVariant *ret = g_dbus_connection_call_sync(conn, VITRA_PORTAL_BUS, VITRA_PORTAL_PATH,
		"org.freedesktop.host.portal.Registry", "Register", g_variant_new("(sa{sv})", app, &opts),
		NULL, G_DBUS_CALL_FLAGS_NONE, VITRA_PORTAL_CALL_TIMEOUT_MS, NULL, NULL);
	if (ret) {
		g_variant_unref(ret);
	}
}

/* vitra_gs_portal_bind binds exactly ids[i] → triggers[i] (i < n) on a new
 * portal session and, on success, closes the previous session. On failure the
 * previous session (and its shortcuts) stays active. Returns 0 on success,
 * 1 when the user cancelled or refused, 2 on any other failure; *err_out
 * (g_free) describes failures. Blocks until the portal answers. Callers
 * serialize calls (webview.go holds a mutex). */
int vitra_gs_portal_bind(const char *const *ids, const char *const *triggers, int n, char **err_out) {
	*err_out = NULL;
	GDBusConnection *conn = gs_bus();
	if (!conn) {
		*err_out = g_strdup("no D-Bus session bus");
		return 2;
	}
	gs_register_app(conn);
	gs_ensure_listener(conn);

	guint tok = ++g_gs_token;
	gchar *ctoken = g_strdup_printf("vitra_c%u", tok);
	gchar *stoken = g_strdup_printf("vitra_s%u", tok);
	gchar *btoken = g_strdup_printf("vitra_b%u", tok);

	GVariantBuilder opts;
	g_variant_builder_init(&opts, G_VARIANT_TYPE_VARDICT);
	g_variant_builder_add(&opts, "{sv}", "handle_token", g_variant_new_string(ctoken));
	g_variant_builder_add(&opts, "{sv}", "session_handle_token", g_variant_new_string(stoken));
	GVariant *results = NULL;
	int code = gs_request(conn, "CreateSession", g_variant_new("(a{sv})", &opts), ctoken, &results, err_out);
	gchar *session = NULL;
	if (code == 0) {
		GVariant *sh = g_variant_lookup_value(results, "session_handle", NULL);
		if (sh && (g_variant_is_of_type(sh, G_VARIANT_TYPE_STRING) || g_variant_is_of_type(sh, G_VARIANT_TYPE_OBJECT_PATH))) {
			session = g_variant_dup_string(sh, NULL);
		}
		if (sh) {
			g_variant_unref(sh);
		}
		g_variant_unref(results);
		results = NULL;
		if (!session || !g_variant_is_object_path(session)) {
			g_free(session);
			session = NULL;
			*err_out = g_strdup("CreateSession: no session handle in the response");
			code = 2;
		}
	} else if (!*err_out) {
		*err_out = g_strdup(code == 1 ? "CreateSession: cancelled by the user" : "CreateSession: refused by the portal");
	}

	if (session) {
		GVariantBuilder list;
		g_variant_builder_init(&list, G_VARIANT_TYPE("a(sa{sv})"));
		for (int i = 0; i < n; i++) {
			GVariantBuilder props;
			g_variant_builder_init(&props, G_VARIANT_TYPE_VARDICT);
			g_variant_builder_add(&props, "{sv}", "description", g_variant_new_string(ids[i]));
			if (triggers[i] && triggers[i][0] != '\0') {
				g_variant_builder_add(&props, "{sv}", "preferred_trigger", g_variant_new_string(triggers[i]));
			}
			g_variant_builder_add(&list, "(sa{sv})", ids[i], &props);
		}
		GVariantBuilder bopts;
		g_variant_builder_init(&bopts, G_VARIANT_TYPE_VARDICT);
		g_variant_builder_add(&bopts, "{sv}", "handle_token", g_variant_new_string(btoken));
		code = gs_request(conn, "BindShortcuts", g_variant_new("(oa(sa{sv})sa{sv})", session, &list, "", &bopts),
			btoken, &results, err_out);
		if (results) {
			g_variant_unref(results);
		}
		if (code == 0) {
			g_mutex_lock(&g_gs_mu);
			gchar *old = g_gs_session;
			g_gs_session = session;
			g_mutex_unlock(&g_gs_mu);
			gs_close_session(conn, old);
			g_free(old);
		} else {
			if (!*err_out) {
				*err_out = g_strdup(code == 1 ? "BindShortcuts: cancelled or refused by the user"
				                              : "BindShortcuts: refused by the portal");
			}
			gs_close_session(conn, session);
			g_free(session);
		}
	}
	g_free(ctoken);
	g_free(stoken);
	g_free(btoken);
	return code == 0 ? 0 : (code == 1 ? 1 : 2);
}

/* vitra_gs_portal_close ends the current session, releasing every shortcut. */
void vitra_gs_portal_close(void) {
	g_mutex_lock(&g_gs_mu);
	gchar *old = g_gs_session;
	g_gs_session = NULL;
	g_mutex_unlock(&g_gs_mu);
	if (old) {
		gs_close_session(gs_bus(), old);
		g_free(old);
	}
}
/* ====================== end Wayland global shortcuts ===================== */
