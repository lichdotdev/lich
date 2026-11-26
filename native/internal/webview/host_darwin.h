#include <stdint.h>

// A LichHost is one page in the window: a CALayerHost of the page's CAContext,
// in a box that places and clips it over the pane.
typedef struct LichHost LichHost;

typedef struct {
	int x, y, width, height;
} LichRect;

uint64_t lich_host_window(uintptr_t view);
LichHost *lich_host_new(uint32_t context);
void lich_host_place(LichHost *host, uintptr_t view, LichRect at, const LichRect *shown, int n);
void lich_host_hide(LichHost *host);
void lich_host_free(LichHost *host);
