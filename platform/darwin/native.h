#ifndef VITRA_DARWIN_NATIVE_H
#define VITRA_DARWIN_NATIVE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct VitraWin VitraWin;

void vitra_app_init(void);
void vitra_app_run(void);
void vitra_app_quit(void);
void vitra_idle_add(void *data);

VitraWin *vitra_win_new(const char *id, const char *title, int width, int height, const char *uri, const char *preload);
void vitra_win_navigate(VitraWin *w, const char *uri);
void vitra_win_eval(VitraWin *w, const char *js);
void vitra_win_close(VitraWin *w);
void vitra_win_free(VitraWin *w);

char *vitra_open_dialog(void);
char *vitra_save_dialog(void);

#ifdef __cplusplus
}
#endif

#endif
