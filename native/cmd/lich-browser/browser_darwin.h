#ifndef LICH_BROWSER_H
#define LICH_BROWSER_H

#include <stdint.h>

// The WKWebView engine's C side. Every type is opaque to Go: the objects
// behind them are Objective-C, kept alive by the C side until freed.
//
// A tab is a WKWebView in an off-screen window of lich-browser's, whose layer
// a CAContext exports; its xid is that context's id, which the window shows
// in a CALayerHost (docs/lich-browser.md).
typedef struct LichPage LichPage;
typedef struct LichTab LichTab;
typedef struct LichDialog LichDialog;
typedef struct LichChooser LichChooser;

void lich_browser_invoke(uintptr_t handle);
int lich_browser_init(void);
void lich_browser_main(void);
void lich_browser_quit(void);
void lich_browser_set_app(unsigned long window);

LichPage *lich_page_new(uintptr_t key, const char *script);
void lich_page_free(LichPage *page);
int lich_page_popped(LichPage *page);
void lich_free(void *p);

// related is unused: a page's own window.open makes its tab (lichTabCreate).
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
void lich_tab_eval(LichTab *tab, const char *program, const char *body, uintptr_t handle);
int lich_js_parses(const char *code);
void lich_tab_snapshot(LichTab *tab, uintptr_t handle);
void lich_tab_run(LichTab *tab, const char *script);
void lich_tab_draw(LichTab *tab, const char *script, uintptr_t key);
const char *lich_tab_uri(LichTab *tab);
const char *lich_tab_title(LichTab *tab);
int lich_tab_loading(LichTab *tab);
unsigned char *lich_tab_icon(LichTab *tab, int *length);

// Input: x, y are CSS pixels of the view; button is the DOM's; mods are
// browserwire.Mods. kind is 0 move, 1 down, 2 up, 3 wheel.
void lich_tab_focus(LichTab *tab);
void lich_tab_mouse(LichTab *tab, int kind, double x, double y, int button, int clicks, int mods, double dx, double dy);
// key is a macOS virtual key code; chars what the key types, NULL for a
// modifier, which goes as a change of the flags instead.
void lich_tab_key(LichTab *tab, int press, int key, const char *chars, const char *bare, int mods);
void lich_tab_text(LichTab *tab, const char *text);

void lich_page_pop_out(LichPage *page, LichTab *tab, int width, int height);
void lich_page_show(LichPage *page, LichTab *tab);
void lich_page_pop_in(LichPage *page);
void lich_page_raise(LichPage *page);

void lich_dialog_answer(LichDialog *dialog, int accept, const char *text);
void lich_files_answer(LichChooser *request, const char **paths, int n);
#endif
