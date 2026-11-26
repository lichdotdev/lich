#import "engine_darwin.h"

#include <math.h>

// The wire's input reaches the page as NSEvents handed to its view, which
// WebKit takes as a person's: the page sees them isTrusted. They go to the
// view itself, never through its window, which would bring lich-browser to
// the front on a click.

// browserwire.Mods' bits.
enum { MOD_SHIFT = 1, MOD_CTRL = 2, MOD_ALT = 4, MOD_META = 8 };

static NSEventModifierFlags flags_of(int mods) {
	NSEventModifierFlags f = 0;
	if (mods & MOD_SHIFT) f |= NSEventModifierFlagShift;
	if (mods & MOD_CTRL) f |= NSEventModifierFlagControl;
	if (mods & MOD_ALT) f |= NSEventModifierFlagOption;
	if (mods & MOD_META) f |= NSEventModifierFlagCommand;
	return f;
}

static NSTimeInterval now(void) { return NSProcessInfo.processInfo.systemUptime; }

void lich_tab_focus(LichTab *t) {
	WKWebView *view = TAB(t).view;
	if (view.window.firstResponder != view) {
		[view.window makeFirstResponder:view];
	}
}

// in_window is x, y, CSS pixels of the view, in its window's coordinates.
static NSPoint in_window(WKWebView *view, double x, double y) {
	CGFloat z = view.pageZoom;
	NSPoint p = NSMakePoint(x * z, y * z);
	if (!view.isFlipped) {
		p.y = view.bounds.size.height - p.y;
	}
	return [view convertPoint:p toView:nil];
}

// pixel_wheel is a wheel event of move's window, at its point, by dx, dy
// points, scale times each field the CGEvent carries.
static NSEvent *pixel_wheel(NSEvent *move, double dx, double dy, double scale) {
	CGEventRef e = CGEventCreateCopy(move.CGEvent);
	CGEventSetType(e, kCGEventScrollWheel);
	CGEventSetIntegerValueField(e, kCGScrollWheelEventIsContinuous, 1);
	CGEventSetIntegerValueField(e, kCGScrollWheelEventPointDeltaAxis1, lround(dy * scale));
	CGEventSetIntegerValueField(e, kCGScrollWheelEventPointDeltaAxis2, lround(dx * scale));
	CGEventSetIntegerValueField(e, kCGScrollWheelEventDeltaAxis1, lround(dy * scale));
	CGEventSetIntegerValueField(e, kCGScrollWheelEventDeltaAxis2, lround(dx * scale));
	CGEventSetDoubleValueField(e, kCGScrollWheelEventFixedPtDeltaAxis1, dy * scale);
	CGEventSetDoubleValueField(e, kCGScrollWheelEventFixedPtDeltaAxis2, dx * scale);
	NSEvent *event = [NSEvent eventWithCGEvent:e];
	CFRelease(e);
	return event;
}

// scroll_event is a pixel wheel event of view's window at at, made from a
// mouse move of that window: NSEvent has no maker for a wheel, and one made
// from a fresh CGEvent names no window, which WebKit drops. AppKit reads its
// deltas through an acceleration of its own (8 points a point on the macOS 15
// runner), so the event is made again at the scale that gives what was
// asked.
static NSEvent *scroll_event(WKWebView *view, NSPoint at, int mods, double dx, double dy) {
	CGFloat z = view.pageZoom;
	NSEvent *move = [NSEvent mouseEventWithType:NSEventTypeMouseMoved location:at modifierFlags:flags_of(mods)
	                                  timestamp:now() windowNumber:view.window.windowNumber context:nil
	                                eventNumber:0 clickCount:0 pressure:0];
	double x = -dx * z, y = -dy * z;
	NSEvent *event = pixel_wheel(move, x, y, 1);
	double got = fabs(y) >= fabs(x) ? event.scrollingDeltaY : event.scrollingDeltaX, want = fabs(y) >= fabs(x) ? y : x;
	if (got != 0 && fabs(got - want) > 0.5) {
		event = pixel_wheel(move, x, y, want / got);
	}
	return event;
}

// wheel waits for the page's next presentation: a wheel sent before WebKit
// committed the page's layers once scrolls nothing (one sent right after a
// load, on the macOS 15 runner).
static void wheel(WKWebView *view, NSPoint at, int mods, double dx, double dy) {
	NSEvent *event = scroll_event(view, at, mods, dx, dy);
	if (![view respondsToSelector:@selector(_doAfterNextPresentationUpdate:)]) {
		[view scrollWheel:event];
		return;
	}
	[view _doAfterNextPresentationUpdate:^{
		[view scrollWheel:event];
	}];
}

// The event types of a press, a release and a drag, by DOM button: 0 left,
// 2 right, any other the system's other buttons.
static const NSEventType downs[] = {NSEventTypeLeftMouseDown, NSEventTypeOtherMouseDown, NSEventTypeRightMouseDown};
static const NSEventType ups[] = {NSEventTypeLeftMouseUp, NSEventTypeOtherMouseUp, NSEventTypeRightMouseUp};
static const NSEventType drags[] = {NSEventTypeLeftMouseDragged, NSEventTypeOtherMouseDragged, NSEventTypeRightMouseDragged};

static int slot(int button) { return button == 0 ? 0 : button == 2 ? 2 : 1; }

// CGMouseButton of a DOM button: 0 left, 1 right, 2 middle, then back and
// forward.
static CGMouseButton cg_button(int button) {
	switch (button) {
	case 0:
		return kCGMouseButtonLeft;
	case 2:
		return kCGMouseButtonRight;
	case 1:
		return kCGMouseButtonCenter;
	}
	return (CGMouseButton)button;
}

static NSEvent *pointer_event(WKWebView *view, NSEventType type, NSPoint at, int button, int clicks, int mods) {
	NSWindow *window = view.window;
	if (slot(button) != 1) {
		return [NSEvent mouseEventWithType:type location:at modifierFlags:flags_of(mods) timestamp:now()
		                      windowNumber:window.windowNumber context:nil eventNumber:0 clickCount:clicks
		                          pressure:(type == downs[slot(button)] || type == drags[slot(button)]) ? 1 : 0];
	}
	// An other button's number goes only by a CGEvent, made from a left
	// button's of the window for the window an NSEvent of its own lacks.
	NSEvent *left = [NSEvent mouseEventWithType:type == downs[1] ? NSEventTypeLeftMouseDown : type == ups[1] ? NSEventTypeLeftMouseUp : NSEventTypeLeftMouseDragged
	                                   location:at modifierFlags:flags_of(mods) timestamp:now() windowNumber:window.windowNumber
	                                    context:nil eventNumber:0 clickCount:clicks pressure:0];
	CGEventRef e = CGEventCreateCopy(left.CGEvent);
	CGEventSetType(e, (CGEventType)type);
	CGEventSetIntegerValueField(e, kCGMouseEventButtonNumber, cg_button(button));
	NSEvent *event = [NSEvent eventWithCGEvent:e];
	CFRelease(e);
	return event;
}

// lich_tab_mouse's button is the one held for a move, -1 for none.
void lich_tab_mouse(LichTab *t, int kind, double x, double y, int button, int clicks, int mods, double dx, double dy) {
	WKWebView *view = TAB(t).view;
	if (!view.window) {
		return;
	}
	NSPoint at = in_window(view, x, y);
	switch (kind) {
	case 0: {
		if (button < 0) {
			[view mouseMoved:pointer_event(view, NSEventTypeMouseMoved, at, 0, 0, mods)];
			return;
		}
		NSEvent *drag = pointer_event(view, drags[slot(button)], at, button, 0, mods);
		switch (slot(button)) {
		case 0:
			[view mouseDragged:drag];
			break;
		case 2:
			[view rightMouseDragged:drag];
			break;
		default:
			[view otherMouseDragged:drag];
		}
		return;
	}
	case 1: {
		NSEvent *down = pointer_event(view, downs[slot(button)], at, button, clicks, mods);
		switch (slot(button)) {
		case 0:
			[view mouseDown:down];
			break;
		case 2:
			[view rightMouseDown:down];
			break;
		default:
			[view otherMouseDown:down];
		}
		return;
	}
	case 2: {
		NSEvent *up = pointer_event(view, ups[slot(button)], at, button, clicks, mods);
		switch (slot(button)) {
		case 0:
			[view mouseUp:up];
			break;
		case 2:
			[view rightMouseUp:up];
			break;
		default:
			[view otherMouseUp:up];
		}
		return;
	}
	case 3:
		wheel(view, at, mods, dx, dy);
	}
}

void lich_tab_key(LichTab *t, int press, int key, const char *chars, const char *bare, int mods) {
	WKWebView *view = TAB(t).view;
	NSWindow *window = view.window;
	if (!chars) {
		NSEvent *flags = [NSEvent keyEventWithType:NSEventTypeFlagsChanged location:NSZeroPoint modifierFlags:flags_of(mods)
		                                 timestamp:now() windowNumber:window.windowNumber context:nil characters:@""
		                charactersIgnoringModifiers:@"" isARepeat:NO keyCode:(unsigned short)key];
		[view flagsChanged:flags];
		return;
	}
	NSEvent *e = [NSEvent keyEventWithType:press ? NSEventTypeKeyDown : NSEventTypeKeyUp location:NSZeroPoint
	                         modifierFlags:flags_of(mods) timestamp:now() windowNumber:window.windowNumber context:nil
	                            characters:lich_string(chars) charactersIgnoringModifiers:lich_string(bare)
	                             isARepeat:NO keyCode:(unsigned short)key];
	if (press) {
		[view keyDown:e];
	} else {
		[view keyUp:e];
	}
}

// lich_tab_text commits text the way an input method does once composing
// ends: one insertion, with no key for each character.
void lich_tab_text(LichTab *t, const char *text) {
	[(id<NSTextInputClient>)TAB(t).view insertText:lich_string(text) replacementRange:NSMakeRange(NSNotFound, 0)];
}
