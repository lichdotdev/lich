#ifndef LICH_BROWSER_H
#define LICH_BROWSER_H

#include <X11/Xlib.h>
#include <gtk/gtk.h>
#include <stdint.h>
#include <webkit2/webkit2.h>

// LichPage is what a page's tabs share: cookies and storage, its scripts, its
// downloads. key is the Go page's, which a callback names it by.
typedef struct LichTab LichTab;

typedef struct {
	WebKitWebContext *context;
	WebKitUserContentManager *content;
	WebKitSettings *settings;
	uintptr_t key;
	// popped is the page's own window while it is popped out, showing
	// shown's view.
	GtkWidget *popped;
	LichTab *shown;
} LichPage;

// LichTab is one tab: its X window, which the window reparents into its own,
// and the view GTK draws there. width and height are its size off screen, in
// device pixels.
struct LichTab {
	Display *x;
	Window xid;
	GdkWindow *window;
	GtkWidget *holder;
	GtkWidget *view;
	LichPage *page;
	uintptr_t key;
	int width, height;
	int embedded;
};

void lich_browser_invoke(uintptr_t handle);
int lich_browser_init(void);
void lich_browser_main(void);
void lich_browser_quit(void);
void lich_browser_set_app(unsigned long xid);

LichPage *lich_page_new(uintptr_t key, const char *script);
void lich_page_free(LichPage *page);

LichTab *lich_tab_new(LichPage *page, WebKitWebView *related, uintptr_t key, int width, int height);
void lich_tab_close(LichTab *tab);
void lich_tab_resize(LichTab *tab, int width, int height);
void lich_tab_zoom(LichTab *tab, double scale);
void lich_tab_navigate(LichTab *tab, const char *url);
void lich_tab_reload(LichTab *tab);
void lich_tab_back(LichTab *tab);
void lich_tab_forward(LichTab *tab);
void lich_tab_inspect(LichTab *tab);
void lich_tab_allow(LichTab *tab, const char *leaf, const char *host, const char *uri);
void lich_tab_eval(LichTab *tab, const char *program, const char *body, uintptr_t handle);
int lich_js_parses(const char *code);
void lich_tab_snapshot(LichTab *tab, uintptr_t handle);
void lich_tab_run(LichTab *tab, const char *script);
void lich_tab_draw(LichTab *tab, const char *script, uintptr_t key);
const char *lich_tab_uri(LichTab *tab);
const char *lich_tab_title(LichTab *tab);
int lich_tab_loading(LichTab *tab);
unsigned char *lich_tab_icon(LichTab *tab, int *length);

void lich_tab_focus(LichTab *tab);
void lich_tab_mouse(LichTab *tab, int kind, double x, double y, guint button, guint state, double dx, double dy);
void lich_tab_key(LichTab *tab, int press, guint keyval, guint state);
void lich_tab_text(LichTab *tab, const char *text);
guint lich_keyval_for(gunichar c);

void lich_page_pop_out(LichPage *page, LichTab *tab, int width, int height);
void lich_page_show(LichPage *page, LichTab *tab);
void lich_page_pop_in(LichPage *page);
void lich_page_raise(LichPage *page);

void lich_dialog_answer(WebKitScriptDialog *dialog, int accept, const char *text);
void lich_files_answer(WebKitFileChooserRequest *request, const char **paths, int n);
#endif
