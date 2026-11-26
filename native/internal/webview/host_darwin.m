#import <AppKit/AppKit.h>
#import <QuartzCore/QuartzCore.h>

#include <stdlib.h>
#include <string.h>

#include "host_darwin.h"

// CALayerHost is QuartzCore's: it shows another process's CAContext by its id.
// Private API, as Chromium shows its GPU process's layers.
@interface CALayerHost : CALayer
@property uint32_t contextId;
@end

@interface LichHostObj : NSObject
@property(strong) CALayer *box;
@property(strong) CALayerHost *host;
@end

@implementation LichHostObj
@end

// Gio's window lays out on its own goroutine while AppKit draws on the main
// thread: every change to a layer goes there, in order, after the frame.
static void on_main(dispatch_block_t work) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[CATransaction begin];
		[CATransaction setDisableActions:YES];
		work();
		[CATransaction commit];
	});
}

uint64_t lich_host_window(uintptr_t view) { return (uint64_t)((__bridge NSView *)(void *)view).window.windowNumber; }

LichHost *lich_host_new(uint32_t context) {
	LichHostObj *h = [LichHostObj new];
	h.box = [CALayer layer];
	h.box.masksToBounds = YES;
	h.box.anchorPoint = CGPointZero;
	h.box.hidden = YES;
	h.host = [CALayerHost layer];
	h.host.contextId = context;
	h.host.anchorPoint = CGPointZero;
	h.host.position = CGPointZero;
	[h.box addSublayer:h.host];
	return (LichHost *)(__bridge_retained void *)h;
}

// lich_host_place shows the page over at, in the window's pixels from its top
// left, only over shown, local to at. lich-browser draws a page a point to a
// device pixel (lich_fit), so the host scales it down to the window's points.
void lich_host_place(LichHost *p, uintptr_t v, LichRect at, const LichRect *shown, int n) {
	LichHostObj *h = (__bridge LichHostObj *)(void *)p;
	NSView *view = (__bridge NSView *)(void *)v;
	LichRect *rects = malloc(sizeof(LichRect) * (n > 0 ? n : 1));
	memcpy(rects, shown, sizeof(LichRect) * n);
	on_main(^{
		CALayer *parent = view.layer;
		CGFloat scale = view.window.backingScaleFactor ?: 1;
		if (h.box.superlayer != parent) {
			[h.box removeFromSuperlayer];
			[parent addSublayer:h.box];
		}
		CGFloat w = at.width / scale, ht = at.height / scale, y = at.y / scale;
		if (!parent.geometryFlipped) {
			y = parent.bounds.size.height - y - ht;
		}
		h.box.frame = CGRectMake(at.x / scale, y, w, ht);
		h.host.transform = CATransform3DMakeScale(1 / scale, 1 / scale, 1);
		CAShapeLayer *mask = [CAShapeLayer layer];
		CGMutablePathRef path = CGPathCreateMutable();
		for (int i = 0; i < n; i++) {
			LichRect r = rects[i];
			CGPathAddRect(path, NULL, CGRectMake(r.x / scale, ht - (r.y + r.height) / scale, r.width / scale, r.height / scale));
		}
		mask.path = path;
		CGPathRelease(path);
		h.box.mask = mask;
		h.box.hidden = NO;
		free(rects);
	});
}

void lich_host_hide(LichHost *p) {
	LichHostObj *h = (__bridge LichHostObj *)(void *)p;
	on_main(^{
		h.box.hidden = YES;
	});
}

void lich_host_free(LichHost *p) {
	LichHostObj *h = (__bridge_transfer LichHostObj *)(void *)p;
	on_main(^{
		[h.box removeFromSuperlayer];
	});
}
