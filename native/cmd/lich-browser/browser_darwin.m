#import "engine_darwin.h"

#include <stdlib.h>
#include <string.h>

extern void lichBrowserRun(uintptr_t handle);
extern void lichInputSeen(uintptr_t page, char *type);
extern void lichConsole(uintptr_t page, char *text);

// A tab no window shows sits here, past the left and bottom of every screen:
// ordered in, so WebKit runs it as a page in sight.
#define OFF_SCREEN (-16000)

@implementation LichWindow
- (BOOL)isKeyWindow {
	return YES;
}
- (BOOL)canBecomeKeyWindow {
	return YES;
}
@end

@implementation LichAnswer
@end

// LichMessages takes what pageScript posts to the "lich" and "lichInput"
// handlers. Apart from the page: the content controller keeps it.
@interface LichMessages : NSObject <WKScriptMessageHandler>
@property uintptr_t key;
@end

@implementation LichMessages
- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
	char *text = (char *)[[message.body description] UTF8String];
	if ([message.name isEqualToString:@"lich"]) {
		lichConsole(self.key, text);
	} else {
		lichInputSeen(self.key, text);
	}
}
@end

@implementation LichPageObj
@end

@implementation LichTabObj
@end

NSString *lich_string(const char *s) { return s ? [NSString stringWithUTF8String:s] : @""; }

void lich_browser_invoke(uintptr_t handle) {
	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			lichBrowserRun(handle);
		}
	});
}

int lich_browser_init(void) {
	[NSApplication sharedApplication];
	// No Dock tile nor menu bar of its own: the person sees lich's window,
	// and lich-browser's only window of theirs is a popped-out page.
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	return 1;
}

void lich_browser_main(void) { [NSApp run]; }

void lich_browser_quit(void) {
	[NSApp stop:nil];
	// stop takes effect once the run loop handles an event.
	NSEvent *wake = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0
	                                  timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
	[NSApp postEvent:wake atStart:YES];
}

// The window's own NSWindow number goes unused: keys reach a page over the
// wire here, never past a window.
void lich_browser_set_app(uint64_t window) {}

void lich_free(void *p) { free(p); }

LichPage *lich_page_new(uintptr_t key, const char *script) {
	LichPageObj *page = [LichPageObj new];
	page.key = key;
	WKWebViewConfiguration *config = [WKWebViewConfiguration new];
	config.websiteDataStore = [WKWebsiteDataStore nonPersistentDataStore];
	WKUserContentController *content = [WKUserContentController new];
	[content addUserScript:[[WKUserScript alloc] initWithSource:lich_string(script)
	                                              injectionTime:WKUserScriptInjectionTimeAtDocumentStart
	                                           forMainFrameOnly:NO]];
	LichMessages *messages = [LichMessages new];
	messages.key = key;
	[content addScriptMessageHandler:messages name:@"lich"];
	[content addScriptMessageHandler:messages name:@"lichInput"];
	config.userContentController = content;
	// window.open from the agent's eval has no gesture behind it, and opens a
	// tab as Chrome's does.
	config.preferences.javaScriptCanOpenWindowsAutomatically = YES;
	[config.preferences setValue:@YES forKey:@"developerExtrasEnabled"];
	page.config = config;
	page.messages = messages;
	return (LichPage *)(__bridge_retained void *)page;
}

void lich_page_free(LichPage *p) {
	LichPageObj *page = (__bridge_transfer LichPageObj *)(void *)p;
	[page.config.userContentController removeScriptMessageHandlerForName:@"lich"];
	[page.config.userContentController removeScriptMessageHandlerForName:@"lichInput"];
}

int lich_page_popped(LichPage *p) { return PAGE(p).popped != nil; }

LichTab *lich_tab_new(LichPage *p, void *related, uintptr_t key, int width, int height) {
	LichTabObj *tab = lich_new_tab(PAGE(p), PAGE(p).config, key, width, height);
	return (LichTab *)(__bridge_retained void *)tab;
}

// lich_new_tab is a page's tab on a new view of config's, which a page's
// window.open hands over with its opener in it.
LichTabObj *lich_new_tab(LichPageObj *page, WKWebViewConfiguration *config, uintptr_t key, int width, int height) {
	LichTabObj *tab = [LichTabObj new];
	tab.key = key;
	tab.page = page;
	tab.scale = 1;
	WKWebView *view = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, width, height) configuration:config];
	if (@available(macOS 13.3, *)) {
		view.inspectable = YES;
	}
	tab.view = view;
	lich_events_connect(tab);
	lich_hold(tab, view, width, height);
	return tab;
}

static NSWindow *off_screen_window(int width, int height) {
	LichWindow *window = [[LichWindow alloc] initWithContentRect:NSMakeRect(OFF_SCREEN, OFF_SCREEN, width, height)
	                                                   styleMask:NSWindowStyleMaskBorderless
	                                                     backing:NSBackingStoreBuffered
	                                                       defer:NO];
	window.releasedWhenClosed = NO;
	window.ignoresMouseEvents = YES;
	window.hasShadow = NO;
	window.opaque = NO;
	window.backgroundColor = NSColor.clearColor;
	window.excludedFromWindowsMenu = YES;
	// On every Space: a window left on another one is out of sight.
	window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
	                            NSWindowCollectionBehaviorIgnoresCycle;
	window.contentView.wantsLayer = YES;
	return window;
}

// lich_hold makes a tab of view, in a window of its own off screen whose
// holder a CAContext exports.
void lich_hold(LichTabObj *tab, WKWebView *view, int width, int height) {
	tab.width = width;
	tab.height = height;
	tab.window = off_screen_window(width, height);
	NSView *holder = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, width, height)];
	holder.wantsLayer = YES;
	holder.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	[tab.window.contentView addSubview:holder];
	tab.holder = holder;
	if ([view respondsToSelector:@selector(_setWindowOcclusionDetectionEnabled:)]) {
		[view _setWindowOcclusionDetectionEnabled:NO];
	} else {
		NSLog(@"lich-browser: this WebKit throttles a page no person sees");
	}
	lich_put(tab, holder);
	[tab.window orderFrontRegardless];
	tab.context = [CAContext contextWithCGSConnection:CGSMainConnectionID() options:@{}];
	lich_export(tab);
}

// lich_put puts the tab's view in into, sized to it, as its first responder.
void lich_put(LichTabObj *tab, NSView *into) {
	WKWebView *view = tab.view;
	[view removeFromSuperview];
	view.frame = into.bounds;
	view.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	[into addSubview:view];
	[into.window makeFirstResponder:view];
	lich_fit(tab);
}

// lich_fit draws the tab at its scale. In its own window a point is a device
// pixel, whatever screen the window is on, which the window scales back into
// its own points; in a popped-out window it takes that window's.
void lich_fit(LichTabObj *tab) {
	WKWebView *view = tab.view;
	BOOL own = view.window == tab.window;
	if ([view respondsToSelector:@selector(_setOverrideDeviceScaleFactor:)]) {
		[view _setOverrideDeviceScaleFactor:own ? 1 : 0];
	}
	CGFloat unit = own || !view.window ? 1 : view.window.backingScaleFactor;
	view.pageZoom = tab.scale / unit;
}

// lich_export makes the holder's layer the root of the tab's context. AppKit
// put it in the window's tree, which nobody sees; taken out, it is the one
// the window's CALayerHost shows.
void lich_export(LichTabObj *tab) {
	[CATransaction begin];
	[CATransaction setDisableActions:YES];
	CALayer *layer = tab.holder.layer;
	if (layer.superlayer) {
		[layer removeFromSuperlayer];
	}
	if (tab.context.layer != layer) {
		tab.context.layer = layer;
	}
	[CATransaction commit];
}

void lich_tab_close(LichTab *t) {
	LichTabObj *tab = (__bridge_transfer LichTabObj *)(void *)t;
	tab.closed = YES;
	if (!tab.inspects) {
		lich_events_disconnect(tab);
		[tab.view stopLoading];
	}
	[tab.view removeFromSuperview];
	tab.context.layer = nil;
	[tab.window orderOut:nil];
	[tab.window close];
	free(tab.uri);
	free(tab.title);
}

uint64_t lich_tab_xid(LichTab *t) { return TAB(t).context.contextId; }

// lich_tab_resize is the tab's size off screen; a popped-out window sizes
// the view itself.
void lich_tab_resize(LichTab *t, int width, int height) {
	LichTabObj *tab = TAB(t);
	tab.width = width;
	tab.height = height;
	[tab.window setFrame:NSMakeRect(OFF_SCREEN, OFF_SCREEN, width, height) display:NO];
	lich_export(tab);
}

void lich_tab_zoom(LichTab *t, double scale) {
	TAB(t).scale = scale;
	lich_fit(TAB(t));
}

// url_of is s as a URL, its characters a URL cannot hold escaped: a data: URL
// of HTML is no URL as NSURL reads one.
static NSURL *url_of(NSString *s) {
	NSURL *url = [NSURL URLWithString:s];
	if (!url) {
		url = [NSURL URLWithString:[s stringByAddingPercentEncodingWithAllowedCharacters:NSCharacterSet.URLFragmentAllowedCharacterSet]];
	}
	return url;
}

extern void lichLoadFailed(uintptr_t key, char *uri, char *error, int download);

void lich_tab_navigate(LichTab *t, const char *u) {
	LichTabObj *tab = TAB(t);
	NSURL *url = url_of(lich_string(u));
	if (!url) {
		lichLoadFailed(tab.key, (char *)u, "not a URL", 0);
		return;
	}
	if (url.isFileURL) {
		// A file page reads what sits beside it, as it does in Chrome.
		[tab.view loadFileURL:url allowingReadAccessToURL:url.URLByDeletingLastPathComponent];
		return;
	}
	[tab.view loadRequest:[NSURLRequest requestWithURL:url]];
}

void lich_tab_reload(LichTab *t) { [TAB(t).view reloadFromOrigin]; }

void lich_tab_back(LichTab *t) { [TAB(t).view goBack]; }

void lich_tab_forward(LichTab *t) { [TAB(t).view goForward]; }

// kept is s copied into *slot, which holds it until the next call.
static const char *kept(char **slot, NSString *s) {
	free(*slot);
	*slot = strdup(s ? s.UTF8String : "");
	return *slot;
}

const char *lich_tab_uri(LichTab *t) {
	LichTabObj *tab = TAB(t);
	char *slot = tab.uri;
	const char *s = kept(&slot, tab.view.URL.absoluteString);
	tab.uri = slot;
	return s;
}

const char *lich_tab_title(LichTab *t) {
	LichTabObj *tab = TAB(t);
	char *slot = tab.title;
	const char *s = kept(&slot, tab.inspects ? @"Web Inspector" : tab.view.title);
	tab.title = slot;
	return s;
}

int lich_tab_loading(LichTab *t) { return TAB(t).view.loading; }

unsigned char *lich_tab_icon(LichTab *t, int *length) {
	NSData *icon = TAB(t).icon;
	*length = (int)icon.length;
	if (!icon.length) {
		return NULL;
	}
	unsigned char *png = malloc(icon.length);
	memcpy(png, icon.bytes, icon.length);
	return png;
}

void lich_dialog_answer(LichDialog *d, int accept, const char *text) {
	LichAnswer *answer = (__bridge_transfer LichAnswer *)(void *)d;
	switch (answer.type) {
	case 0:
		((void (^)(void))answer.handler)();
		break;
	case 1:
	case 3:
		((void (^)(BOOL))answer.handler)(accept != 0);
		break;
	default:
		// nil is the prompt's null, a dismissal.
		((void (^)(NSString *))answer.handler)(accept ? lich_string(text) : nil);
		break;
	}
}

void lich_files_answer(LichChooser *c, const char **paths, int n) {
	LichAnswer *answer = (__bridge_transfer LichAnswer *)(void *)c;
	NSMutableArray<NSURL *> *urls = nil;
	if (n > 0) {
		urls = [NSMutableArray arrayWithCapacity:n];
		for (int i = 0; i < n; i++) {
			[urls addObject:[NSURL fileURLWithPath:lich_string(paths[i])]];
		}
	}
	((void (^)(NSArray<NSURL *> *))answer.handler)(urls);
}
