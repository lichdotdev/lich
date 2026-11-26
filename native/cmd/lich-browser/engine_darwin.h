// What the WKWebView engine's Objective-C files share; Go sees only
// browser_darwin.h.
#import <Cocoa/Cocoa.h>
#import <JavaScriptCore/JavaScriptCore.h>
#import <QuartzCore/QuartzCore.h>
#import <WebKit/WebKit.h>

#include "browser_darwin.h"

// CAContext is QuartzCore's: a layer tree of this process's that another
// process shows by its id in a CALayerHost. Private API, as Chromium's GPU
// process uses it (ui/base/cocoa/remote_layer_api.h).
@interface CAContext : NSObject
+ (instancetype)contextWithCGSConnection:(uint32_t)connection options:(NSDictionary *)options;
@property(readonly) uint32_t contextId;
@property(retain) CALayer *layer;
@end

uint32_t CGSMainConnectionID(void);

// WKWebView's private API (WKWebViewPrivate.h), each call guarded by
// respondsToSelector: a WebKit without one degrades instead of crashing.
@interface WKWebView (LichPrivate)
// Off, WebKit takes a window no person sees as occluded and throttles its
// page: timers, requestAnimationFrame, layout.
- (void)_setWindowOcclusionDetectionEnabled:(BOOL)enabled;
// The page draws this many pixels per point whatever screen its window is
// on; 0 gives it back to the window.
- (void)_setOverrideDeviceScaleFactor:(CGFloat)scale;
- (id)_inspector;
- (void)_doAfterNextPresentationUpdate:(void (^)(void))update;
- (void)_setResourceLoadDelegate:(id)delegate;
@end

// _WKInspector, the page's Web Inspector.
@protocol LichInspector
- (void)show;
- (void)close;
- (void)detach;
- (WKWebView *)inspectorWebView;
@end

// _WKResourceLoadInfo, one load the page made.
@protocol LichResourceLoad
- (uint64_t)resourceLoadID;
- (NSURL *)originalURL;
@end

// LichWindow holds a tab off screen. It says it is the key window: WebKit
// focuses a page, and types into it, only in one, and the window it sits in
// must never take the keyboard from the person's own app.
@interface LichWindow : NSWindow
@end

@class LichPageObj;

@interface LichTabObj : NSObject <WKNavigationDelegate, WKUIDelegate>
@property uintptr_t key;
@property(weak) LichPageObj *page;
@property(strong) WKWebView *view;
// window is the tab's own window; holder the view whose layer context
// exports, the view in it.
@property(strong) NSWindow *window;
@property(strong) NSView *holder;
@property(strong) CAContext *context;
// width, height are the tab's size off screen in device pixels, one point
// each; scale is device pixels per CSS pixel.
@property int width, height;
@property double scale;
@property(strong) NSData *icon;
@property(copy) NSString *iconURL;
// inspects is the tab an inspector's tab inspects, nil for a page's tab;
// theirs is the window WebKit opened the inspector in.
@property(weak) LichTabObj *inspects;
@property(strong) NSWindow *theirs;
@property(strong) id loads;
@property BOOL closed;
@property char *uri, *title;
@end

@interface LichPageObj : NSObject <NSWindowDelegate>
@property uintptr_t key;
@property(strong) WKWebViewConfiguration *config;
@property(strong) id messages;
// popped is the page's own window while it is popped out, showing shown.
@property(strong) NSWindow *popped;
@property(strong) LichTabObj *shown;
@end

// LichAnswer is a dialog or file chooser the page waits on.
@interface LichAnswer : NSObject
@property int type;
@property(copy) id handler;
@end

#define TAB(t) ((__bridge LichTabObj *)(void *)(t))
#define PAGE(p) ((__bridge LichPageObj *)(void *)(p))

LichTabObj *lich_new_tab(LichPageObj *page, WKWebViewConfiguration *config, uintptr_t key, int width, int height);
void lich_hold(LichTabObj *tab, WKWebView *view, int width, int height);
void lich_put(LichTabObj *tab, NSView *into);
void lich_fit(LichTabObj *tab);
void lich_export(LichTabObj *tab);
void lich_events_connect(LichTabObj *tab);
void lich_events_disconnect(LichTabObj *tab);
void lich_fetch_icon(LichTabObj *tab);
void lich_download_start(LichTabObj *tab, WKDownload *download) API_AVAILABLE(macos(11.3));
NSString *lich_string(const char *s);
