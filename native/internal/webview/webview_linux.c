#include "webview_linux.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/shape.h>
#include <fcntl.h>
#include <gbm.h>
#include <gdk/gdkx.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

extern void lichWebviewRun(uintptr_t handle);
extern void lichWebviewLoad(uintptr_t owner, int step, char *uri);
extern void lichWebviewEvaluated(uintptr_t handle, char *json, char *error);

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

// allocates is whether the render node at path makes the buffers WebKit's
// DMA-BUF renderer maps: NVIDIA's GBM refuses a linear one to render to.
static int allocates(const char *path) {
	int fd = open(path, O_RDWR | O_CLOEXEC);
	if (fd < 0) {
		return 0;
	}
	struct gbm_device *gbm = gbm_create_device(fd);
	struct gbm_bo *bo = gbm ? gbm_bo_create(gbm, 64, 64, GBM_FORMAT_ARGB8888, GBM_BO_USE_RENDERING | GBM_BO_USE_LINEAR) : NULL;
	if (bo) {
		gbm_bo_destroy(bo);
	}
	if (gbm) {
		gbm_device_destroy(gbm);
	}
	close(fd);
	return bo != NULL;
}

// The render nodes the kernel numbers from 128, one per GPU.
#define FIRST_RENDER_NODE 128
#define RENDER_NODES 64

// choose_render_device hands WebKit the first render node that makes its
// buffers. Left to itself it takes the X server's, which on a laptop whose
// XWayland runs on the NVIDIA GPU fails every buffer ("Failed to create GBM
// buffer: Invalid argument") and leaves the page black. With none that makes
// them, pages draw without DMA-BUF.
static void choose_render_device(void) {
	if (getenv("WEBKIT_WEB_RENDER_DEVICE_FILE")) {
		return;
	}
	char path[32];
	for (int n = FIRST_RENDER_NODE; n < FIRST_RENDER_NODE + RENDER_NODES; n++) {
		snprintf(path, sizeof path, "/dev/dri/renderD%d", n);
		if (allocates(path)) {
			setenv("WEBKIT_WEB_RENDER_DEVICE_FILE", path, 0);
			return;
		}
	}
	setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 0);
}

int lich_webview_init(void) {
	// Pages draw on the GPU WebKit picks for itself, not the one prime-run
	// picked for the window, whose variables the web process would inherit.
	unsetenv("__NV_PRIME_RENDER_OFFLOAD");
	unsetenv("__GLX_VENDOR_LIBRARY_NAME");
	unsetenv("__VK_LAYER_NV_optimus");
	gdk_set_allowed_backends("x11");
	if (!gtk_init_check(NULL, NULL)) {
		return 0;
	}
	// WebKit's DMA-BUF renderer leaves pages black on an X server without
	// DRI3, such as Xvfb.
	if (!has_dri3()) {
		setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 0);
	}
	choose_render_device();
	return 1;
}

void lich_webview_main(void) { gtk_main(); }

// X sends a key to the window under the pointer when that window is inside
// the one holding the keyboard, and never past the one holding it. So the
// app's window keeps the keyboard, and a key a page gets only for sitting
// under the pointer goes on to the app; a click into a page gives that page
// the keyboard until the app takes it back.
static LichPage *keyed;
static unsigned long app;

// ground is the color behind the page, which X paints where a page window
// leaves the app's: the app's own drawing never covers what a child window
// hid, so a page that shrank left black there until the app's next frame.
static unsigned long ground;

void lich_webview_set_ground(unsigned long color) {
	ground = color;
	if (!app) {
		return;
	}
	Display *x = gdk_x11_display_get_xdisplay(gdk_display_get_default());
	XSetWindowBackground(x, app, ground);
	XFlush(x);
}

void lich_webview_keyboard_to_window(void) {
	if (!app) {
		return;
	}
	Display *x = gdk_x11_display_get_xdisplay(gdk_display_get_default());
	keyed = NULL;
	XSetInputFocus(x, app, RevertToParent, CurrentTime);
	XFlush(x);
}

static gboolean take_keyboard(GtkWidget *view, GdkEvent *event, gpointer page) {
	LichPage *p = page;
	GdkWindow *window = gtk_widget_get_window(view);
	keyed = p;
	XSetInputFocus(p->x, gdk_x11_window_get_xid(window), RevertToParent, CurrentTime);
	XFlush(p->x);
	return FALSE;
}

static gboolean pass_key(GtkWidget *holder, GdkEventKey *event, gpointer page) {
	LichPage *p = page;
	if (keyed == p) {
		return FALSE;
	}
	XKeyEvent key = {0};
	key.type = event->type == GDK_KEY_PRESS ? KeyPress : KeyRelease;
	key.display = p->x;
	key.window = app;
	key.root = DefaultRootWindow(p->x);
	key.time = event->time;
	key.state = event->state;
	key.keycode = event->hardware_keycode;
	key.same_screen = True;
	XSendEvent(p->x, app, False, key.type == KeyPress ? KeyPressMask : KeyReleaseMask, (XEvent *)&key);
	XFlush(p->x);
	return TRUE;
}

// The window's realize hands GTK the X window we made inside the parent, so
// GTK draws into it instead of making a toplevel of its own; registered, so
// destroying the widget can unrealize it.
static void adopt(GtkWidget *widget, gpointer window) {
	gtk_widget_set_window(widget, GDK_WINDOW(window));
	gtk_widget_register_window(widget, GDK_WINDOW(window));
}

static void load_changed(WebKitWebView *view, WebKitLoadEvent step, gpointer page) {
	lichWebviewLoad(((LichPage *)page)->owner, step, (char *)webkit_web_view_get_uri(view));
}

LichPage *lich_webview_open(unsigned long parent, const char *url, uintptr_t owner) {
	GdkDisplay *display = gdk_display_get_default();
	Display *x = gdk_x11_display_get_xdisplay(display);
	if (app != parent) {
		app = parent;
		XSetWindowBackground(x, app, ground);
	}
	Window child = XCreateSimpleWindow(x, parent, 0, 0, 1, 1, 0, 0, ground);
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

	LichPage *page = calloc(1, sizeof(LichPage));
	page->owner = owner;
	g_signal_connect(view, "load-changed", G_CALLBACK(load_changed), page);
	webkit_web_view_load_uri(WEBKIT_WEB_VIEW(view), url);
	page->x = x;
	page->child = child;
	page->window = window;
	page->holder = holder;
	page->view = view;
	gdk_window_ensure_native(gtk_widget_get_window(view));
	g_signal_connect(view, "button-press-event", G_CALLBACK(take_keyboard), page);
	g_signal_connect(holder, "key-press-event", G_CALLBACK(pass_key), page);
	g_signal_connect(holder, "key-release-event", G_CALLBACK(pass_key), page);
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

// lich_webview_shape leaves the page window out of rects, page-local, where
// the app draws over the page, so the app's drawing shows there; none gives
// the page its whole window back. It goes on x, the app's own connection,
// unflushed: the frame the app presents next flushes it, so X takes the cut
// and the frame that draws there together, and XWayland shows them in one
// commit. Sent apart, the cut showed the background until the frame came. The
// shape is one request (a region): setting the whole window first and taking
// the rects out after exposed every cut anew.
void lich_webview_shape(Display *x, unsigned long child, int width, int height, XRectangle *rects, int n) {
	if (n == 0) {
		XShapeCombineMask(x, child, ShapeBounding, 0, 0, None, ShapeSet);
		return;
	}
	Region shape = XCreateRegion();
	XRectangle whole = {0, 0, width, height};
	XUnionRectWithRegion(&whole, shape, shape);
	Region cut = XCreateRegion();
	for (int i = 0; i < n; i++) {
		XUnionRectWithRegion(&rects[i], cut, cut);
	}
	XSubtractRegion(shape, cut, shape);
	XShapeCombineRegion(x, child, ShapeBounding, 0, 0, shape, ShapeSet);
	XDestroyRegion(cut);
	XDestroyRegion(shape);
}

// An empty input shape lets the pointer through the page to the app's window
// under it; no input shape takes it back.
void lich_webview_pointer_to_window(LichPage *page) {
	XShapeCombineRectangles(page->x, page->child, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	XFlush(page->x);
}

void lich_webview_pointer_to_page(LichPage *page) {
	XShapeCombineMask(page->x, page->child, ShapeInput, 0, 0, None, ShapeSet);
	XFlush(page->x);
}

static void evaluated(GObject *view, GAsyncResult *result, gpointer handle) {
	GError *error = NULL;
	JSCValue *value = webkit_web_view_evaluate_javascript_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (!value) {
		lichWebviewEvaluated((uintptr_t)handle, NULL, error->message);
		g_error_free(error);
		return;
	}
	char *json = jsc_value_to_json(value, 0);
	lichWebviewEvaluated((uintptr_t)handle, json ? json : "null", NULL);
	g_free(json);
	g_object_unref(value);
}

// lich_webview_eval runs script in the page's own world, where its scripts'
// objects are, and hands its value back as JSON.
void lich_webview_eval(LichPage *page, const char *script, uintptr_t handle) {
	webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(page->view), script, -1, NULL, NULL, NULL, evaluated, (gpointer)handle);
}

// lich_webview_run runs script in the page's own world for what it does.
void lich_webview_run(LichPage *page, const char *script) {
	webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(page->view), script, -1, NULL, NULL, NULL, NULL, NULL);
}

// lich_webview_zoom draws scale device pixels per CSS pixel, as Chrome draws
// the screencast, so the window's pointer and boxes divide by the same scale.
void lich_webview_zoom(LichPage *page, double scale) {
	webkit_web_view_set_zoom_level(WEBKIT_WEB_VIEW(page->view), scale / gtk_widget_get_scale_factor(page->view));
}

void lich_webview_hide(LichPage *page) {
	XUnmapWindow(page->x, page->child);
	XFlush(page->x);
}

void lich_webview_navigate(LichPage *page, const char *url) { webkit_web_view_load_uri(WEBKIT_WEB_VIEW(page->view), url); }

void lich_webview_close(LichPage *page) {
	if (keyed == page) {
		keyed = NULL;
	}
	gtk_widget_destroy(page->holder);
	XDestroyWindow(page->x, page->child);
	XFlush(page->x);
	free(page);
}
