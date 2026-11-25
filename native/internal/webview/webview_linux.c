#include "webview_linux.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/shape.h>
#include <gdk/gdkx.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

extern void lichWebviewRun(uintptr_t handle);
extern void lichWebviewLoad(uintptr_t owner, int step, char *uri);
extern void lichWebviewTLS(uintptr_t owner, char *uri, char *chain);
extern void lichWebviewDrawn(uintptr_t handle);
extern void lichWebviewEvaluated(uintptr_t handle, char *json, char *error);
extern void lichWebviewSnapshot(uintptr_t handle, unsigned char *data, int width, int height, int stride, char *error);

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
	// WebKit's DMA-BUF renderer leaves pages black on an X server without
	// DRI3, such as Xvfb.
	if (!has_dri3()) {
		setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1", 0);
	}
	// On GTK 3 over X11 WebKit asks for linear DMA-BUF buffers, which NVIDIA's
	// GBM refuses ("Failed to create GBM buffer: Invalid argument"): every
	// page stayed black. Shared memory keeps the page on the GPU the web
	// process inherits, the X server's under prime-run; sent to another GPU
	// to dodge the refusal, a page cost the window 125 ms a frame crossing
	// back (hybrid laptop, measured 2026-11-19).
	setenv("WEBKIT_DMABUF_RENDERER_FORCE_SHM", "1", 0);
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

// tls_failed hands Go the chain of a server whose root WebKit cannot find,
// mostly a server that left out its intermediate (see completeChain); WebKit
// shows its error page meanwhile.
static gboolean tls_failed(WebKitWebView *view, char *uri, GTlsCertificate *cert, GTlsCertificateFlags errors,
                           gpointer page) {
	if (errors != G_TLS_CERTIFICATE_UNKNOWN_CA) {
		return FALSE;
	}
	GString *chain = g_string_new(NULL);
	for (GTlsCertificate *c = cert; c; c = g_tls_certificate_get_issuer(c)) {
		char *pem = NULL;
		g_object_get(c, "certificate-pem", &pem, NULL);
		if (pem) {
			g_string_append(chain, pem);
			g_free(pem);
		}
	}
	lichWebviewTLS(((LichPage *)page)->owner, uri, chain->str);
	g_string_free(chain, TRUE);
	return FALSE;
}

// keep_detached refuses the inspector a place inside the page's own window,
// which the pane sizes and cuts to the page alone.
static gboolean keep_detached(WebKitWebInspector *inspector, gpointer data) { return TRUE; }

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
	webkit_settings_set_enable_developer_extras(webkit_web_view_get_settings(WEBKIT_WEB_VIEW(view)), TRUE);
	g_signal_connect(webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(view)), "attach", G_CALLBACK(keep_detached), NULL);
	gtk_container_add(GTK_CONTAINER(holder), view);
	gtk_widget_show_all(holder);

	LichPage *page = calloc(1, sizeof(LichPage));
	page->owner = owner;
	g_signal_connect(view, "load-changed", G_CALLBACK(load_changed), page);
	g_signal_connect(view, "load-failed-with-tls-errors", G_CALLBACK(tls_failed), page);
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

// lich_webview_place takes the window's pixels; GTK lays out in its own units,
// scale factor device pixels each (GNOME's XWayland scale, GDK_SCALE), and a
// page given device pixels there drew twice its window's size, cut to the top
// left.
void lich_webview_place(LichPage *page, int x, int y, int width, int height) {
	int scale = gtk_widget_get_scale_factor(page->holder);
	GtkAllocation area = {0, 0, (width + scale - 1) / scale, (height + scale - 1) / scale};
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

static void drawn(GObject *view, GAsyncResult *result, gpointer handle) {
	GError *error = NULL;
	JSCValue *value = webkit_web_view_evaluate_javascript_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (value) g_object_unref(value);
	if (error) g_error_free(error);
	lichWebviewDrawn((uintptr_t)handle);
}

// lich_webview_draw runs script like lich_webview_run, telling the handle
// once the page ran it.
void lich_webview_draw(LichPage *page, const char *script, uintptr_t handle) {
	webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(page->view), script, -1, NULL, NULL, NULL, drawn, (gpointer)handle);
}

static void snapshotted(GObject *view, GAsyncResult *result, gpointer handle) {
	GError *error = NULL;
	cairo_surface_t *shot = webkit_web_view_get_snapshot_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (!shot) {
		lichWebviewSnapshot((uintptr_t)handle, NULL, 0, 0, 0, error ? error->message : "no snapshot");
		g_clear_error(&error);
		return;
	}
	cairo_surface_flush(shot);
	lichWebviewSnapshot((uintptr_t)handle, cairo_image_surface_get_data(shot), cairo_image_surface_get_width(shot),
	                    cairo_image_surface_get_height(shot), cairo_image_surface_get_stride(shot), NULL);
	cairo_surface_destroy(shot);
}

// lich_webview_snapshot draws the part of the page in sight, as WebKit draws
// it, into an image the window reads.
void lich_webview_snapshot(LichPage *page, uintptr_t handle) {
	webkit_web_view_get_snapshot(WEBKIT_WEB_VIEW(page->view), WEBKIT_SNAPSHOT_REGION_VISIBLE, WEBKIT_SNAPSHOT_OPTIONS_NONE, NULL,
	                             snapshotted, (gpointer)handle);
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

// lich_webview_allow trusts leaf, a PEM certificate Go verified, for host in
// the page's context, and loads uri again.
void lich_webview_allow(LichPage *page, const char *leaf, const char *host, const char *uri) {
	GError *error = NULL;
	GTlsCertificate *cert = g_tls_certificate_new_from_pem(leaf, -1, &error);
	if (!cert) {
		fprintf(stderr, "webview: allow %s: %s\n", host, error->message);
		g_error_free(error);
		return;
	}
	WebKitWebView *view = WEBKIT_WEB_VIEW(page->view);
	webkit_web_context_allow_tls_certificate_for_host(webkit_web_view_get_context(view), cert, host);
	g_object_unref(cert);
	webkit_web_view_load_uri(view, uri);
}

void lich_webview_reload(LichPage *page) { webkit_web_view_reload_bypass_cache(WEBKIT_WEB_VIEW(page->view)); }

void lich_webview_inspect(LichPage *page) { webkit_web_inspector_show(webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(page->view))); }

void lich_webview_close(LichPage *page) {
	if (keyed == page) {
		keyed = NULL;
	}
	gtk_widget_destroy(page->holder);
	XDestroyWindow(page->x, page->child);
	XFlush(page->x);
	free(page);
}
