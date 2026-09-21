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
void vitra_idle_add(void *data);

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload);
void vitra_win_navigate(VitraWin *w, const char *uri);
int vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);
void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path);
VitraChrome vitra_win_chrome(VitraWin *w);

void vitra_win_clear_menu(VitraWin *w);
void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut);
int vitra_win_activate_accel(VitraWin *w, const char *shortcut);

char *vitra_open_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_save_dialog(const char *title, const char *default_path, const char *filters);
char *vitra_open_directory_dialog(void);
int vitra_message_dialog(const char *title, const char *message, int confirm);
int vitra_show_notification(const char *title, const char *body);

void vitra_tray_set(const char *tooltip);
void vitra_tray_clear_menu(void);
void vitra_tray_add_menu_item(const char *item_id, const char *item_label);
void vitra_tray_clear(void);

void vitra_win_set_drag_drop(VitraWin *w, int enabled);

int vitra_register_hotkey(const char *accelerator, const char *action_id);
int vitra_unregister_hotkey(const char *accelerator);
void vitra_clear_hotkeys(void);

#ifdef __cplusplus
}
#endif

#endif
