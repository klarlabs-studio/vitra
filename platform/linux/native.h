#ifndef VITRA_LINUX_NATIVE_H
#define VITRA_LINUX_NATIVE_H

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

typedef struct {
	GtkWidget *window;
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

char *vitra_clip_get(void);
void vitra_clip_set(const char *text);
char *vitra_open_dialog(void);

#endif
