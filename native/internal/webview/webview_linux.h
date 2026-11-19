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
} LichPage;

void lich_webview_invoke(uintptr_t handle);
int lich_webview_init(void);
void lich_webview_main(void);
LichPage *lich_webview_open(unsigned long parent, const char *url);
void lich_webview_place(LichPage *page, int x, int y, int width, int height);
void lich_webview_hide(LichPage *page);
void lich_webview_navigate(LichPage *page, const char *url);
void lich_webview_close(LichPage *page);
void lich_webview_keyboard_to_window(void);
