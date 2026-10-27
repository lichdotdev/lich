package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// The pane's notes: the person points at an element of the page and leaves a
// note on it for the agent. The page only reports what is where; the pane
// draws the outline and the pins, so nothing is injected into its DOM.

// elementScript reads the element under a point: a selector that finds it
// again (an id, or up to two classes that are not a CSS-in-JS hash, with
// :nth-of-type only where a level is not yet unique), a label, the React
// component that drew it when the page is a development build, and its
// markup with every field's value taken out. A point on the page's root
// answers null.
const elementScript = `((x, y) => {
	const cut = (s, n) => s.length > n ? s.slice(0, n - 1) + '…' : s;
	let el = document.elementFromPoint(x, y);
	while (el && el.shadowRoot) {
		const inner = el.shadowRoot.elementFromPoint(x, y);
		if (!inner || inner === el) break;
		el = inner;
	}
	if (!el || el === document.documentElement || el === document.body) return null;
	const word = /^[a-zA-Z][\w-]*$/;
	const stable = c => word.test(c) && !/^(css|sc|jsx|emotion|svelte)-|[0-9a-f]{5,}|__/.test(c);
	const part = e => {
		const tag = e.tagName.toLowerCase();
		if (e.id && word.test(e.id)) return tag + '#' + e.id;
		return tag + [...e.classList].filter(stable).slice(0, 2).map(c => '.' + c).join('');
	};
	const root = el.getRootNode();
	const unique = s => { try { return root.querySelectorAll(s).length === 1; } catch { return false; } };
	let selector = '';
	for (let e = el, depth = 0; e && e.nodeType === 1 && depth < 6; e = e.parentElement, depth++) {
		const p = part(e);
		const join = q => selector ? q + ' > ' + selector : q;
		if (unique(join(p))) { selector = join(p); break; }
		const same = e.parentElement ? [...e.parentElement.children].filter(c => c.tagName === e.tagName) : [];
		const q = same.length > 1 && !p.includes('#') ? p + ':nth-of-type(' + (same.indexOf(e) + 1) + ')' : p;
		selector = join(q);
		if (unique(selector)) break;
	}
	const text = (el.getAttribute('aria-label') || el.innerText || el.getAttribute('placeholder') ||
		el.getAttribute('alt') || el.getAttribute('title') || '').trim().split('\n')[0];
	const label = part(el) + (text ? ' "' + cut(text, 32) + '"' : '');
	let component = '', source = '';
	const fiber = Object.keys(el).find(k => k.startsWith('__reactFiber$'));
	for (let f = fiber ? el[fiber] : null; f; f = f.return) {
		if (!source && f._debugSource) {
			source = f._debugSource.fileName.replace(/^.*?\/((src|app|pages|components|lib)\/)/, '$1') + ':' + f._debugSource.lineNumber;
		}
		const t = f.type;
		if (!t || typeof t === 'string') continue;
		const inner = t.render || t.type || t;
		const name = t.displayName || inner.displayName || inner.name || '';
		if (name && !/Provider|Context|Router|Boundary|^Anonymous$/.test(name)) { component = name; break; }
	}
	const copy = el.cloneNode(true);
	for (const n of [copy, ...copy.querySelectorAll('*')]) {
		if (n.matches('input, textarea, select, option')) { n.removeAttribute('value'); n.textContent = ''; }
		if (n !== copy && n.matches('script, style')) n.remove();
	}
	const r = el.getBoundingClientRect();
	return {
		label, selector, component, source,
		html: cut(copy.outerHTML.replace(/\s+/g, ' '), 600),
		url: location.href.split(/[?#]/)[0],
		box: {x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height)},
	};
})`

// locateScript is the viewport box of the element each selector finds, an
// empty one where it finds none.
const locateScript = `(selectors => selectors.map(s => {
	let e = null;
	try { e = document.querySelector(s); } catch {}
	if (!e) return {x: 0, y: 0, w: 0, h: 0};
	const r = e.getBoundingClientRect();
	return {x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height)};
}))`

func (p *chromePage) ElementAt(ctx context.Context, x, y float64) (api.BrowserElement, error) {
	var el *api.BrowserElement
	if err := p.run(ctx, chromedp.Evaluate(fmt.Sprintf("%s(%g, %g)", elementScript, x, y), &el)); err != nil {
		return api.BrowserElement{}, err
	}
	if el == nil {
		return api.BrowserElement{}, nil
	}
	return *el, nil
}

func (p *chromePage) Locate(ctx context.Context, selectors []string) ([]api.BrowserRect, error) {
	arg, err := json.Marshal(selectors)
	if err != nil {
		return nil, err
	}
	boxes := []api.BrowserRect{}
	return boxes, p.run(ctx, chromedp.Evaluate(fmt.Sprintf("%s(%s)", locateScript, arg), &boxes))
}

// ElementAtPoint is the element under a point of this session's page.
func (s *Service) ElementAtPoint(_ context.Context, opts api.BrowserPointOptions) (api.BrowserElement, error) {
	var el api.BrowserElement
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		var err error
		el, err = p.ElementAt(ctx, opts.X, opts.Y)
		return err
	})
	return el, err
}

// LocateElements is where the elements opts.Selectors find stand now.
func (s *Service) LocateElements(_ context.Context, opts api.BrowserLocateOptions) ([]api.BrowserRect, error) {
	var boxes []api.BrowserRect
	_, err := s.steer(opts.ID, inputWait, func(p Page, ctx context.Context) error {
		var err error
		boxes, err = p.Locate(ctx, opts.Selectors)
		return err
	})
	return boxes, err
}
