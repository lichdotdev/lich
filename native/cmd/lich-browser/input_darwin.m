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

// on_screen is a window point in Quartz's global coordinates, from the top
// left of the main screen, which a CGEvent carries.
static CGPoint on_screen(NSWindow *window, NSPoint p) {
	NSPoint s = [window convertPointToScreen:p];
	return CGPointMake(s.x, NSMaxY(NSScreen.screens.firstObject.frame) - s.y);
}

// to_window makes a CGEvent's NSEvent one of window's, at its point there.
static NSEvent *to_window(CGEventRef e, NSWindow *window) {
	CGEventSetIntegerValueField(e, kCGMouseEventWindowUnderMousePointer, window.windowNumber);
	CGEventSetIntegerValueField(e, kCGMouseEventWindowUnderMousePointerThatCanHandleThisEvent, window.windowNumber);
	NSEvent *event = [NSEvent eventWithCGEvent:e];
	CFRelease(e);
	return event;
}

static void wheel(WKWebView *view, NSPoint at, int mods, double dx, double dy) {
	CGFloat z = view.pageZoom;
	// A wheel turned down scrolls the page down: a negative delta, in points.
	CGEventRef e = CGEventCreateScrollWheelEvent2(NULL, kCGScrollEventUnitPixel, 2, (int32_t)lround(-dy * z),
	                                              (int32_t)lround(-dx * z), 0);
	CGEventSetLocation(e, on_screen(view.window, at));
	CGEventSetFlags(e, (CGEventFlags)flags_of(mods));
	[view scrollWheel:to_window(e, view.window)];
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
	// An other button's number goes only by a CGEvent.
	CGEventRef e = CGEventCreateMouseEvent(NULL, (CGEventType)type, on_screen(window, at), cg_button(button));
	CGEventSetIntegerValueField(e, kCGMouseEventClickState, clicks);
	CGEventSetFlags(e, (CGEventFlags)flags_of(mods));
	return to_window(e, window);
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
