// Package pagescript holds the scripts that read a page for the browser
// pane's notes, run as they are by every engine that draws a page: Chrome for
// the agent's page, and the web view the window holds itself.
package pagescript

import (
	"encoding/json"
	"fmt"
)

// elementScript reads the element under a point: a selector that finds it
// again (an id, or up to two classes that are not a CSS-in-JS hash, with
// :nth-of-type only where a level is not yet unique), a label, the React
// component that drew it when the page is a development build (its file cut
// to the project's src/, or else app/, pages/, components/ or lib/), and its
// markup with every field's value taken out. A component name of one or two
// letters is a production build's minifier, which tells the agent nothing. A
// point on the page's root answers null.
const elementScript = `((x, y) => {
	const visible = ` + visibleScript + `;
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
			const file = f._debugSource.fileName;
			const root = ['/src/', '/app/', '/pages/', '/components/', '/lib/'].map(r => file.indexOf(r)).find(i => i >= 0);
			source = (root === undefined ? file : file.slice(root + 1)) + ':' + f._debugSource.lineNumber;
		}
		const t = f.type;
		if (!t || typeof t === 'string') continue;
		const inner = t.render || t.type || t;
		const name = t.displayName || inner.displayName || inner.name || '';
		if (name.length > 2 && !/Provider|Context|Router|Boundary|^Anonymous$/.test(name)) { component = name; break; }
	}
	const copy = el.cloneNode(true);
	for (const n of [copy, ...copy.querySelectorAll('*')]) {
		if (n.matches('input, textarea, select, option')) { n.removeAttribute('value'); n.textContent = ''; }
		if (n !== copy && n.matches('script, style')) n.remove();
	}
	return {
		label, selector, component, source,
		html: cut(copy.outerHTML.replace(/\s+/g, ' '), 600),
		url: location.href.split(/[?#]/)[0],
		box: visible(el),
	};
})`

// visibleScript is the viewport box of the part of an element that shows: a
// list, sidebar or dialog that scrolls cuts its rows to its own box, and a
// row it hides whole answers an empty box, so a pin is never drawn over what
// stands beside the list. A partly hidden one keeps the part that shows,
// where the person still sees it. The walk stops at a fixed ancestor, which
// no scrolling box above it cuts.
const visibleScript = `(e => {
	const r = e.getBoundingClientRect();
	let x0 = r.left, y0 = r.top, x1 = r.right, y1 = r.bottom;
	for (let a = e; a && a !== document.body && a !== document.documentElement; a = a.parentElement) {
		const s = getComputedStyle(a);
		if (a !== e && (s.overflowX !== 'visible' || s.overflowY !== 'visible')) {
			const c = a.getBoundingClientRect(), left = c.left + a.clientLeft, top = c.top + a.clientTop;
			x0 = Math.max(x0, left); y0 = Math.max(y0, top);
			x1 = Math.min(x1, left + a.clientWidth); y1 = Math.min(y1, top + a.clientHeight);
		}
		if (s.position === 'fixed') break;
	}
	if (x1 <= x0 || y1 <= y0) return {x: 0, y: 0, w: 0, h: 0};
	return {x: Math.round(x0), y: Math.round(y0), w: Math.round(x1 - x0), h: Math.round(y1 - y0)};
})`

// locateScript is the visible box of the element each selector finds, an
// empty one where it finds none. It first scrolls reveal's element, when
// that does not show whole, to the middle of every box it scrolls in, or to
// their top when it is taller than the viewport. The scroll is instant so
// the boxes it answers are where the element stops.
const locateScript = `((selectors, reveal) => {
	const visible = ` + visibleScript + `;
	const find = s => { try { return document.querySelector(s); } catch { return null; } };
	const shown = reveal ? find(reveal) : null;
	if (shown) {
		const r = shown.getBoundingClientRect(), v = visible(shown);
		const whole = v.w === Math.round(r.width) && v.h === Math.round(r.height) && v.w > 0 &&
			r.top >= 0 && r.left >= 0 && r.bottom <= innerHeight && r.right <= innerWidth;
		if (!whole) shown.scrollIntoView({block: r.height > innerHeight ? 'start' : 'center', inline: 'nearest', behavior: 'instant'});
	}
	return selectors.map(s => {
		const e = find(s);
		return e ? visible(e) : {x: 0, y: 0, w: 0, h: 0};
	});
})`

// ElementAt is the call of elementScript at x,y, CSS pixels of the viewport.
// It answers the element as api.BrowserElement's JSON, or null.
func ElementAt(x, y float64) string {
	return fmt.Sprintf("%s(%g, %g)", elementScript, x, y)
}

// Locate is the call of locateScript for selectors, first revealing reveal's
// element. It answers a list of api.BrowserRect's JSON.
func Locate(selectors []string, reveal string) (string, error) {
	arg, err := json.Marshal(selectors)
	if err != nil {
		return "", err
	}
	shown, err := json.Marshal(reveal)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s, %s)", locateScript, arg, shown), nil
}
