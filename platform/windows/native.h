#ifndef VITRA_WINDOWS_NATIVE_H
#define VITRA_WINDOWS_NATIVE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct VitraWin VitraWin;

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

void vitra_win32_init(const char *prgname);
const char *vitra_get_program_name(void);
void vitra_win32_main(void);
void vitra_win32_quit(void);
/* Make the calling thread the UI thread: queued jobs are posted to it. */
void vitra_win32_bind_ui_thread(void);
/* Queue Go job id for goVitraIdle on the UI thread. */
void vitra_idle_add(unsigned long long id);
/* Post Go job id to the UI thread's job window, even from the UI thread. */
void vitra_idle_post(unsigned long long id);
unsigned long vitra_current_thread_id(void);
void vitra_set_devtools(int enabled);

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload);
void vitra_win_navigate(VitraWin *w, const char *uri);
int vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);
void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path);
VitraChrome vitra_win_chrome(VitraWin *w);
void vitra_win_focus(VitraWin *w);
void vitra_win_blur(VitraWin *w);

void vitra_win_clear_menu(VitraWin *w);
/* vitra_win_set_skip_taskbar keeps the window off the taskbar (accessory
 * apps): it is owned by a hidden window, which keeps its normal caption. */
void vitra_win_set_skip_taskbar(VitraWin *w, int skip);
int vitra_win_skips_taskbar(VitraWin *w);

/* Menu item flags (vitra_win_add_menu_item, vitra_tray_add_menu_item). */
enum {
	VITRA_MENU_SEPARATOR = 1,
	VITRA_MENU_DISABLED = 2,
	VITRA_MENU_CHECKED = 4,
};

void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut, int flags);
int vitra_win_activate_accel(VitraWin *w, const char *shortcut);

char *vitra_open_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_open_dialog_multi(const char *title, const char *default_path, const char *filters, int *out_len);
char *vitra_save_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_open_directory_dialog(const char *title, const char *default_path);
int vitra_message_dialog(const char *title, const char *message, int confirm);
int vitra_show_notification(const char *title, const char *body);

/* click_activates: a left click calls goVitraTrayClick instead of reporting
 * the "tray.activate" action (the menu stays on right click). */
void vitra_tray_set(const char *tooltip, const void *png, int png_len, int click_activates);
/* vitra_tray_anchor stores the notification icon's rectangle in screen
 * pixels; 0 when no icon is shown. */
int vitra_tray_anchor(int *x, int *y, int *w, int *h);
void vitra_tray_clear_menu(void);
void vitra_tray_add_menu_item(const char *item_id, const char *item_label, int flags);
void vitra_tray_clear(void);

void vitra_win_set_drag_drop(VitraWin *w, int enabled);

int vitra_register_hotkey(const char *accelerator, const char *action_id);
int vitra_unregister_hotkey(const char *accelerator);
void vitra_clear_hotkeys(void);

#ifdef __cplusplus
}
#endif

#endif
