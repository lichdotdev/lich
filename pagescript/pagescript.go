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
// point on the page's root answers null. A point on a same-origin frame reads
// the element under it inside the frame, and its selector is the frame's and
// the element's joined by FrameSeparator; a cross-origin frame is out of the
// page's reach and answers the frame itself.
const elementScript = `((x, y) => {
	` + Frames + `
	const cut = (s, n) => s.length > n ? s.slice(0, n - 1) + '…' : s;
	const {e: el, frames} = deepest(x, y);
	const doc = el && el.ownerDocument;
	if (!el || el === doc.documentElement || el === doc.body) return null;
	const word = /^[a-zA-Z][\w-]*$/;
	const stable = c => word.test(c) && !/^(css|sc|jsx|emotion|svelte)-|[0-9a-f]{5,}|__/.test(c);
	const part = e => {
		const tag = e.tagName.toLowerCase();
		if (e.id && word.test(e.id)) return tag + '#' + e.id;
		return tag + [...e.classList].filter(stable).slice(0, 2).map(c => '.' + c).join('');
	};
	const path = el => {
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
		return selector;
	};
	const selector = [...frames, el].map(path).join(` + "'" + FrameSeparator + "'" + `);
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
		box: framed(el, frames),
	};
})`

// FrameSeparator joins a frame's selector to the selector of an element
// inside it. No CSS selector holds it, so splitting on it is unambiguous.
const FrameSeparator = " >>> "

// Frames declares what a script needs to see into same-origin frames, whose
// documents a page's own script reaches; a cross-origin one stays the frame
// element. A frame is an iframe or frame element, and frames are a path of
// them, outermost first.
//
//   - visible(e): visibleScript, in e's own document.
//   - content(f): the viewport box of f's content, where its document starts.
//   - offset(frames): where the innermost frame's viewport starts in the top
//     one.
//   - framed(e, frames): visible(e) in the top viewport, cut to each frame's
//     content and to what shows of the frame itself.
//   - find(s): {e, frames} a selector joined by FrameSeparator names, null
//     where any step is gone. An invalid selector throws, as querySelector.
//   - deepest(x, y): {e, frames} under a point of the top viewport, through
//     open shadow roots and same-origin frames.
//   - documents(): every same-origin document of the page, {doc, frames}, the
//     top one first.
//   - focused(): the element that holds the focus, inside frames too.
const Frames = `const visible = ` + visibleScript + `;
	const content = f => {
		const r = f.getBoundingClientRect(), s = f.ownerDocument.defaultView.getComputedStyle(f);
		const pl = parseFloat(s.paddingLeft), pt = parseFloat(s.paddingTop);
		return {x: r.left + f.clientLeft + pl, y: r.top + f.clientTop + pt,
			w: f.clientWidth - pl - parseFloat(s.paddingRight), h: f.clientHeight - pt - parseFloat(s.paddingBottom)};
	};
	const offset = frames => frames.reduce((o, f) => {
		const c = content(f);
		return {x: o.x + c.x, y: o.y + c.y};
	}, {x: 0, y: 0});
	const inside = f => {
		const doc = /^i?frame$/i.test(f.tagName) ? f.contentDocument : null;
		return doc && doc.documentElement ? doc : null;
	};
	const framed = (e, frames) => {
		let b = visible(e);
		for (let i = frames.length - 1; i >= 0 && b.w > 0; i--) {
			const c = content(frames[i]), v = visible(frames[i]);
			const x0 = Math.max(b.x + c.x, c.x, v.x), y0 = Math.max(b.y + c.y, c.y, v.y);
			const x1 = Math.min(b.x + b.w + c.x, c.x + c.w, v.x + v.w), y1 = Math.min(b.y + b.h + c.y, c.y + c.h, v.y + v.h);
			b = x1 <= x0 || y1 <= y0 ? {x: 0, y: 0, w: 0, h: 0} :
				{x: Math.round(x0), y: Math.round(y0), w: Math.round(x1 - x0), h: Math.round(y1 - y0)};
		}
		return b;
	};
	const find = s => {
		const frames = [];
		let doc = document;
		for (const p of s.split(` + "'" + FrameSeparator + "'" + `)) {
			if (!doc) return null;
			const e = doc.querySelector(p);
			if (!e) return null;
			frames.push(e);
			doc = inside(e);
		}
		return {e: frames.pop(), frames};
	};
	const deepest = (x, y) => {
		const frames = [];
		let doc = document, e;
		for (;;) {
			e = doc.elementFromPoint(x, y);
			while (e && e.shadowRoot) {
				const inner = e.shadowRoot.elementFromPoint(x, y);
				if (!inner || inner === e) break;
				e = inner;
			}
			const inner = e && inside(e);
			if (!inner) return {e, frames};
			const c = content(e);
			x -= c.x; y -= c.y;
			frames.push(e);
			doc = inner;
		}
	};
	const documents = () => {
		const all = [];
		const walk = (doc, frames) => {
			all.push({doc, frames});
			for (const f of doc.querySelectorAll('iframe, frame')) {
				const inner = inside(f);
				if (inner) walk(inner, [...frames, f]);
			}
		};
		walk(document, []);
		return all;
	};
	const focused = () => {
		let e = document.activeElement;
		while (e && inside(e) && inside(e).activeElement) e = inside(e).activeElement;
		return e;
	};`

// visibleScript is the viewport box of the part of an element that shows: a
// list, sidebar or dialog that scrolls cuts its rows to its own box, and a
// row it hides whole answers an empty box, so a pin is never drawn over what
// stands beside the list. A partly hidden one keeps the part that shows,
// where the person still sees it. The walk stops at a fixed ancestor, which
// no scrolling box above it cuts.
const visibleScript = `(e => {
	const r = e.getBoundingClientRect(), doc = e.ownerDocument;
	let x0 = r.left, y0 = r.top, x1 = r.right, y1 = r.bottom;
	for (let a = e; a && a !== doc.body && a !== doc.documentElement; a = a.parentElement) {
		const s = doc.defaultView.getComputedStyle(a);
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
// the boxes it answers are where the element stops. A selector of
// elementScript's that reaches into frames finds the element inside them.
const locateScript = `((selectors, reveal) => {
	` + Frames + `
	const found = s => { try { return find(s); } catch { return null; } };
	const shown = reveal ? found(reveal) : null;
	if (shown) {
		const r = shown.e.getBoundingClientRect(), v = framed(shown.e, shown.frames);
		const whole = v.w === Math.round(r.width) && v.h === Math.round(r.height) && v.w > 0 &&
			v.y >= 0 && v.x >= 0 && v.y + v.h <= innerHeight && v.x + v.w <= innerWidth;
		if (!whole) shown.e.scrollIntoView({block: r.height > innerHeight ? 'start' : 'center', inline: 'nearest', behavior: 'instant'});
	}
	return selectors.map(s => {
		const f = found(s);
		return f ? framed(f.e, f.frames) : {x: 0, y: 0, w: 0, h: 0};
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
