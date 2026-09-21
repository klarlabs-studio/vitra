//go:build linux && cgo && vitra_native

#include "native.h"
#include <stdlib.h>
#include <string.h>

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xlib.h>
#endif

extern void goVitraIdle(void *);
extern void goVitraMessage(char *, char *);
extern void goVitraDestroy(char *);
extern int goVitraNav(char *, char *);
extern void goVitraAction(char *);
extern void goVitraDrop(char *, char *);

static GtkStatusIcon *g_tray = NULL;
static GtkWidget *g_tray_menu = NULL;

static gboolean idle_cb(gpointer data) {
	goVitraIdle(data);
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
	w->accels = gtk_accel_group_new();
	gtk_window_add_accel_group(GTK_WINDOW(w->window), w->accels);

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

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut) {
	if (!w || !w->menubar || !menu_label || !item_id || !item_label) {
		return;
	}
	GtkWidget *top = find_or_create_menu(w->menubar, menu_label);
	GtkWidget *submenu = gtk_menu_item_get_submenu(GTK_MENU_ITEM(top));
	GtkWidget *item = gtk_menu_item_new_with_label(item_label);
	char *id_copy = g_strdup(item_id);
	g_signal_connect_data(item, "activate", G_CALLBACK(on_action), id_copy, (GClosureNotify)g_free, 0);
	if (shortcut && shortcut[0] != '\0' && w->accels) {
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


char *vitra_save_dialog(void) {
	GtkWidget *dialog = gtk_file_chooser_dialog_new(
		"Save File", NULL, GTK_FILE_CHOOSER_ACTION_SAVE,
		"_Cancel", GTK_RESPONSE_CANCEL,
		"_Save", GTK_RESPONSE_ACCEPT, NULL);
	gtk_file_chooser_set_do_overwrite_confirmation(GTK_FILE_CHOOSER(dialog), TRUE);
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

static void on_tray_popup(GtkStatusIcon *icon, guint button, guint32 activate_time, gpointer user_data) {
	(void)user_data;
	if (!g_tray_menu) {
		return;
	}
	gtk_widget_show_all(g_tray_menu);
	gtk_menu_popup(GTK_MENU(g_tray_menu), NULL, NULL, gtk_status_icon_position_menu, icon, button, activate_time);
}

void vitra_tray_set(const char *tooltip) {
	if (!g_tray) {
		g_tray = gtk_status_icon_new_from_icon_name("application-x-executable");
		g_signal_connect(g_tray, "activate", G_CALLBACK(on_tray_activate), NULL);
		g_signal_connect(g_tray, "popup-menu", G_CALLBACK(on_tray_popup), NULL);
	}
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

void vitra_tray_add_menu_item(const char *item_id, const char *item_label) {
	if (!item_id || !item_label) {
		return;
	}
	if (!g_tray_menu) {
		g_tray_menu = gtk_menu_new();
	}
	GtkWidget *item = gtk_menu_item_new_with_label(item_label);
	char *id_copy = g_strdup(item_id);
	g_signal_connect_data(item, "activate", G_CALLBACK(on_action), id_copy, (GClosureNotify)g_free, 0);
	gtk_menu_shell_append(GTK_MENU_SHELL(g_tray_menu), item);
	gtk_widget_show_all(item);
}

void vitra_tray_clear(void) {
	vitra_tray_clear_menu();
	if (g_tray) {
		gtk_status_icon_set_visible(g_tray, FALSE);
	}
}

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
