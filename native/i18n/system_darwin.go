//go:build darwin && cgo

package i18n

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static int firstPreferredLanguage(char *buf, int size) {
	CFArrayRef langs = CFLocaleCopyPreferredLanguages();
	if (langs == NULL) return 0;
	int ok = 0;
	if (CFArrayGetCount(langs) > 0) {
		CFStringRef tag = (CFStringRef)CFArrayGetValueAtIndex(langs, 0);
		ok = CFStringGetCString(tag, buf, size, kCFStringEncodingUTF8);
	}
	CFRelease(langs);
	return ok;
}
*/
import "C"

import "unsafe"

// languageTagMax holds any BCP 47 tag macOS reports ("zh-Hans-CN").
const languageTagMax = 64

// systemTag is the first of the user's preferred languages: an app opened
// from Finder or the Dock gets no LANG, so the environment says nothing there.
func systemTag() string {
	buf := (*C.char)(C.malloc(languageTagMax))
	defer C.free(unsafe.Pointer(buf))
	if C.firstPreferredLanguage(buf, languageTagMax) == 0 {
		return ""
	}
	return C.GoString(buf)
}
