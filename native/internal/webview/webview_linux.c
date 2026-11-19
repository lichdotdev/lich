#include "webview_linux.h"

#include <X11/Xlib.h>
#include <gdk/gdkx.h>
#include <stdlib.h>
#include <string.h>

extern void lichWebviewRun(uintptr_t handle);

static gboolean run(gpointer data) {
	lichWebviewRun((uintptr_t)data);
	return G_SOURCE_REMOVE;
}

void lich_webview_invoke(uintptr_t handle) { g_idle_add(run, (gpointer)handle); }

static int has_dri3(void) {
	Display *x = gdk_x11_display_get_xdisplay(gdk_display_get_default());
	int opcode, event, error;
	return XQueryExtension(x, "DRI3", &opcode, &event, &error);
}

int lich_webview_init(void) {
	gdk_set_allowed_backends("x11");
	if (!gtk_init_check(NULL, NULL)) {
		return 0;
	}
	// WebKit's DMA-BUF renderer leaves pages blank on an X server without
	// DRI3, such as Xvfb.
	if (!has_dri3()) {
		setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 0);
	}
	return 1;
}

void lich_webview_main(void) { gtk_main(); }

// The window's realize hands GTK the X window we made inside the parent, so
// GTK draws into it instead of making a toplevel of its own; registered, so
// destroying the widget can unrealize it.
static void adopt(GtkWidget *widget, gpointer window) {
	gtk_widget_set_window(widget, GDK_WINDOW(window));
	gtk_widget_register_window(widget, GDK_WINDOW(window));
}

LichPage *lich_webview_open(unsigned long parent, const char *url) {
	GdkDisplay *display = gdk_display_get_default();
	Display *x = gdk_x11_display_get_xdisplay(display);
	Window child = XCreateSimpleWindow(x, parent, 0, 0, 1, 1, 0, 0, 0);
	GdkWindow *window = gdk_x11_window_foreign_new_for_display(display, child);

	GtkWidget *holder = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	g_signal_connect(holder, "realize", G_CALLBACK(adopt), window);
	gtk_widget_set_has_window(holder, TRUE);
	gtk_widget_realize(holder);

	WebKitWebContext *context = webkit_web_context_new_ephemeral();
	GtkWidget *view = webkit_web_view_new_with_context(context);
	g_object_unref(context);
	gtk_container_add(GTK_CONTAINER(holder), view);
	gtk_widget_show_all(holder);
	webkit_web_view_load_uri(WEBKIT_WEB_VIEW(view), url);

	LichPage *page = calloc(1, sizeof(LichPage));
	page->x = x;
	page->child = child;
	page->window = window;
	page->holder = holder;
	page->view = view;
	return page;
}

void lich_webview_place(LichPage *page, int x, int y, int width, int height) {
	GtkAllocation area = {0, 0, width, height};
	gtk_widget_size_allocate(page->holder, &area);
	gtk_widget_size_allocate(page->view, &area);
	XMoveResizeWindow(page->x, page->child, x, y, width, height);
	XMapWindow(page->x, page->child);
	XFlush(page->x);
}

void lich_webview_hide(LichPage *page) {
	XUnmapWindow(page->x, page->child);
	XFlush(page->x);
}

void lich_webview_navigate(LichPage *page, const char *url) { webkit_web_view_load_uri(WEBKIT_WEB_VIEW(page->view), url); }

void lich_webview_close(LichPage *page) {
	gtk_widget_destroy(page->holder);
	XDestroyWindow(page->x, page->child);
	XFlush(page->x);
	free(page);
}
