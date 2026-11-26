#import "engine_darwin.h"

#include <math.h>
#include <stdlib.h>

extern void lichEvaluated(uintptr_t handle, char *json, char *error);
extern void lichSnapshot(uintptr_t handle, unsigned char *data, int width, int height, int stride, char *error);
extern void lichDrawn(uintptr_t key);

static void answer_json(uintptr_t handle, id json) {
	// Anything but a string is undefined, which JSON.stringify leaves out.
	lichEvaluated(handle, [json isKindOfClass:NSString.class] ? (char *)[json UTF8String] : "", NULL);
}

// The page's own words for what it threw, past WebKit's.
static void answer_error(uintptr_t handle, NSError *error) {
	NSString *message = error.userInfo[@"WKJavaScriptExceptionMessage"] ?: error.localizedDescription;
	lichEvaluated(handle, NULL, (char *)message.UTF8String);
}

// run_body runs body as an async function's, in the page's world, as WebKitGTK
// does: the value as JSON, a throw or a rejection as its words.
static void run_body(WKWebView *view, NSString *body, uintptr_t handle) {
	NSString *script = [NSString stringWithFormat:@"try { const v = await (async () => {%@\n})();"
	                                              @" return [0, v === undefined ? undefined : JSON.stringify(v)]; }"
	                                              @" catch (e) { return [2, String(e)]; }",
	                                              body];
	[view callAsyncJavaScript:script arguments:@{} inFrame:nil inContentWorld:WKContentWorld.pageWorld
	        completionHandler:^(id value, NSError *error) {
		        if (error) {
			        answer_error(handle, error);
			        return;
		        }
		        NSArray *got = [value isKindOfClass:NSArray.class] ? value : @[];
		        id second = got.count > 1 ? got[1] : nil;
		        if (got.count > 0 && [got[0] intValue] == 2) {
			        lichEvaluated(handle, NULL, (char *)[[second description] UTF8String]);
			        return;
		        }
		        answer_json(handle, second);
	        }];
}

// lich_tab_eval runs program in the page's own world, then body as an async
// function's, whose value the wire gets; either may be NULL (repl.go). The
// program answers [0, its value as JSON], or [1] once it left a thenable for
// body to await.
void lich_tab_eval(LichTab *t, const char *program, const char *body, uintptr_t handle) {
	WKWebView *view = TAB(t).view;
	NSString *awaiting = body ? lich_string(body) : nil;
	if (!program) {
		run_body(view, awaiting, handle);
		return;
	}
	[view evaluateJavaScript:lich_string(program) inFrame:nil inContentWorld:WKContentWorld.pageWorld
	       completionHandler:^(id value, NSError *error) {
		       if (error) {
			       answer_error(handle, error);
			       return;
		       }
		       NSArray *got = [value isKindOfClass:NSArray.class] ? value : nil;
		       if (!awaiting || !got) {
			       // Run as written, the script parses no way and threw above.
			       answer_json(handle, nil);
			       return;
		       }
		       if (got.count > 0 && [got[0] intValue] == 1) {
			       run_body(view, awaiting, handle);
			       return;
		       }
		       answer_json(handle, got.count > 1 ? got[1] : nil);
	       }];
}

// lich_js_parses is whether code is a script JavaScript parses, by JSC's own
// parser in a context of lich-browser's.
int lich_js_parses(const char *code) {
	static JSGlobalContextRef js;
	if (!js) {
		js = JSGlobalContextCreate(NULL);
	}
	JSStringRef script = JSStringCreateWithUTF8CString(code);
	bool ok = JSCheckScriptSyntax(js, script, NULL, 1, NULL);
	JSStringRelease(script);
	return ok;
}

// device_size is what the tab shows in device pixels: its own size off
// screen, its view's points at its window's scale when popped out.
static void device_size(LichTabObj *tab, int *width, int *height) {
	if (tab.view.window == tab.window) {
		*width = tab.width;
		*height = tab.height;
		return;
	}
	CGFloat scale = tab.view.window.backingScaleFactor;
	*width = (int)lround(tab.view.bounds.size.width * scale);
	*height = (int)lround(tab.view.bounds.size.height * scale);
}

// lich_tab_snapshot draws the part of the page in sight, in sRGB, as cairo's
// ARGB32 lays it out: B, G, R, A premultiplied (snapshot.go).
void lich_tab_snapshot(LichTab *t, uintptr_t handle) {
	LichTabObj *tab = TAB(t);
	int width, height;
	device_size(tab, &width, &height);
	WKSnapshotConfiguration *config = [WKSnapshotConfiguration new];
	config.afterScreenUpdates = YES;
	[tab.view takeSnapshotWithConfiguration:config
	                      completionHandler:^(NSImage *image, NSError *error) {
		                      CGImageRef shot = [image CGImageForProposedRect:NULL context:nil hints:nil];
		                      if (!shot) {
			                      lichSnapshot(handle, NULL, 0, 0, 0,
			                                   (char *)(error.localizedDescription ?: @"no snapshot").UTF8String);
			                      return;
		                      }
		                      int stride = width * 4;
		                      unsigned char *pixels = calloc((size_t)height, (size_t)stride);
		                      CGColorSpaceRef srgb = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
		                      CGContextRef bitmap = CGBitmapContextCreate(pixels, width, height, 8, stride, srgb,
		                                                                  kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little);
		                      CGContextDrawImage(bitmap, CGRectMake(0, 0, width, height), shot);
		                      CGContextRelease(bitmap);
		                      CGColorSpaceRelease(srgb);
		                      lichSnapshot(handle, pixels, width, height, stride, NULL);
		                      free(pixels);
	                      }];
}

void lich_tab_run(LichTab *t, const char *script) {
	[TAB(t).view evaluateJavaScript:lich_string(script) inFrame:nil inContentWorld:WKContentWorld.pageWorld completionHandler:nil];
}

// lich_tab_draw runs script like lich_tab_run, telling the page key once the
// page ran it.
void lich_tab_draw(LichTab *t, const char *script, uintptr_t key) {
	[TAB(t).view evaluateJavaScript:lich_string(script) inFrame:nil inContentWorld:WKContentWorld.pageWorld
	              completionHandler:^(id value, NSError *error) {
		              lichDrawn(key);
	              }];
}
