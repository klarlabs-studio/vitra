#ifndef VITRA_LINUX_NATIVE_H
#define VITRA_LINUX_NATIVE_H

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

typedef struct {
	GtkWidget *window;
	GtkWidget *vbox;
	GtkWidget *menubar;
	WebKitWebView *view;
	GtkAccelGroup *accels;
	char *id;
	int maximized;
	int fullscreen;
	int above;
	int minimized;
	int hidden;
	int req_width;
	int req_height;
	char *icon_path;
	int panel;          /* a tray panel (WindowKindPanel) */
	gint64 hidden_at;   /* g_get_monotonic_time when the panel hid on focus-out */
} VitraWin;

typedef struct {
	char *title;
	int width;
	int height;
	int maximized;
	int fullscreen;
	int above;
	int minimized;
	int hidden;
	char *icon_path;
} VitraChrome;

void vitra_gtk_init(const char *prgname);
const char *vitra_get_prgname(void);
void vitra_gtk_main(void);
void vitra_gtk_quit(void);
/* Queue Go job id for goVitraIdle on the UI thread. */
void vitra_idle_add(unsigned long long id);
void vitra_set_devtools(int enabled);

/* panel: a tray panel (undecorated, kept above, no taskbar entry, opened
 * hidden, hidden on focus-out) instead of a regular window. */
VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload, int panel);
/* vitra_panel_show places a panel under the anchor (centered on the primary
 * monitor when has_anchor is 0) and shows it. Returns 0 when w is no panel. */
int vitra_panel_show(VitraWin *w, int x, int y, int aw, int ah, int has_anchor);
int vitra_panel_hide(VitraWin *w);
/* 1 while shown or hidden on focus-out a moment ago, 0, or -1 for no panel. */
int vitra_panel_shown(VitraWin *w);
/* Tests: the window's position and size, and a focus loss. */
void vitra_win_frame(VitraWin *w, int *x, int *y, int *fw, int *fh);
void vitra_panel_blur(VitraWin *w);
void vitra_win_navigate(VitraWin *w, const char *uri);
void vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);
void vitra_win_clear_menu(VitraWin *w);
/* vitra_win_set_skip_taskbar keeps the window out of taskbars and pagers
 * (accessory apps). */
void vitra_win_set_skip_taskbar(VitraWin *w, int skip);
int vitra_win_skips_taskbar(VitraWin *w);

/* Menu item flags (vitra_win_add_menu_item, vitra_tray_add_menu_item, vitra_sni_set). */
enum {
	VITRA_MENU_SEPARATOR = 1,
	VITRA_MENU_DISABLED = 2,
	VITRA_MENU_CHECKED = 4,
};

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut, int flags);
int vitra_win_activate_accel(VitraWin *w, const char *shortcut);

char *vitra_clip_get(void);
void vitra_clip_set(const char *text);
char *vitra_open_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_open_dialog_multi(const char *title, const char *default_path, const char *filters, int *out_len);
char *vitra_save_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_open_directory_dialog(const char *title, const char *default_path);
int vitra_message_dialog(const char *title, const char *message, int confirm);
int vitra_show_notification(const char *title, const char *body);

/* click_activates: a left click calls goVitraTrayClick instead of reporting
 * the "tray.activate" action (the menu stays on right click). */
void vitra_tray_set(const char *tooltip, const char *title, const unsigned char *rgba, int width, int height, int click_activates);
/* vitra_tray_anchor stores the GtkStatusIcon's rectangle; 0 when unknown. */
int vitra_tray_anchor(int *x, int *y, int *w, int *h);
void vitra_tray_clear_menu(void);
void vitra_tray_add_menu_item(const char *item_id, const char *item_label, int flags);
void vitra_tray_clear(void);

/* StatusNotifierItem tray (org.kde.StatusNotifierWatcher + dbusmenu). */
int vitra_sni_available(void);
int vitra_sni_set(const char *tooltip, const char *title, const unsigned char *argb, int width, int height,
	const char *const *ids, const char *const *labels, const int *flags, int n, int click_activates);
/* vitra_sni_anchor stores the position of the last Activate the panel sent;
 * 0 when there was none or the panel sent no position. */
int vitra_sni_anchor(int *x, int *y);
void vitra_sni_clear(void);

void vitra_win_set_drag_drop(VitraWin *w, int enabled);
void vitra_win_flush(void);
void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path);
VitraChrome vitra_win_chrome(VitraWin *w);
void vitra_win_focus(VitraWin *w);
void vitra_win_blur(VitraWin *w);

int vitra_hotkey_supported(void);
int vitra_register_hotkey(const char *accelerator, const char *action_id);
int vitra_unregister_hotkey(const char *accelerator);

/* Wayland global shortcuts (org.freedesktop.portal.GlobalShortcuts). */
int vitra_gs_portal_version(void);
int vitra_gs_portal_bind(const char *const *ids, const char *const *triggers, int n, char **err_out);
void vitra_gs_portal_close(void);

#endif
