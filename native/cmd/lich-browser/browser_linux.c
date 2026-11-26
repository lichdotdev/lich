#include "browser_linux.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <gdk/gdkx.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

extern void lichBrowserRun(uintptr_t handle);
extern void lichDrawn(uintptr_t key);
extern uintptr_t lichDevtoolsKey(uintptr_t inspected);
extern void lichDevtoolsOpened(uintptr_t key, LichTab *tab);
extern void lichDevtoolsClosed(uintptr_t inspected);
extern void lichEvaluated(uintptr_t handle, char *json, char *error);
extern void lichSnapshot(uintptr_t handle, unsigned char *data, int width, int height, int stride, char *error);

void lich_events_connect_page(LichPage *page);
void lich_events_connect_tab(LichTab *tab);
void lich_input_connect_tab(LichTab *tab);

// A tab no window shows sits here, past the left and top of every screen and
// within X's 16-bit coordinates: mapped, so WebKit runs it as a page in
// sight.
#define OFF_SCREEN (-16000)

static gboolean run(gpointer data) {
	lichBrowserRun((uintptr_t)data);
	return G_SOURCE_REMOVE;
}

void lich_browser_invoke(uintptr_t handle) { g_idle_add(run, (gpointer)handle); }

static int has_dri3(void) {
	Display *x = gdk_x11_display_get_xdisplay(gdk_display_get_default());
	int opcode, event, error;
	return XQueryExtension(x, "DRI3", &opcode, &event, &error);
}

int lich_browser_init(void) {
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

void lich_browser_main(void) { gtk_main(); }

void lich_browser_quit(void) { gtk_main_quit(); }

LichPage *lich_page_new(uintptr_t key, const char *script) {
	LichPage *page = calloc(1, sizeof(LichPage));
	page->key = key;
	page->context = webkit_web_context_new_ephemeral();
	// An ephemeral context keeps its icons in memory; enabling them on its
	// data manager alone never loaded one (WebKitGTK 2.54).
	webkit_web_context_set_favicon_database_directory(page->context, NULL);
	page->content = webkit_user_content_manager_new();
	WebKitUserScript *s = webkit_user_script_new(script, WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
	                                             WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START, NULL, NULL);
	webkit_user_content_manager_add_script(page->content, s);
	webkit_user_script_unref(s);
	page->settings = webkit_settings_new();
	webkit_settings_set_enable_developer_extras(page->settings, TRUE);
	webkit_settings_set_enable_write_console_messages_to_stdout(page->settings, FALSE);
	lich_events_connect_page(page);
	return page;
}

void lich_page_free(LichPage *page) {
	g_signal_handlers_disconnect_by_data(page->context, page);
	g_signal_handlers_disconnect_by_data(page->content, page);
	g_object_unref(page->settings);
	g_object_unref(page->content);
	g_object_unref(page->context);
	free(page);
}

// allocate lays the view out over width x height device pixels; GTK lays out
// in its own units, scale factor device pixels each (GDK_SCALE), and a page
// given device pixels there drew twice its window's size, cut to the top left.
static void allocate(LichTab *tab, int width, int height) {
	int scale = gtk_widget_get_scale_factor(tab->holder);
	GtkAllocation area = {0, 0, (width + scale - 1) / scale, (height + scale - 1) / scale};
	gtk_widget_size_allocate(tab->holder, &area);
	gtk_widget_size_allocate(tab->view, &area);
}

// lich_tab_fit lays a view back in its own X window out at that window's
// size, which no ConfigureNotify tells when it did not change.
void lich_tab_fit(LichTab *tab) {
	XWindowAttributes a;
	if (XGetWindowAttributes(tab->x, tab->xid, &a)) {
		allocate(tab, a.width, a.height);
	}
}

// follow sizes the view to its X window, which the window resizes once it
// holds it, and puts a window handed back to the root off screen again.
static GdkFilterReturn follow(GdkXEvent *xevent, GdkEvent *event, gpointer data) {
	LichTab *tab = data;
	XEvent *e = xevent;
	if (e->xany.window != tab->xid) {
		return GDK_FILTER_CONTINUE;
	}
	if (e->type == ConfigureNotify) {
		allocate(tab, e->xconfigure.width, e->xconfigure.height);
	} else if (e->type == ReparentNotify) {
		tab->embedded = e->xreparent.parent != DefaultRootWindow(tab->x);
		if (!tab->embedded) {
			XMoveResizeWindow(tab->x, tab->xid, OFF_SCREEN, OFF_SCREEN, tab->width, tab->height);
			XMapWindow(tab->x, tab->xid);
			XFlush(tab->x);
		}
	}
	return GDK_FILTER_CONTINUE;
}

// The holder's realize hands GTK the X window made for the tab, so GTK draws
// into it instead of making a toplevel of its own; registered, so destroying
// the widget can unrealize it.
static void adopt(GtkWidget *widget, gpointer window) {
	gtk_widget_set_window(widget, GDK_WINDOW(window));
	gtk_widget_register_window(widget, GDK_WINDOW(window));
}

// keep_detached refuses the inspector a place inside the tab's own window,
// which the pane sizes and cuts to the page alone.
static gboolean keep_detached(WebKitWebInspector *inspector, gpointer data) { return TRUE; }

static GtkWidget *new_view(LichPage *page, WebKitWebView *related) {
	if (related) {
		return g_object_new(WEBKIT_TYPE_WEB_VIEW, "related-view", related, "user-content-manager", page->content,
		                    "settings", page->settings, NULL);
	}
	return g_object_new(WEBKIT_TYPE_WEB_VIEW, "web-context", page->context, "user-content-manager", page->content,
	                    "settings", page->settings, NULL);
}

static Window new_window(Display *x, int width, int height) {
	XSetWindowAttributes a = {0};
	a.override_redirect = True;
	Window xid = XCreateWindow(x, DefaultRootWindow(x), OFF_SCREEN, OFF_SCREEN, width, height, 0, CopyFromParent,
	                           InputOutput, CopyFromParent, CWOverrideRedirect, &a);
	XSelectInput(x, xid, StructureNotifyMask);
	return xid;
}

// hold makes a tab of view, in an X window of its own off screen.
static LichTab *hold(LichPage *page, GtkWidget *view, uintptr_t key, int width, int height) {
	GdkDisplay *display = gdk_display_get_default();
	LichTab *tab = calloc(1, sizeof(LichTab));
	tab->x = gdk_x11_display_get_xdisplay(display);
	tab->xid = new_window(tab->x, width, height);
	tab->window = gdk_x11_window_foreign_new_for_display(display, tab->xid);
	tab->page = page;
	tab->key = key;
	tab->width = width;
	tab->height = height;

	tab->holder = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	g_signal_connect(tab->holder, "realize", G_CALLBACK(adopt), tab->window);
	gtk_widget_set_has_window(tab->holder, TRUE);
	gtk_widget_realize(tab->holder);
	tab->view = view;
	gtk_container_add(GTK_CONTAINER(tab->holder), tab->view);
	gtk_widget_show_all(tab->holder);
	gdk_window_ensure_native(gtk_widget_get_window(tab->view));

	// GTK set the window's events its own way on realize; StructureNotify is
	// what follow reads.
	XWindowAttributes attrs;
	XGetWindowAttributes(tab->x, tab->xid, &attrs);
	XSelectInput(tab->x, tab->xid, attrs.your_event_mask | StructureNotifyMask);
	gdk_window_add_filter(tab->window, follow, tab);
	lich_input_connect_tab(tab);
	XMoveResizeWindow(tab->x, tab->xid, OFF_SCREEN, OFF_SCREEN, width, height);
	XMapWindow(tab->x, tab->xid);
	XFlush(tab->x);
	allocate(tab, width, height);
	return tab;
}

static gboolean open_inspector(WebKitWebInspector *inspector, gpointer tab);
static void inspector_closed(WebKitWebInspector *inspector, gpointer tab);

LichTab *lich_tab_new(LichPage *page, WebKitWebView *related, uintptr_t key, int width, int height) {
	LichTab *tab = hold(page, new_view(page, related), key, width, height);
	WebKitWebInspector *inspector = webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(tab->view));
	g_signal_connect(inspector, "attach", G_CALLBACK(keep_detached), NULL);
	g_signal_connect(inspector, "open-window", G_CALLBACK(open_inspector), tab);
	g_signal_connect(inspector, "closed", G_CALLBACK(inspector_closed), tab);
	lich_events_connect_tab(tab);
	return tab;
}

// open_inspector holds the inspector's view in a tab of its own, as Chrome's
// DevTools are a tab of the pane's, instead of the window WebKit opens.
static gboolean open_inspector(WebKitWebInspector *inspector, gpointer data) {
	LichTab *inspected = data;
	uintptr_t key = lichDevtoolsKey(inspected->key);
	if (!key) {
		return FALSE;
	}
	GtkWidget *view = GTK_WIDGET(webkit_web_inspector_get_web_view(inspector));
	LichTab *tab = hold(inspected->page, view, key, inspected->width, inspected->height);
	tab->inspector = 1;
	lichDevtoolsOpened(key, tab);
	return TRUE;
}

static void inspector_closed(WebKitWebInspector *inspector, gpointer tab) { lichDevtoolsClosed(((LichTab *)tab)->key); }

void lich_tab_close_inspector(LichTab *tab) {
	webkit_web_inspector_close(webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(tab->view)));
}

void lich_tab_close(LichTab *tab) {
	gdk_window_remove_filter(tab->window, follow, tab);
	g_signal_handlers_disconnect_by_data(tab->view, tab);
	if (tab->inspector) {
		// The inspector's view is WebKit's, which ends it with the inspector.
		gtk_container_remove(GTK_CONTAINER(tab->holder), tab->view);
	} else {
		g_signal_handlers_disconnect_by_data(webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(tab->view)), tab);
	}
	gtk_widget_destroy(tab->holder);
	XDestroyWindow(tab->x, tab->xid);
	XFlush(tab->x);
	free(tab);
}

// lich_tab_resize is the tab's size while no window holds it; one that holds
// it sizes it itself.
void lich_tab_resize(LichTab *tab, int width, int height) {
	tab->width = width;
	tab->height = height;
	if (tab->embedded) {
		return;
	}
	XResizeWindow(tab->x, tab->xid, width, height);
	XFlush(tab->x);
}

// lich_tab_zoom draws scale device pixels per CSS pixel, as Chrome draws the
// screencast, so the window's pointer and boxes divide by the same scale.
void lich_tab_zoom(LichTab *tab, double scale) {
	webkit_web_view_set_zoom_level(WEBKIT_WEB_VIEW(tab->view), scale / gtk_widget_get_scale_factor(tab->view));
}

void lich_tab_navigate(LichTab *tab, const char *url) { webkit_web_view_load_uri(WEBKIT_WEB_VIEW(tab->view), url); }

void lich_tab_reload(LichTab *tab) { webkit_web_view_reload_bypass_cache(WEBKIT_WEB_VIEW(tab->view)); }

void lich_tab_back(LichTab *tab) { webkit_web_view_go_back(WEBKIT_WEB_VIEW(tab->view)); }

void lich_tab_forward(LichTab *tab) { webkit_web_view_go_forward(WEBKIT_WEB_VIEW(tab->view)); }

void lich_tab_inspect(LichTab *tab) { webkit_web_inspector_show(webkit_web_view_get_inspector(WEBKIT_WEB_VIEW(tab->view))); }

const char *lich_tab_uri(LichTab *tab) {
	const char *uri = webkit_web_view_get_uri(WEBKIT_WEB_VIEW(tab->view));
	return uri ? uri : "";
}

const char *lich_tab_title(LichTab *tab) {
	const char *title = webkit_web_view_get_title(WEBKIT_WEB_VIEW(tab->view));
	return title ? title : "";
}

int lich_tab_loading(LichTab *tab) { return webkit_web_view_is_loading(WEBKIT_WEB_VIEW(tab->view)); }

static cairo_status_t append_png(void *png, const unsigned char *data, unsigned int length) {
	g_byte_array_append(png, data, length);
	return CAIRO_STATUS_SUCCESS;
}

// lich_tab_icon is the tab's favicon as PNG, g_malloc'd, NULL for none.
unsigned char *lich_tab_icon(LichTab *tab, int *length) {
	*length = 0;
	cairo_surface_t *icon = webkit_web_view_get_favicon(WEBKIT_WEB_VIEW(tab->view));
	if (!icon) {
		return NULL;
	}
	GByteArray *png = g_byte_array_new();
	if (cairo_surface_write_to_png_stream(icon, append_png, png) != CAIRO_STATUS_SUCCESS) {
		g_byte_array_free(png, TRUE);
		return NULL;
	}
	*length = png->len;
	return g_byte_array_free(png, FALSE);
}

// lich_tab_allow trusts leaf, a PEM certificate Go verified, for host in the
// page's context, and loads uri again.
void lich_tab_allow(LichTab *tab, const char *leaf, const char *host, const char *uri) {
	GError *error = NULL;
	GTlsCertificate *cert = g_tls_certificate_new_from_pem(leaf, -1, &error);
	if (!cert) {
		fprintf(stderr, "lich-browser: allow %s: %s\n", host, error->message);
		g_error_free(error);
		return;
	}
	WebKitWebView *view = WEBKIT_WEB_VIEW(tab->view);
	webkit_web_context_allow_tls_certificate_for_host(webkit_web_view_get_context(view), cert, host);
	g_object_unref(cert);
	webkit_web_view_load_uri(view, uri);
}

typedef struct {
	WebKitWebView *view;
	char *body;
	uintptr_t handle;
} Eval;

// EVAL_SOURCE names the scripts the wire runs, of an origin of their own, so
// the page sees their errors as another origin's (pageScript leaves those
// out), and the message the wire gets is the page's own past WebKit's
// "source:line:column: ".
#define EVAL_SOURCE "lich://eval"

static const char *page_message(const char *message) {
	if (!g_str_has_prefix(message, EVAL_SOURCE ":")) {
		return message;
	}
	const char *rest = strstr(message + strlen(EVAL_SOURCE ":"), ": ");
	return rest ? rest + 2 : message;
}

// evaluated answers the wire: undefined as no JSON at all, as the wire has it.
static void evaluated(Eval *e, JSCValue *value, GError *error) {
	if (!value) {
		lichEvaluated(e->handle, NULL, (char *)page_message(error->message));
		g_error_free(error);
	} else {
		char *json = jsc_value_is_undefined(value) ? NULL : jsc_value_to_json(value, 0);
		lichEvaluated(e->handle, json ? json : "", NULL);
		g_free(json);
		g_object_unref(value);
	}
	g_object_unref(e->view);
	g_free(e->body);
	g_free(e);
}

static void as_body(GObject *view, GAsyncResult *result, gpointer data) {
	GError *error = NULL;
	JSCValue *value = webkit_web_view_call_async_javascript_function_finish(WEBKIT_WEB_VIEW(view), result, &error);
	evaluated(data, value, error);
}

static void run_body(Eval *e) {
	webkit_web_view_call_async_javascript_function(e->view, e->body, -1, NULL, NULL, EVAL_SOURCE, NULL, as_body, e);
}

// as_program reads the program's [0, value], or runs the body for [1]
// (consoleRun in repl.go).
static void as_program(GObject *view, GAsyncResult *result, gpointer data) {
	Eval *e = data;
	GError *error = NULL;
	JSCValue *value = webkit_web_view_evaluate_javascript_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (!value || !e->body || !jsc_value_is_array(value)) {
		evaluated(e, value, error);
		return;
	}
	JSCValue *pending = jsc_value_object_get_property_at_index(value, 0);
	int thenable = jsc_value_to_int32(pending) == 1;
	g_object_unref(pending);
	if (thenable) {
		g_object_unref(value);
		run_body(e);
		return;
	}
	JSCValue *answer = jsc_value_object_get_property_at_index(value, 1);
	g_object_unref(value);
	evaluated(e, answer, NULL);
}

// lich_tab_eval runs program in the page's own world, then body as an async
// function's, whose value the wire gets; either may be NULL (repl.go).
void lich_tab_eval(LichTab *tab, const char *program, const char *body, uintptr_t handle) {
	Eval *e = g_new0(Eval, 1);
	e->view = g_object_ref(WEBKIT_WEB_VIEW(tab->view));
	e->body = g_strdup(body);
	e->handle = handle;
	if (!program) {
		run_body(e);
		return;
	}
	webkit_web_view_evaluate_javascript(e->view, program, -1, NULL, EVAL_SOURCE, NULL, as_program, e);
}

// lich_js_parses is whether code is a script JavaScript parses, by JSC's own
// parser in a context of lich-browser's.
int lich_js_parses(const char *code) {
	static JSCContext *js;
	if (!js) {
		js = jsc_context_new();
	}
	return jsc_context_check_syntax(js, code, -1, JSC_CHECK_SYNTAX_MODE_SCRIPT, "lich", 1, NULL) ==
	       JSC_CHECK_SYNTAX_RESULT_SUCCESS;
}

static void snapshotted(GObject *view, GAsyncResult *result, gpointer handle) {
	GError *error = NULL;
	cairo_surface_t *shot = webkit_web_view_get_snapshot_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (!shot) {
		lichSnapshot((uintptr_t)handle, NULL, 0, 0, 0, error ? error->message : "no snapshot");
		g_clear_error(&error);
		return;
	}
	cairo_surface_flush(shot);
	lichSnapshot((uintptr_t)handle, cairo_image_surface_get_data(shot), cairo_image_surface_get_width(shot),
	             cairo_image_surface_get_height(shot), cairo_image_surface_get_stride(shot), NULL);
	cairo_surface_destroy(shot);
}

// lich_tab_snapshot draws the part of the page in sight, as WebKit draws it.
void lich_tab_snapshot(LichTab *tab, uintptr_t handle) {
	webkit_web_view_get_snapshot(WEBKIT_WEB_VIEW(tab->view), WEBKIT_SNAPSHOT_REGION_VISIBLE, WEBKIT_SNAPSHOT_OPTIONS_NONE,
	                             NULL, snapshotted, (gpointer)handle);
}

// lich_tab_run runs script in the page's own world for what it does.
void lich_tab_run(LichTab *tab, const char *script) {
	webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(tab->view), script, -1, NULL, EVAL_SOURCE, NULL, NULL, NULL);
}

static void drawn(GObject *view, GAsyncResult *result, gpointer key) {
	GError *error = NULL;
	JSCValue *value = webkit_web_view_evaluate_javascript_finish(WEBKIT_WEB_VIEW(view), result, &error);
	if (value) {
		g_object_unref(value);
	}
	if (error) {
		g_error_free(error);
	}
	lichDrawn((uintptr_t)key);
}

// lich_tab_draw runs script like lich_tab_run, telling the page key once the
// page ran it.
void lich_tab_draw(LichTab *tab, const char *script, uintptr_t key) {
	webkit_web_view_evaluate_javascript(WEBKIT_WEB_VIEW(tab->view), script, -1, NULL, EVAL_SOURCE, NULL, drawn,
	                                    (gpointer)key);
}
