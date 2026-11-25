#include <X11/Xlib.h>
#include <gtk/gtk.h>
#include <stdint.h>
#include <webkit2/webkit2.h>

typedef struct {
	Display *x;
	Window child;
	GdkWindow *window;
	GtkWidget *holder;
	GtkWidget *view;
	uintptr_t owner;
} LichPage;

void lich_webview_invoke(uintptr_t handle);
int lich_webview_init(void);
void lich_webview_main(void);
LichPage *lich_webview_open(unsigned long parent, const char *url, uintptr_t owner);
void lich_webview_place(LichPage *page, int x, int y, int width, int height);
void lich_webview_hide(LichPage *page);
void lich_webview_eval(LichPage *page, const char *script, uintptr_t handle);
void lich_webview_run(LichPage *page, const char *script);
void lich_webview_draw(LichPage *page, const char *script, uintptr_t handle);
void lich_webview_snapshot(LichPage *page, uintptr_t handle);
void lich_webview_zoom(LichPage *page, double scale);
void lich_webview_shape(Display *x, unsigned long child, int width, int height, XRectangle *rects, int n);
void lich_webview_pointer_to_window(LichPage *page);
void lich_webview_pointer_to_page(LichPage *page);
void lich_webview_navigate(LichPage *page, const char *url);
void lich_webview_reload(LichPage *page);
void lich_webview_allow(LichPage *page, const char *leaf, const char *host, const char *uri);
void lich_webview_inspect(LichPage *page);
void lich_webview_close(LichPage *page);
void lich_webview_keyboard_to_window(void);
void lich_webview_set_ground(unsigned long color);
