#ifndef VITRA_LINUX_NATIVE_H
#define VITRA_LINUX_NATIVE_H

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

typedef struct {
	GtkWidget *window;
	GtkWidget *vbox;
	GtkWidget *menubar;
	WebKitWebView *view;
	char *id;
} VitraWin;

void vitra_gtk_init(void);
void vitra_gtk_main(void);
void vitra_gtk_quit(void);
void vitra_idle_add(void *data);

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload);
void vitra_win_navigate(VitraWin *w, const char *uri);
void vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);
void vitra_win_clear_menu(VitraWin *w);
void vitra_win_add_menu_item(VitraWin *w, const char *menu_label, const char *item_id, const char *item_label);

char *vitra_clip_get(void);
void vitra_clip_set(const char *text);
char *vitra_open_dialog(void);

void vitra_tray_set(const char *tooltip);
void vitra_tray_clear(void);

#endif
