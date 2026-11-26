#ifndef LICH_BROWSER_H
#define LICH_BROWSER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// The WebView2 engine (browser_windows.cpp) behind the API browser_linux.h
// declares. A tab is a WebView2 controller in a window of lich-browser's own,
// which the window makes a child of its own (docs/lich-browser.md): xid is
// that window's HWND. WebView2 makes a view asynchronously, so a tab takes
// requests at once and runs them once its view is ready.
typedef struct LichTab LichTab;
typedef struct LichPage LichPage;
typedef struct LichDialog LichDialog;

void lich_browser_invoke(uintptr_t handle);
// lich_browser_init loads WebView2 with its data under folder; it answers
// NULL, or why it could not, malloc'd.
char *lich_browser_init(const char *folder);
void lich_browser_main(void);
void lich_browser_quit(void);
void lich_browser_set_app(uint64_t hwnd);

LichPage *lich_page_new(uintptr_t key, const char *script);
void lich_page_free(LichPage *page);
int lich_page_popped(LichPage *page);
void lich_free(void *p);

LichTab *lich_tab_new(LichPage *page, void *related, uintptr_t key, int width, int height);
void lich_tab_close(LichTab *tab);
uint64_t lich_tab_xid(LichTab *tab);
void lich_tab_close_inspector(LichTab *tab);
void lich_tab_resize(LichTab *tab, int width, int height);
void lich_tab_zoom(LichTab *tab, double scale);
void lich_tab_navigate(LichTab *tab, const char *url);
void lich_tab_reload(LichTab *tab);
void lich_tab_back(LichTab *tab);
void lich_tab_forward(LichTab *tab);
void lich_tab_inspect(LichTab *tab);
void lich_tab_run(LichTab *tab, const char *script);
void lich_tab_draw(LichTab *tab, const char *script, uintptr_t key);
const char *lich_tab_uri(LichTab *tab);
const char *lich_tab_title(LichTab *tab);
int lich_tab_loading(LichTab *tab);
unsigned char *lich_tab_icon(LichTab *tab, int *length);
// lich_tab_cdp calls a DevTools protocol method of the tab's, answered with
// lichCDPDone(handle, ok, json).
void lich_tab_cdp(LichTab *tab, const char *method, const char *params, uintptr_t handle);

void lich_page_pop_out(LichPage *page, LichTab *tab, int width, int height);
void lich_page_show(LichPage *page, LichTab *tab);
void lich_page_pop_in(LichPage *page);
void lich_page_raise(LichPage *page);

void lich_dialog_answer(LichDialog *dialog, int accept, const char *text);

#ifdef __cplusplus
}
#endif
#endif
