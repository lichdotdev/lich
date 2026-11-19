package browser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	// iconSide is the edge, in pixels, a page's icon is drawn at: the tab
	// shows it at 12dp, 24px on a 2x screen.
	iconSide = 32
	// iconMaxBytes caps the PNG a page hands back; a 32px one is a few KB.
	iconMaxBytes = 64 << 10
	// iconFresh is how long an icon address's answer, an icon or none, is
	// reused: an icon edited, or a dev server that was down, shows on a load
	// after it.
	iconFresh = time.Minute
	// iconWait bounds one icon's load in the page, tried iconTries times.
	iconWait  = 3 * time.Second
	iconTries = 2
	// iconsKept bounds the cache; past it, it starts over.
	iconsKept = 256
)

var errNoIcon = errors.New("no such icon; the page's icon changed or was dropped from the cache")

// iconCandidates lists the addresses a page's icon may be at, best first: the
// icons it declares, scalable or 32px and up before smaller ones, then its
// touch icon, then its origin's /favicon.ico.
const iconCandidates = `(() => {
  const ok = u => /^(https?:|data:image\/)/.test(u);
  const score = l => {
    const sizes = l.sizes ? l.sizes.value : '';
    if (/svg/.test(l.type) || /\bany\b/.test(sizes) || /\.svg(\?|#|$)/.test(l.href)) return 0;
    return Math.max(0, ...sizes.split(/\s+/).map(s => parseInt(s, 10) || 0)) >= 32 ? 1 : 2;
  };
  const links = sel => [...document.querySelectorAll(sel)].filter(l => ok(l.href));
  const out = [...links('link[rel~="icon" i]').sort((a, b) => score(a) - score(b)), ...links('link[rel="apple-touch-icon" i]')].map(l => l.href);
  if (/^https?:$/.test(location.protocol)) out.push(location.origin + '/favicon.ico');
  return [...new Set(out)];
})()`

// iconRaster draws the image at an address onto a square canvas, centred,
// and answers it as a PNG data URL, "" when its load did not end in wait. It
// waits on load, not decode(): a detached image's decode() now and then never
// settled (Chromium 157, 1 in 60 reads, measured 2026-11-19). The page decodes it, so every format its
// browser draws (ICO, SVG, WebP) comes out the same; an icon on another
// origin that sends no CORS header taints the canvas and fails. The image sits
// under an element of the window's color scheme once it decoded: an SVG
// icon's prefers-color-scheme follows it once its style is computed, which
// the layout read forces: without it a dark window got the light icon 4 times
// in 6 (Chromium 157, measured 2026-11-19). A headless page's own is light.
const iconRaster = `(async (url, side, wait, scheme) => {
  const host = document.createElement('div');
  host.style.cssText = 'position:fixed;left:-9999px;top:0;color-scheme:' + scheme;
  const img = new Image();
  if (!url.startsWith('data:')) img.crossOrigin = 'anonymous';
  const loaded = await new Promise((ok, no) => {
    img.onload = () => ok(true);
    img.onerror = () => no(new Error('the icon did not load'));
    setTimeout(() => ok(false), wait);
    img.src = url;
  });
  if (!loaded) return '';
  host.appendChild(img);
  document.documentElement.appendChild(host);
  try {
    img.getBoundingClientRect();
    const w = img.naturalWidth || side, h = img.naturalHeight || side, k = side / Math.max(w, h);
    const c = document.createElement('canvas');
    c.width = c.height = side;
    c.getContext('2d').drawImage(img, (side - w * k) / 2, (side - h * k) / 2, w * k, h * k);
    return c.toDataURL('image/png');
  } finally {
    host.remove();
  }
})(%s, %d, %d, %q)`

// iconCache holds the icons pages declared, by color scheme and address and
// by key, for every browser of the backend.
type iconCache struct {
	mu    sync.Mutex
	byURL map[string]iconEntry
	pngs  map[string][]byte
}

// iconEntry is what an icon address gave: the key of its PNG, "" for none.
type iconEntry struct {
	key string
	at  time.Time
}

var icons = &iconCache{byURL: map[string]iconEntry{}, pngs: map[string][]byte{}}

// iconsDark is the window's theme being dark, which the icons are drawn
// for; the window says so with its new-tab page's colors.
var iconsDark atomic.Bool

func iconScheme() string {
	if iconsDark.Load() {
		return "dark"
	}
	return "light"
}

// darkColor says whether a CSS hex color is a dark one; a color it cannot
// read is not.
func darkColor(hex string) bool {
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return false
	}
	return 299*int(r)+587*int(g)+114*int(b) < 128*1000
}

// lookup is the answer url gave within iconFresh of now.
func (c *iconCache) lookup(url string, now time.Time) (key string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byURL[url]
	if !ok || now.Sub(e.at) > iconFresh {
		return "", false
	}
	return e.key, true
}

// store records the PNG url gave, nil for none, and returns its key.
func (c *iconCache) store(url string, pngData []byte, now time.Time) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.byURL) >= iconsKept {
		clear(c.byURL)
		clear(c.pngs)
	}
	key := ""
	if pngData != nil {
		sum := sha256.Sum256(pngData)
		key = hex.EncodeToString(sum[:8])
		c.pngs[key] = pngData
	}
	c.byURL[url] = iconEntry{key: key, at: now}
	return key
}

func (c *iconCache) png(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.pngs[key]
	return b, ok
}

// iconPNG is the PNG in a data URL the page answered, refused unless it is a
// PNG no larger than iconSide: the page's script could answer anything.
func iconPNG(dataURL string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) || len(dataURL) > len(prefix)+base64.StdEncoding.EncodedLen(iconMaxBytes) {
		return nil, errors.New("the page answered no PNG of an icon's size")
	}
	data, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil {
		return nil, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width > iconSide || cfg.Height > iconSide {
		return nil, fmt.Errorf("the icon is %dx%d, over %dpx", cfg.Width, cfg.Height, iconSide)
	}
	return data, nil
}

// readIcon finds t's page's icon once it loaded, and shows it on the tab
// unless the tab navigated past the seen navigation meanwhile. It waits for
// the renderer's load event: Chrome's frameStoppedLoading can come before the
// new document is the one a script reads (Chromium 157, measured 2026-11-19).
func (p *chromePage) readIcon(t *tab, seen int) {
	var urls []string
	if err := pageValue(t, iconCandidates, &urls); err != nil {
		return
	}
	key, scheme := "", iconScheme()
	for _, u := range urls {
		k, ok := icons.lookup(scheme+" "+u, time.Now())
		if !ok {
			data, answered := rasterIcon(t, u, scheme)
			// A navigation cuts the icon's load short: that is no answer.
			if p.tabs.navigations(t) != seen {
				return
			}
			if !answered {
				continue
			}
			k = icons.store(scheme+" "+u, data, time.Now())
		}
		if k != "" {
			key = k
			break
		}
	}
	if p.tabs.setIcon(t, key, seen) {
		p.tabsChanged()
	}
}

// rasterIcon is the icon at url as t's page draws it in scheme, nil when it
// does not load there; not answered when its load never ended.
func rasterIcon(t *tab, url, scheme string) (data []byte, answered bool) {
	arg, err := json.Marshal(url)
	if err != nil {
		return nil, true
	}
	script := fmt.Sprintf(iconRaster, arg, iconSide, iconWait.Milliseconds(), scheme)
	// An image load that neither lands nor fails is retried once: the same
	// SVG loaded a second time stalled now and then (Chromium 157, 1 read in
	// 25, measured 2026-11-19).
	for range iconTries {
		var dataURL string
		if err := pageValue(t, script, &dataURL); err != nil {
			return nil, true
		}
		if dataURL == "" {
			continue
		}
		data, err := iconPNG(dataURL)
		if err != nil {
			return nil, true
		}
		return data, true
	}
	return nil, false
}

// pageValue runs script in t's page and decodes its value into out; a
// promise is awaited.
func pageValue(t *tab, script string, out any) error {
	return runOn(t.ctx, nil, chromedp.ActionFunc(func(ctx context.Context) error {
		res, exc, err := runtime.Evaluate(script).WithReturnByValue(true).WithAwaitPromise(true).Do(ctx)
		if err != nil {
			return err
		}
		if exc != nil {
			return fmt.Errorf("the script threw: %s", exceptionText(exc))
		}
		return json.Unmarshal(res.Value, out)
	}))
}
