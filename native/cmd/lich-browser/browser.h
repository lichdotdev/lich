#ifndef LICH_BROWSER_ENGINE_H
#define LICH_BROWSER_ENGINE_H
// The engine's C side, one per system, under the one API the Go side calls.
#ifdef _WIN32
#include "browser_windows.h"
#else
#include "browser_linux.h"
#endif
#endif
