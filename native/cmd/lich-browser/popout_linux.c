#include "browser_linux.h"

extern void lichClosed(uintptr_t page);

void lich_tab_fit(LichTab *tab);

// The person closing the window ends the page: the wire tells the backend,
// and Go closes the page, this window with it.
static gboolean closing(GtkWidget *window, GdkEvent *event, gpointer page) {
	lichClosed(((LichPage *)page)->key);
	return TRUE;
}

// move puts view into parent, keeping it alive in between: the same view,
// so its page goes on as it was.
static void move(GtkWidget *view, GtkWidget *parent) {
	g_object_ref(view);
	gtk_container_remove(GTK_CONTAINER(gtk_widget_get_parent(view)), view);
	gtk_container_add(GTK_CONTAINER(parent), view);
	g_object_unref(view);
	gtk_widget_show(view);
}

// lich_page_pop_out shows tab's view in a window of the page's own, width x
// height device pixels.
void lich_page_pop_out(LichPage *page, LichTab *tab, int width, int height) {
	GdkDisplay *display = gdk_display_get_default();
	// Wayland has no primary monitor.
	GdkMonitor *monitor = gdk_display_get_primary_monitor(display);
	if (!monitor) {
		monitor = gdk_display_get_monitor(display, 0);
	}
	int scale = monitor ? gdk_monitor_get_scale_factor(monitor) : 1;
	page->popped = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_default_size(GTK_WINDOW(page->popped), width / scale, height / scale);
	g_signal_connect(page->popped, "delete-event", G_CALLBACK(closing), page);
	move(tab->view, page->popped);
	page->shown = tab;
	gtk_widget_show_all(page->popped);
}

// lich_page_show puts tab in the popped-out window, the tab there before back
// in its own.
void lich_page_show(LichPage *page, LichTab *tab) {
	if (page->shown == tab) {
		return;
	}
	if (page->shown) {
		LichTab *back = page->shown;
		move(back->view, back->holder);
		lich_tab_fit(back);
	}
	move(tab->view, page->popped);
	page->shown = tab;
}

void lich_page_pop_in(LichPage *page) {
	LichTab *tab = page->shown;
	move(tab->view, tab->holder);
	lich_tab_fit(tab);
	gtk_widget_destroy(page->popped);
	page->popped = NULL;
	page->shown = NULL;
}

void lich_page_raise(LichPage *page) { gtk_window_present(GTK_WINDOW(page->popped)); }
