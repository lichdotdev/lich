#include "embed_linux.h"

#include <X11/Xutil.h>
#include <X11/extensions/shape.h>
#include <stdio.h>

// tolerate logs an X error instead of Xlib's default, which ends the
// process: a page is lich-browser's window, and one it closed between a
// browser-state and the frame that places it fails every call on it.
static int tolerate(Display *x, XErrorEvent *e) {
	char text[128];
	XGetErrorText(x, e->error_code, text, sizeof text);
	fprintf(stderr, "webview: X error on window 0x%lx: %s\n", e->resourceid, text);
	return 0;
}

void lich_embed_init(void) { XSetErrorHandler(tolerate); }

// lich_embed takes page into parent, in the save set: a window that dies
// hands its pages back to the root instead of destroying them.
void lich_embed(Display *x, unsigned long parent, unsigned long page, int left, int top) {
	XReparentWindow(x, page, parent, left, top);
	XAddToSaveSet(x, page);
	XFlush(x);
}

// lich_embed_release hands page back to the root, where lich-browser puts it
// off screen again.
void lich_embed_release(Display *x, unsigned long page) {
	XUnmapWindow(x, page);
	XReparentWindow(x, page, DefaultRootWindow(x), 0, 0);
	XRemoveFromSaveSet(x, page);
	XFlush(x);
}

void lich_embed_place(Display *x, unsigned long page, int left, int top, int width, int height) {
	XMoveResizeWindow(x, page, left, top, width, height);
	XMapWindow(x, page);
	XFlush(x);
}

void lich_embed_hide(Display *x, unsigned long page) {
	XUnmapWindow(x, page);
	XFlush(x);
}

// lich_embed_shape leaves the page window out of rects, page-local, where the
// window draws over the page, so the window's drawing shows there; none gives
// the page its whole window back. Unflushed: the frame the window presents
// next flushes it, so X takes the cut and the frame that draws there
// together, and XWayland shows them in one commit. Sent apart, the cut showed
// the background until the frame came. The shape is one request (a region):
// setting the whole window first and taking the rects out after exposed
// every cut anew.
void lich_embed_shape(Display *x, unsigned long page, int width, int height, XRectangle *rects, int n) {
	if (n == 0) {
		XShapeCombineMask(x, page, ShapeBounding, 0, 0, None, ShapeSet);
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
	XShapeCombineRegion(x, page, ShapeBounding, 0, 0, shape, ShapeSet);
	XDestroyRegion(cut);
	XDestroyRegion(shape);
}

// An empty input shape lets the pointer through the page to the window under
// it; no input shape takes it back.
void lich_embed_pointer(Display *x, unsigned long page, int to_page) {
	if (to_page) {
		XShapeCombineMask(x, page, ShapeInput, 0, 0, None, ShapeSet);
	} else {
		XShapeCombineRectangles(x, page, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	}
	XFlush(x);
}

// lich_embed_keyboard gives the keyboard back to the window from a page a
// click gave it to.
void lich_embed_keyboard(Display *x, unsigned long window) {
	XSetInputFocus(x, window, RevertToParent, CurrentTime);
	XFlush(x);
}

// lich_embed_ground is the color X paints where a page leaves the window:
// the window's own drawing never covers what a child window hid, so a page
// that shrank left black there until the window's next frame.
void lich_embed_ground(Display *x, unsigned long window, unsigned long color) {
	XSetWindowBackground(x, window, color);
	XFlush(x);
}
