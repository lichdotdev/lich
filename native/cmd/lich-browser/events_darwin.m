#import "engine_darwin.h"

extern void lichLoad(uintptr_t key, int step, char *uri);
extern void lichLoadFailed(uintptr_t key, char *uri, char *error, int download);
extern void lichTabChanged(uintptr_t key, int icon);
extern uintptr_t lichTabCreate(uintptr_t opener);
extern void lichTabCreated(uintptr_t key, LichTab *tab);
extern void lichTabClosing(uintptr_t key);
extern int lichRequestStarted(uintptr_t key, char *uri, char *accept, char *kind);
extern void lichRequestFinished(uintptr_t key, int id);
extern void lichCrashed(uintptr_t key);
extern int lichDialog(uintptr_t key, LichDialog *dialog, int type, char *message, char *text);
extern void lichFiles(uintptr_t key, LichChooser *chooser, int multiple);

// The load steps lichLoad takes, WebKitGTK's numbers.
enum { LOAD_STARTED = 0, LOAD_COMMITTED = 2, LOAD_FINISHED = 3 };

// WebKitErrorFrameLoadInterruptedByPolicyChange: a navigation that turned
// into a download.
#define FRAME_LOAD_INTERRUPTED 102

static void *const changes = (void *)&changes;

static char *uri_of(WKWebView *view) { return (char *)(view.URL.absoluteString ?: @"").UTF8String; }

// LichLoads is the tab's _WKResourceLoadDelegate: WKWebView tells nothing of
// a page's requests otherwise.
@interface LichLoads : NSObject
@property uintptr_t key;
@property(strong) NSMutableDictionary *told;
@end

@implementation LichLoads
// load_id pairs a load's start with its end.
static id load_id(id load) {
	if ([load respondsToSelector:@selector(resourceLoadID)]) {
		return @([(id<LichResourceLoad>)load resourceLoadID]);
	}
	if ([load respondsToSelector:@selector(originalURL)]) {
		return [(id<LichResourceLoad>)load originalURL].absoluteString ?: @"";
	}
	return @"";
}

- (void)webView:(WKWebView *)view resourceLoad:(id)load didSendRequest:(NSURLRequest *)request {
	char *kind = [request.URL isEqual:request.mainDocumentURL] ? "document" : "";
	NSString *accept = [request valueForHTTPHeaderField:@"Accept"] ?: @"";
	int told = lichRequestStarted(self.key, (char *)(request.URL.absoluteString ?: @"").UTF8String, (char *)accept.UTF8String, kind);
	if (told) {
		self.told[load_id(load)] = @(told);
	}
}

- (void)webView:(WKWebView *)view resourceLoad:(id)load didCompleteWithError:(NSError *)error response:(NSURLResponse *)response {
	id key = load_id(load);
	NSNumber *told = self.told[key];
	if (told) {
		[self.told removeObjectForKey:key];
		lichRequestFinished(self.key, told.intValue);
	}
}
@end

void lich_events_connect(LichTabObj *tab) {
	WKWebView *view = tab.view;
	view.navigationDelegate = tab;
	view.UIDelegate = tab;
	if ([view respondsToSelector:@selector(_setResourceLoadDelegate:)]) {
		LichLoads *loads = [LichLoads new];
		loads.key = tab.key;
		loads.told = [NSMutableDictionary new];
		tab.loads = loads;
		[view _setResourceLoadDelegate:loads];
	} else {
		NSLog(@"lich-browser: this WebKit tells no requests, so Settle waits on none");
	}
	for (NSString *path in @[ @"title", @"URL", @"loading" ]) {
		[view addObserver:tab forKeyPath:path options:0 context:changes];
	}
}

void lich_events_disconnect(LichTabObj *tab) {
	WKWebView *view = tab.view;
	for (NSString *path in @[ @"title", @"URL", @"loading" ]) {
		[view removeObserver:tab forKeyPath:path context:changes];
	}
	view.navigationDelegate = nil;
	view.UIDelegate = nil;
	if (tab.loads) {
		[view _setResourceLoadDelegate:nil];
	}
}

@implementation LichTabObj (Events)
- (void)observeValueForKeyPath:(NSString *)path ofObject:(id)object change:(NSDictionary *)change context:(void *)context {
	if (context != changes) {
		[super observeValueForKeyPath:path ofObject:object change:change context:context];
		return;
	}
	if (self.closed) {
		return;
	}
	if ([path isEqualToString:@"title"] && self.page.shown == self) {
		self.page.popped.title = self.view.title ?: @"";
	}
	lichTabChanged(self.key, 0);
}

- (void)webView:(WKWebView *)view didStartProvisionalNavigation:(WKNavigation *)navigation {
	lichLoad(self.key, LOAD_STARTED, uri_of(view));
}

- (void)webView:(WKWebView *)view didCommitNavigation:(WKNavigation *)navigation {
	if (self.icon) {
		self.icon = nil;
		self.iconURL = nil;
		lichTabChanged(self.key, 1);
	}
	lichLoad(self.key, LOAD_COMMITTED, uri_of(view));
}

- (void)webView:(WKWebView *)view didFinishNavigation:(WKNavigation *)navigation {
	lichLoad(self.key, LOAD_FINISHED, uri_of(view));
	lich_fetch_icon(self);
}

- (void)failed:(WKWebView *)view error:(NSError *)error {
	NSString *uri = error.userInfo[NSURLErrorFailingURLStringErrorKey] ?: view.URL.absoluteString ?: @"";
	int download = error.code == FRAME_LOAD_INTERRUPTED;
	lichLoadFailed(self.key, (char *)uri.UTF8String, (char *)error.localizedDescription.UTF8String, download);
}

- (void)webView:(WKWebView *)view didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
	[self failed:view error:error];
}

- (void)webView:(WKWebView *)view didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
	[self failed:view error:error];
}

- (void)webViewWebContentProcessDidTerminate:(WKWebView *)view {
	lichCrashed(self.key);
}

// A navigation the page cannot show, or one the server sends as an
// attachment, lands in the page's downloads as it does in Chrome.
- (void)webView:(WKWebView *)view
    decidePolicyForNavigationAction:(WKNavigationAction *)action
                    decisionHandler:(void (^)(WKNavigationActionPolicy))decision {
	if (@available(macOS 11.3, *)) {
		if (action.shouldPerformDownload) {
			decision(WKNavigationActionPolicyDownload);
			return;
		}
	}
	decision(WKNavigationActionPolicyAllow);
}

- (void)webView:(WKWebView *)view
    decidePolicyForNavigationResponse:(WKNavigationResponse *)response
                      decisionHandler:(void (^)(WKNavigationResponsePolicy))decision {
	NSString *disposition = nil;
	if ([response.response isKindOfClass:NSHTTPURLResponse.class]) {
		disposition = [(NSHTTPURLResponse *)response.response valueForHTTPHeaderField:@"Content-Disposition"];
	}
	BOOL attachment = [disposition.lowercaseString hasPrefix:@"attachment"];
	if (@available(macOS 11.3, *)) {
		if (response.isForMainFrame && (attachment || !response.canShowMIMEType)) {
			decision(WKNavigationResponsePolicyDownload);
			return;
		}
	}
	decision(WKNavigationResponsePolicyAllow);
}

// becameDownload tells the navigation's load failed by the download it
// turned into, as WebKitGTK does; a failure WebKit tells after it is told
// once already.
- (void)becameDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
	lichLoadFailed(self.key, (char *)(download.originalRequest.URL.absoluteString ?: @"").UTF8String,
	               "Frame load interrupted", 1);
	lich_download_start(self, download);
}

- (void)webView:(WKWebView *)view navigationAction:(WKNavigationAction *)action didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
	[self becameDownload:download];
}

- (void)webView:(WKWebView *)view navigationResponse:(WKNavigationResponse *)response didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
	[self becameDownload:download];
}

// A page's window.open or target=_blank link is a tab of the page's, on the
// configuration WebKit hands over, so the two can script each other.
- (WKWebView *)webView:(WKWebView *)view
    createWebViewWithConfiguration:(WKWebViewConfiguration *)config
               forNavigationAction:(WKNavigationAction *)action
                    windowFeatures:(WKWindowFeatures *)features {
	uintptr_t key = lichTabCreate(self.key);
	if (!key || !self.page) {
		return nil;
	}
	LichTabObj *tab = lich_new_tab(self.page, config, key, self.width, self.height);
	lichTabCreated(key, (LichTab *)(__bridge_retained void *)tab);
	return tab.view;
}

- (void)webViewDidClose:(WKWebView *)view {
	lichTabClosing(self.key);
}

// ask holds handler until the wire answers the dialog (lichDialog).
- (void)ask:(int)type message:(NSString *)message text:(NSString *)text handler:(id)handler {
	LichAnswer *answer = [LichAnswer new];
	answer.type = type;
	answer.handler = handler;
	lichDialog(self.key, (LichDialog *)(__bridge_retained void *)answer, type, (char *)(message ?: @"").UTF8String,
	           (char *)(text ?: @"").UTF8String);
}

- (void)webView:(WKWebView *)view
    runJavaScriptAlertPanelWithMessage:(NSString *)message
                      initiatedByFrame:(WKFrameInfo *)frame
                     completionHandler:(void (^)(void))handler {
	[self ask:0 message:message text:nil handler:handler];
}

- (void)webView:(WKWebView *)view
    runJavaScriptConfirmPanelWithMessage:(NSString *)message
                        initiatedByFrame:(WKFrameInfo *)frame
                       completionHandler:(void (^)(BOOL))handler {
	[self ask:1 message:message text:nil handler:handler];
}

- (void)webView:(WKWebView *)view
    runJavaScriptTextInputPanelWithPrompt:(NSString *)prompt
                              defaultText:(NSString *)text
                         initiatedByFrame:(WKFrameInfo *)frame
                        completionHandler:(void (^)(NSString *))handler {
	[self ask:2 message:prompt text:text handler:handler];
}

// WKUIDelegatePrivate's: WebKit asks it for beforeunload.
- (void)_webView:(WKWebView *)view
    runBeforeUnloadConfirmPanelWithMessage:(NSString *)message
                          initiatedByFrame:(WKFrameInfo *)frame
                         completionHandler:(void (^)(BOOL))handler {
	[self ask:3 message:message text:nil handler:handler];
}

- (void)webView:(WKWebView *)view
    runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters
              initiatedByFrame:(WKFrameInfo *)frame
             completionHandler:(void (^)(NSArray<NSURL *> *))handler {
	LichAnswer *answer = [LichAnswer new];
	answer.handler = handler;
	lichFiles(self.key, (LichChooser *)(__bridge_retained void *)answer, parameters.allowsMultipleSelection);
}
@end

// iconScript finds the page's favicon, the one Chrome shows: its last icon
// link, else /favicon.ico of an http page.
static NSString *const iconScript =
    @"(() => { const l = [...document.querySelectorAll('link[rel~=\"icon\" i]')].pop();"
    @" if (l && l.href) return l.href;"
    @" return /^https?:$/.test(location.protocol) ? new URL('/favicon.ico', location.href).href : ''; })()";

static NSData *png_of(NSData *data) {
	NSImage *image = [[NSImage alloc] initWithData:data];
	if (!image) {
		return nil;
	}
	NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithData:image.TIFFRepresentation];
	return [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
}

// lich_fetch_icon reads the page's favicon once a load finished: WKWebView
// keeps none.
void lich_fetch_icon(LichTabObj *tab) {
	__weak LichTabObj *weak = tab;
	[tab.view evaluateJavaScript:iconScript inFrame:nil inContentWorld:WKContentWorld.defaultClientWorld
	           completionHandler:^(id href, NSError *error) {
		           LichTabObj *tab = weak;
		           if (!tab || tab.closed || ![href isKindOfClass:NSString.class] || ![href length] ||
		               [href isEqualToString:tab.iconURL]) {
			           return;
		           }
		           tab.iconURL = href;
		           NSURL *url = [NSURL URLWithString:href];
		           if (!url) {
			           return;
		           }
		           [[NSURLSession.sharedSession dataTaskWithURL:url
		                                      completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
			                                      NSData *png = data ? png_of(data) : nil;
			                                      dispatch_async(dispatch_get_main_queue(), ^{
				                                      LichTabObj *tab = weak;
				                                      if (!png || !tab || tab.closed || ![tab.iconURL isEqualToString:href]) {
					                                      return;
				                                      }
				                                      tab.icon = png;
				                                      lichTabChanged(tab.key, 1);
			                                      });
		                                      }] resume];
	           }];
}
