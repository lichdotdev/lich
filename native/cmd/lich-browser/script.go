package main

// pageScript runs in every frame before the page's own scripts: WebKitGTK
// tells the UI process nothing of a page's console, so the errors it shows and
// the ones nothing caught are posted to the "lich" message handler, worded as
// Chrome's console words them. It also posts each input event the page gets
// to "lichInput", which the wire's input waits on (seen_linux.go).
const pageScript = `(() => {
	const post = text => { try { webkit.messageHandlers.lich.postMessage(String(text)); } catch {} };
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
	const seen = e => { if (e.isTrusted) try { webkit.messageHandlers.lichInput.postMessage(e.type); } catch {} };
	for (const type of ['mousedown', 'mouseup', 'keydown', 'keyup', 'wheel']) {
		addEventListener(type, seen, {capture: true, passive: true});
	}
})();`
