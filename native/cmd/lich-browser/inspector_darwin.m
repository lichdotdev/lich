#import "engine_darwin.h"

extern uintptr_t lichDevtoolsKey(uintptr_t inspected);
extern void lichDevtoolsOpened(uintptr_t key, LichTab *tab);
extern void lichDevtoolsClosed(uintptr_t inspected);

// A tab's Web Inspector is a tab of the page's, as Chrome's DevTools are in
// the pane. WebKit opens it docked into the page or in a window of its own;
// detached, its view moves into a tab of lich-browser's, the way WebKitGTK
// hands it over (devtools.go), and WebKit's window stays out of sight. Its
// closing, from the wire or the inspector's own, is that window closing.

// How long WebKit is given to make the inspector's view after show.
#define FIND_TRIES 50
#define FIND_EVERY (100 * NSEC_PER_MSEC)

static id<LichInspector> inspector_of(LichTabObj *tab) {
	WKWebView *view = tab.view;
	return [view respondsToSelector:@selector(_inspector)] ? [view _inspector] : nil;
}

static WKWebView *web_view_in(NSView *view) {
	if ([view isKindOfClass:WKWebView.class]) {
		return (WKWebView *)view;
	}
	for (NSView *sub in view.subviews) {
		WKWebView *found = web_view_in(sub);
		if (found) {
			return found;
		}
	}
	return nil;
}

// inspector_view is the inspector's own WKWebView, nil until WebKit made it.
static WKWebView *inspector_view(LichTabObj *tab, id<LichInspector> inspector) {
	if ([(id)inspector respondsToSelector:@selector(inspectorWebView)]) {
		WKWebView *view = [inspector inspectorWebView];
		// Docked, it is in the page's own window until detach moved it.
		if (view.window && ![view.window isKindOfClass:LichWindow.class]) {
			return view;
		}
		return nil;
	}
	// A WebKit without inspectorWebView: the web view in a window of no tab's.
	for (NSWindow *window in NSApp.windows) {
		if (![window isKindOfClass:LichWindow.class] && window != tab.page.popped) {
			WKWebView *view = web_view_in(window.contentView);
			if (view) {
				return view;
			}
		}
	}
	return nil;
}

static void adopt(LichTabObj *inspected, WKWebView *view) {
	uintptr_t key = lichDevtoolsKey(inspected.key);
	if (!key) {
		return;
	}
	LichTabObj *tab = [LichTabObj new];
	tab.key = key;
	tab.page = inspected.page;
	tab.inspects = inspected;
	tab.scale = inspected.scale;
	tab.view = view;
	tab.theirs = view.window;
	[tab.theirs orderOut:nil];
	lich_hold(tab, view, inspected.width, inspected.height);
	__weak LichTabObj *weak = inspected;
	[NSNotificationCenter.defaultCenter addObserverForName:NSWindowWillCloseNotification
	                                                object:tab.theirs
	                                                 queue:NSOperationQueue.mainQueue
	                                            usingBlock:^(NSNotification *note) {
		                                            LichTabObj *inspected = weak;
		                                            if (inspected && !inspected.closed) {
			                                            lichDevtoolsClosed(inspected.key);
		                                            }
	                                            }];
	lichDevtoolsOpened(key, (LichTab *)(__bridge_retained void *)tab);
}

static void find(LichTabObj *inspected, id<LichInspector> inspector, int tries) {
	if (inspected.closed) {
		return;
	}
	if ([(id)inspector respondsToSelector:@selector(isAttached)] && [inspector isAttached]) {
		[inspector detach];
	}
	WKWebView *view = inspector_view(inspected, inspector);
	if (view) {
		adopt(inspected, view);
		return;
	}
	if (tries == 0) {
		NSLog(@"lich-browser: WebKit opened no inspector");
		return;
	}
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, FIND_EVERY), dispatch_get_main_queue(), ^{
		find(inspected, inspector, tries - 1);
	});
}

void lich_tab_inspect(LichTab *t) {
	LichTabObj *tab = TAB(t);
	id<LichInspector> inspector = inspector_of(tab);
	if (!inspector) {
		NSLog(@"lich-browser: this WebKit has no inspector to open");
		return;
	}
	[inspector show];
	find(tab, inspector, FIND_TRIES);
}

void lich_tab_close_inspector(LichTab *t) { [inspector_of(TAB(t)) close]; }
