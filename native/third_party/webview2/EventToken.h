// WebView2.h includes the Windows SDK's EventToken.h, which mingw-w64 (and so
// Zig's Windows headers) lacks; this is its one declaration.
#pragma once
typedef struct EventRegistrationToken {
	__int64 value;
} EventRegistrationToken;
