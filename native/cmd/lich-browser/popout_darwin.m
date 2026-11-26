#import "engine_darwin.h"

extern void lichClosed(uintptr_t page);

@implementation LichPageObj (Popped)
// The person closing the window ends the page: the wire tells the backend,
// and Go closes the page, this window with it.
- (BOOL)windowShouldClose:(NSWindow *)window {
	lichClosed(self.key);
	return NO;
}
@end

// back puts a tab shown in the popped-out window back in its own.
static void back(LichTabObj *tab) {
	lich_put(tab, tab.holder);
	lich_export(tab);
}

static void show(LichPageObj *page, LichTabObj *tab) {
	lich_put(tab, page.popped.contentView);
	page.shown = tab;
	page.popped.title = tab.view.title ?: @"";
}

// lich_page_pop_out shows tab's view in a window of the page's own, width x
// height device pixels.
void lich_page_pop_out(LichPage *p, LichTab *t, int width, int height) {
	LichPageObj *page = PAGE(p);
	CGFloat scale = NSScreen.mainScreen.backingScaleFactor;
	NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, width / scale, height / scale)
	                                               styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
	                                                         NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
	                                                 backing:NSBackingStoreBuffered
	                                                   defer:NO];
	window.releasedWhenClosed = NO;
	window.delegate = page;
	[window center];
	page.popped = window;
	show(page, TAB(t));
	lich_page_raise(p);
}

// lich_page_show puts tab in the popped-out window, the tab there before back
// in its own.
void lich_page_show(LichPage *p, LichTab *t) {
	LichPageObj *page = PAGE(p);
	if (page.shown == TAB(t)) {
		return;
	}
	if (page.shown) {
		back(page.shown);
	}
	show(page, TAB(t));
}

void lich_page_pop_in(LichPage *p) {
	LichPageObj *page = PAGE(p);
	back(page.shown);
	page.shown = nil;
	page.popped.delegate = nil;
	[page.popped orderOut:nil];
	[page.popped close];
	page.popped = nil;
}

void lich_page_raise(LichPage *p) {
	[NSApp activateIgnoringOtherApps:YES];
	[PAGE(p).popped makeKeyAndOrderFront:nil];
}
