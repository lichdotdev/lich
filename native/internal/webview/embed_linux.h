#include <X11/Xlib.h>

void lich_embed_init(void);
void lich_embed(Display *x, unsigned long parent, unsigned long page, int left, int top);
void lich_embed_release(Display *x, unsigned long page);
void lich_embed_place(Display *x, unsigned long page, int left, int top, int width, int height);
void lich_embed_hide(Display *x, unsigned long page);
void lich_embed_shape(Display *x, unsigned long page, int width, int height, XRectangle *rects, int n);
void lich_embed_pointer(Display *x, unsigned long page, int to_page);
void lich_embed_keyboard(Display *x, unsigned long window);
void lich_embed_ground(Display *x, unsigned long window, unsigned long color);
