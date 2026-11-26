//go:build linux || windows

package main

// pageScript runs in every frame before the page's own scripts: the engine
// tells lich-browser nothing of a page's console, so the errors it shows and
// the ones nothing caught are posted to the "lich" message handler, worded as
// Chrome's console words them. It also posts each input event the page gets
// to "lichInput", which the wire's input waits on (seen_linux.go). send is
// the engine's way of posting to a handler (postToHandler).
const pageScript = `(() => {
	const send = ` + postToHandler + `;
	const post = text => { try { send('lich', String(text)); } catch {} };
	const word = v => {
		if (v instanceof Error) return v.name + ': ' + v.message;
		if (v !== null && typeof v === 'object') { try { return JSON.stringify(v); } catch { return String(v); } }
		return String(v);
	};
	const error = console.error;
	console.error = function (...args) { post(args.map(word).join(' ')); return error.apply(this, args); };
	addEventListener('error', e => {
		// A script of another origin's error reaches the page as "Script
		// error." with nothing else, the wire's own scripts' included.
		if (!e.filename && e.error === null) return;
		const where = ' (' + e.filename + ':' + e.lineno + ')';
		post('Uncaught ' + (e.error != null ? word(e.error) : e.message) + where);
	});
	addEventListener('unhandledrejection', e => post('Uncaught (in promise) ' + word(e.reason)));
	// Told a task later, once the event's default action ran: a key's
	// editing lands after its keydown, and told at the keydown the wire's next
	// request read the field before it.
	const seen = e => { if (e.isTrusted) setTimeout(() => { try { send('lichInput', e.type); } catch {} }); };
	for (const type of ['mousedown', 'mouseup', 'keydown', 'keyup', 'wheel']) {
		addEventListener(type, seen, {capture: true, passive: true});
	}
})();`
