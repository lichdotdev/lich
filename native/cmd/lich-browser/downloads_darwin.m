#import "engine_darwin.h"

#include <stdlib.h>
#include <unistd.h>

extern char *lichDownloadPath(uintptr_t page, char *suggested);
extern void lichDownload(uintptr_t page, int guid, char *url, char *name, char *path, int state, int64_t received,
                         int64_t total);

enum { IN_PROGRESS, COMPLETED, CANCELED };

static int downloads;

static void *const progressed = (void *)&progressed;

// fetches keeps each download's delegate, which WKDownload holds weakly,
// until it ends.
static NSMutableSet *fetches;

// LichFetch is one download: its page, its id on the wire, and where it
// lands.
API_AVAILABLE(macos(11.3))
@interface LichFetch : NSObject <WKDownloadDelegate>
@property uintptr_t page;
@property int guid;
@property(weak) WKDownload *download;
@property(copy) NSString *name, *path;
@property int64_t total;
@property BOOL watching, done;
@end

@implementation LichFetch
- (void)report:(WKDownload *)download state:(int)state {
	lichDownload(self.page, self.guid, (char *)(download.originalRequest.URL.absoluteString ?: @"").UTF8String,
	             (char *)(self.name ?: @"").UTF8String, (char *)(self.path ?: @"").UTF8String, state,
	             download.progress.completedUnitCount, self.total);
}

- (void)end:(WKDownload *)download state:(int)state {
	if (self.done) {
		return;
	}
	self.done = YES;
	if (self.watching) {
		[download.progress removeObserver:self forKeyPath:@"completedUnitCount" context:progressed];
		self.watching = NO;
	}
	[self report:download state:state];
	LichFetch *fetch = self;
	dispatch_async(dispatch_get_main_queue(), ^{
		[fetches removeObject:fetch];
	});
}

// decideDestination lands the download where Go reserved it. WKDownload
// refuses a file that exists, so the empty one Go made goes first.
//
// shortcut: another download of the same name could take the name in
// between; Chrome's .crdownload holds it instead, worth doing once two
// downloads of one name at once are seen.
- (void)download:(WKDownload *)download
    decideDestinationUsingResponse:(NSURLResponse *)response
                 suggestedFilename:(NSString *)suggested
                 completionHandler:(void (^)(NSURL *))handler {
	char *path = lichDownloadPath(self.page, (char *)(suggested ?: @"").UTF8String);
	if (!path) {
		handler(nil);
		return;
	}
	self.path = lich_string(path);
	self.name = self.path.lastPathComponent;
	free(path);
	unlink(self.path.fileSystemRepresentation);
	self.total = MAX(response.expectedContentLength, 0);
	[download.progress addObserver:self forKeyPath:@"completedUnitCount" options:0 context:progressed];
	self.watching = YES;
	handler([NSURL fileURLWithPath:self.path]);
	[self report:download state:IN_PROGRESS];
}

- (void)observeValueForKeyPath:(NSString *)path ofObject:(id)object change:(NSDictionary *)change context:(void *)context {
	if (context != progressed) {
		[super observeValueForKeyPath:path ofObject:object change:change context:context];
		return;
	}
	dispatch_async(dispatch_get_main_queue(), ^{
		if (!self.done && self.download) {
			[self report:self.download state:IN_PROGRESS];
		}
	});
}

- (void)downloadDidFinish:(WKDownload *)download {
	[self end:download state:COMPLETED];
}

- (void)download:(WKDownload *)download didFailWithError:(NSError *)error resumeData:(NSData *)resume {
	[self end:download state:CANCELED];
}
@end

void lich_download_start(LichTabObj *tab, WKDownload *download) {
	if (!fetches) {
		fetches = [NSMutableSet new];
	}
	LichFetch *fetch = [LichFetch new];
	fetch.page = tab.page.key;
	fetch.guid = ++downloads;
	fetch.download = download;
	[fetches addObject:fetch];
	download.delegate = fetch;
}
