#ifndef VITRA_DARWIN_NATIVE_H
#define VITRA_DARWIN_NATIVE_H

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

void vitra_app_init(void);
void vitra_app_run(void);
void vitra_app_quit(void);
void vitra_idle_add(void *data);

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload);
void vitra_win_navigate(VitraWin *w, const char *uri);
void vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);
void vitra_win_apply_chrome(VitraWin *w, const char *title, int width, int height, int maximized, int fullscreen, int above, int minimized, int hidden, const char *icon_path);
VitraChrome vitra_win_chrome(VitraWin *w);
void vitra_win_clear_menu(VitraWin *w);
void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label, const char *shortcut);
int vitra_win_activate_accel(VitraWin *w, const char *shortcut);

void vitra_tray_set(const char *tooltip);
void vitra_tray_clear_menu(void);
void vitra_tray_add_menu_item(const char *item_id, const char *item_label);
void vitra_tray_clear(void);

char *vitra_open_dialog(void);
char *vitra_save_dialog(void);

#ifdef __cplusplus
}
#endif

#endif
