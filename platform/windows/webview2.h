#ifndef VITRA_WEBVIEW2_H
#define VITRA_WEBVIEW2_H

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct VitraWV2 VitraWV2;

VitraWV2 *vitra_wv2_attach(HWND hwnd, const char *id, const char *uri, const char *preload);
void vitra_wv2_navigate(VitraWV2 *wv, const char *uri);
int vitra_wv2_eval(VitraWV2 *wv, const char *js);
void vitra_wv2_resize(VitraWV2 *wv);
void vitra_wv2_free(VitraWV2 *wv);
int vitra_wv2_loader_available(void);

#ifdef __cplusplus
}
#endif

#endif
