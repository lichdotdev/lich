package browser

import (
	"context"
	"os"
	"sync"

	"github.com/chromedp/cdproto/network"
	"github.com/lichdotdev/lich/api"
)

// fakePage is a Page that never talks to Chrome. Tests inject it through
// Launcher so the suite stays off the OS boundary.
type fakePage struct {
	mu          sync.Mutex
	url, title  string
	closed      bool
	navigations []string
	clicks      []Target
	typed       []typed
	pressed     []pressed
	reloads     int
	backs       int
	forwards    int
	scrolled    []int
	scrolledAt  []*point
	focused     int
	cookies     []*network.CookieParam
	setCookies  []*network.CookieParam
	clickAt     [2]float64
	onFrame     func(string)
	onInfo      func(url, title string)
	casting     bool
	viewports   []Viewport
	mice        []api.BrowserMouseOptions
	mouseErr    error
	clickErr    error
	// dead is a Chrome whose process ended under the page.
	dead      bool
	keys      []api.BrowserKeyOptions
	texts     []string
	settles   int
	selection string
	element   api.BrowserElement
	pointedAt [2]float64
	boxes     map[string]api.BrowserRect
	located   [][]string
	info      PageInfo
	shotErr   error
	navErr    error
	infoErr   error
}

type typed struct {
	text  string
	clear bool
	t     Target
}

type pressed struct {
	key string
	t   Target
}

func (p *fakePage) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.closed && !p.dead
}

func (p *fakePage) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *fakePage) Navigate(_ context.Context, url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.navErr != nil {
		return p.navErr
	}
	p.url = url
	p.navigations = append(p.navigations, url)
	return nil
}

func (p *fakePage) Settle(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.settles++
	return nil
}

func (p *fakePage) Info(context.Context) (PageInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.infoErr != nil {
		return PageInfo{}, p.infoErr
	}
	info := p.info
	if info.URL == "" {
		info.URL = p.url
	}
	if info.Title == "" {
		info.Title = p.title
	}
	return info, nil
}

func (p *fakePage) Click(_ context.Context, t Target) (x, y float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.clickErr != nil {
		return 0, 0, p.clickErr
	}
	p.clicks = append(p.clicks, t)
	return p.clickAt[0], p.clickAt[1], nil
}

func (p *fakePage) Type(_ context.Context, text string, clear bool, t Target) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.typed = append(p.typed, typed{text: text, clear: clear, t: t})
	return nil
}

func (p *fakePage) Press(_ context.Context, key string, t Target) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pressed = append(p.pressed, pressed{key: key, t: t})
	return nil
}

func (p *fakePage) Screenshot(_ context.Context, dest string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shotErr != nil {
		return p.shotErr
	}
	return os.WriteFile(dest, []byte("png"), 0o600)
}

func (p *fakePage) Reload(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reloads++
	return nil
}

func (p *fakePage) Back(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backs++
	return nil
}

func (p *fakePage) Forward(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forwards++
	return nil
}

func (p *fakePage) Scroll(_ context.Context, at *point, dy int) (x, y float64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scrolled = append(p.scrolled, dy)
	p.scrolledAt = append(p.scrolledAt, at)
	if at == nil {
		return 400, 300, nil
	}
	return at.X, at.Y, nil
}

func (p *fakePage) Focus(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused++
	return nil
}

func (p *fakePage) Cookies(context.Context) ([]*network.CookieParam, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cookies, nil
}

func (p *fakePage) SetCookies(_ context.Context, cookies []*network.CookieParam) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setCookies = append(p.setCookies, cookies...)
	return nil
}

func (p *fakePage) Listen(onFrame func(string), onInfo func(url, title string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onFrame, p.onInfo = onFrame, onInfo
}

func (p *fakePage) Screencast(_ context.Context, v Viewport) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.casting = true
	p.viewports = append(p.viewports, v)
	return nil
}

func (p *fakePage) StopScreencast(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.casting = false
	return nil
}

func (p *fakePage) Resize(_ context.Context, v Viewport) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.viewports = append(p.viewports, v)
	return nil
}

func (p *fakePage) Mouse(_ context.Context, m api.BrowserMouseOptions) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mouseErr != nil {
		return p.mouseErr
	}
	p.mice = append(p.mice, m)
	return nil
}

func (p *fakePage) Key(_ context.Context, k api.BrowserKeyOptions) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, k)
	return nil
}

func (p *fakePage) Selection(context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.selection, nil
}

func (p *fakePage) InsertText(_ context.Context, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.texts = append(p.texts, text)
	return nil
}

// emitted records what the service pushed to the window, by event name.
type emitted struct {
	mu   sync.Mutex
	sent []sentEvent
}

type sentEvent struct {
	name string
	data any
}

func (e *emitted) emit(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sent = append(e.sent, sentEvent{name, data})
}

func (e *emitted) named(name string) []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []any
	for _, ev := range e.sent {
		if ev.name == name {
			out = append(out, ev.data)
		}
	}
	return out
}

func (e *emitted) states() []api.BrowserState {
	var out []api.BrowserState
	for _, d := range e.named(api.BrowserStateEvent.Name) {
		out = append(out, d.(api.BrowserState))
	}
	return out
}

type fakeLauncher struct {
	mu     sync.Mutex
	pages  []*fakePage
	headed []bool
	dirs   []string
	// gate, when set, runs before the page exists: a test holds a launch open
	// with it to see what else the service lets through meanwhile.
	gate func()
}

func (f *fakeLauncher) launch(dir string, headed bool) (Page, error) {
	if f.gate != nil {
		f.gate()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &fakePage{url: "about:blank"}
	f.pages = append(f.pages, p)
	f.headed = append(f.headed, headed)
	f.dirs = append(f.dirs, dir)
	return p, nil
}

func (p *fakePage) ElementAt(_ context.Context, x, y float64) (api.BrowserElement, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pointedAt = [2]float64{x, y}
	return p.element, nil
}

func (p *fakePage) Locate(_ context.Context, selectors []string) ([]api.BrowserRect, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.located = append(p.located, selectors)
	out := make([]api.BrowserRect, len(selectors))
	for i, s := range selectors {
		out[i] = p.boxes[s]
	}
	return out, nil
}
