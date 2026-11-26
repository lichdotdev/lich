#include "browser_linux.h"

#include <X11/Xlib.h>
#include <gdk/gdkx.h>
#include <math.h>
#include <string.h>

// injecting is set while an event from the wire goes through GTK, which the
// handlers below then leave alone: it is the agent's, not the person's.
static int injecting;

// X sends a key to the window under the pointer when that window is inside
// the one holding the keyboard, and never past the one holding it. So the
// window keeps the keyboard, and a key a page gets only for sitting under the
// pointer goes on to the window; a click into a page gives that page the
// keyboard until the window takes it back.
static LichTab *keyed;
static unsigned long app;

void lich_browser_set_app(uint64_t xid) {
	app = xid;
	keyed = NULL;
}

static gboolean take_keyboard(GtkWidget *view, GdkEvent *event, gpointer data) {
	LichTab *tab = data;
	if (injecting || !tab->embedded) {
		return FALSE;
	}
	keyed = tab;
	XSetInputFocus(tab->x, gdk_x11_window_get_xid(gtk_widget_get_window(view)), RevertToParent, CurrentTime);
	XFlush(tab->x);
	return FALSE;
}

// An input method that answers a key later (fcitx5, ibus) takes the wire's
// key before this handler sees it and hands it back as a new event once it let
// it through, which reaches only this handler: it went to the window, and no
// key of the agent's reached the page (rc.4, measured under fcitx5). The keys
// the wire sent are kept, so they go on to the page when they come back.
#define LET_THROUGH 32

typedef struct {
	guint32 time;
	guint keyval;
	GdkEventType type;
} Key;

static Key let_through[LET_THROUGH];
static int let_next;

static int same_key(Key k, GdkEventKey *e) { return k.time == e->time && k.keyval == e->keyval && k.type == e->type; }

static gboolean was_let_through(GdkEventKey *e) {
	for (int i = 0; i < LET_THROUGH; i++) {
		if (same_key(let_through[i], e)) {
			return TRUE;
		}
	}
	return FALSE;
}

static void let_key_through(GdkEventKey *e) {
	let_through[let_next] = (Key){e->time, e->keyval, e->type};
	let_next = (let_next + 1) % LET_THROUGH;
}

static gboolean pass_key(GtkWidget *holder, GdkEventKey *event, gpointer data) {
	LichTab *tab = data;
	if (injecting || keyed == tab || !app || was_let_through(event)) {
		return FALSE;
	}
	XKeyEvent key = {0};
	key.type = event->type == GDK_KEY_PRESS ? KeyPress : KeyRelease;
	key.display = tab->x;
	key.window = app;
	key.root = DefaultRootWindow(tab->x);
	key.time = event->time;
	key.state = event->state;
	key.keycode = event->hardware_keycode;
	key.same_screen = True;
	XSendEvent(tab->x, app, False, key.type == KeyPress ? KeyPressMask : KeyReleaseMask, (XEvent *)&key);
	XFlush(tab->x);
	return TRUE;
}

static void forget(GtkWidget *widget, gpointer tab) {
	if (keyed == tab) {
		keyed = NULL;
	}
}

void lich_input_connect_tab(LichTab *tab) {
	g_signal_connect(tab->view, "button-press-event", G_CALLBACK(take_keyboard), tab);
	g_signal_connect(tab->holder, "key-press-event", G_CALLBACK(pass_key), tab);
	g_signal_connect(tab->holder, "key-release-event", G_CALLBACK(pass_key), tab);
	g_signal_connect(tab->holder, "destroy", G_CALLBACK(forget), tab);
}

// now is in the X server's milliseconds as far as WebKit cares: it counts a
// double click by the time between presses, and with every event at 0 two
// clicks of the agent's a minute apart made one double click.
static guint32 now(void) { return (guint32)(g_get_monotonic_time() / 1000); }

static void inject(GdkEvent *e, GdkDevice *device) {
	gdk_event_set_device(e, device);
	injecting = 1;
	gtk_main_do_event(e);
	injecting = 0;
	gdk_event_free(e);
}

static GdkSeat *seat(void) { return gdk_display_get_default_seat(gdk_display_get_default()); }

// lich_tab_focus makes the tab's GTK window believe it has the keyboard, as
// it does once the person clicks it: without it, WebKit's page is not focused
// and keys typed into an input go nowhere. It takes nothing from X: the
// person's keyboard stays where it is.
void lich_tab_focus(LichTab *tab) {
	GtkWindow *holder = GTK_WINDOW(gtk_widget_get_toplevel(tab->view));
	if (gtk_window_get_focus(holder) != tab->view) {
		gtk_widget_grab_focus(tab->view);
	}
	if (gtk_window_has_toplevel_focus(holder)) {
		return;
	}
	GdkEvent *e = gdk_event_new(GDK_FOCUS_CHANGE);
	e->focus_change.window = g_object_ref(gtk_widget_get_window(GTK_WIDGET(holder)));
	e->focus_change.in = TRUE;
	inject(e, gdk_seat_get_keyboard(seat()));
}

static GdkEvent *pointer_event(GdkEventType type, GdkWindow *window, double x, double y, guint state) {
	GdkEvent *e = gdk_event_new(type);
	e->any.window = g_object_ref(window);
	switch (type) {
	case GDK_MOTION_NOTIFY:
		e->motion.time = now();
		e->motion.x = e->motion.x_root = x;
		e->motion.y = e->motion.y_root = y;
		e->motion.state = state;
		break;
	case GDK_SCROLL:
		e->scroll.time = now();
		e->scroll.x = e->scroll.x_root = x;
		e->scroll.y = e->scroll.y_root = y;
		e->scroll.state = state;
		e->scroll.direction = GDK_SCROLL_SMOOTH;
		break;
	default:
		e->button.time = now();
		e->button.x = e->button.x_root = x;
		e->button.y = e->button.y_root = y;
		e->button.state = state;
		break;
	}
	return e;
}

// wheel_units is a smooth scroll delta that moves the page css CSS pixels:
// WebKitGTK scrolls one unit by the view's size along that axis to the power
// 2/3, in the view's pixels (Scrollbar::pixelsPerLineStep), which page zoom
// makes zoom times a CSS pixel. Measured: 44 for a 300 px view, 71 for 600.
static double wheel_units(double css, double zoom, int size) {
	int step = (int)pow(size, 2.0 / 3.0);
	return step > 0 ? css * zoom / step : 0;
}

// lich_tab_mouse takes x, y in CSS pixels; kind is 0 move, 1 down, 2 up, 3
// wheel, and button and state are GDK's.
void lich_tab_mouse(LichTab *tab, int kind, double x, double y, guint button, guint state, double dx, double dy) {
	GdkWindow *window = gtk_widget_get_window(tab->view);
	double zoom = webkit_web_view_get_zoom_level(WEBKIT_WEB_VIEW(tab->view));
	static const GdkEventType types[] = {GDK_MOTION_NOTIFY, GDK_BUTTON_PRESS, GDK_BUTTON_RELEASE, GDK_SCROLL};
	if (kind < 0 || kind > 3) {
		return;
	}
	GdkEvent *e = pointer_event(types[kind], window, x * zoom, y * zoom, state);
	if (kind == 1 || kind == 2) {
		e->button.button = button;
	} else if (kind == 3) {
		e->scroll.delta_x = wheel_units(dx, zoom, gtk_widget_get_allocated_width(tab->view));
		e->scroll.delta_y = wheel_units(dy, zoom, gtk_widget_get_allocated_height(tab->view));
	}
	inject(e, gdk_seat_get_pointer(seat()));
}

static int is_modifier(guint keyval) {
	return (keyval >= GDK_KEY_Shift_L && keyval <= GDK_KEY_Hyper_R) || keyval == GDK_KEY_ISO_Level3_Shift;
}

void lich_tab_key(LichTab *tab, int press, guint keyval, guint state) {
	GdkEvent *e = gdk_event_new(press ? GDK_KEY_PRESS : GDK_KEY_RELEASE);
	e->key.window = g_object_ref(gtk_widget_get_window(gtk_widget_get_toplevel(tab->view)));
	e->key.time = now();
	e->key.state = state;
	e->key.keyval = keyval;
	e->key.is_modifier = is_modifier(keyval);
	GdkKeymapKey *keys = NULL;
	gint n = 0;
	if (gdk_keymap_get_entries_for_keyval(gdk_keymap_get_for_display(gdk_display_get_default()), keyval, &keys, &n)) {
		e->key.hardware_keycode = keys[0].keycode;
		e->key.group = keys[0].group;
		g_free(keys);
	}
	char text[8] = {0};
	gunichar c = gdk_keyval_to_unicode(keyval);
	if (c && !(state & GDK_CONTROL_MASK)) {
		g_unichar_to_utf8(c, text);
	}
	e->key.string = g_strdup(text);
	e->key.length = strlen(text);
	let_key_through(&e->key);
	inject(e, gdk_seat_get_keyboard(seat()));
}

// lich_tab_text commits text the way an input method does once composing
// ends: one insertion, with no key for each character.
void lich_tab_text(LichTab *tab, const char *text) {
	WebKitInputMethodContext *im = webkit_web_view_get_input_method_context(WEBKIT_WEB_VIEW(tab->view));
	if (im) {
		g_signal_emit_by_name(im, "committed", text);
	}
}

guint lich_keyval_for(gunichar c) { return gdk_unicode_to_keyval(c); }
