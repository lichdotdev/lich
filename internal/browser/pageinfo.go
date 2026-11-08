package browser

// pageInfoScript is injected into the page the agent is driving. It stamps
// data-lich-idx so a later click/type can find the same node, and it never
// reads a form control's value into the result.
//
// An element keeps its index while it lives, so an agent's index stays good
// after the page changes around it: an element that appears above another
// would otherwise shift every index below it, and agents fell back to CSS
// selectors on "stale indexes after navigating". A new element takes the next
// number of a counter the browser never winds back, base, which Info carries
// from document to document and tab to tab, so a number is never handed to a
// second element: an index of a replaced element, a page navigated away from
// or another tab finds nothing and click/type report the page changed. The
// stamp alone is not trusted, as cloneNode copies it: of, which a page cannot
// copy, says which element a number is. An element scrolled out of view keeps
// its index but leaves the list, and click/type scroll it back; one no longer
// displayed is not clicked. The list is in page order, not number order.
//
// A password input has no innerText, so an innerText || value label chain
// hands the live credential to the agent and into the session transcript.
// Rather than classify which fields are sensitive — type === 'password' is
// defeated by a show-password toggle, and an ordinary text input can still
// hold a recovery phrase — no control value is serialized at all. Agents get
// filled / checked instead, which is enough to confirm that typing landed
// without the content leaving the page.
//
// A textarea is covered by the same rule: its content is its default value, so
// innerText on a textarea is a value read. The single exception is a
// push-button input (submit / button / reset), whose value is the author-written
// caption and the only label it has. A user cannot type into one.
//
// The page's text leaves out each line that is the whole label of an element
// listed with it, before its cut: a quarter of a page answer's text repeated
// the list (measured 2026-11-07). That left the cut at 1000 holding as much
// text of its own as 1200 had.
const pageInfoScript = `((base) => {
  const sel = 'a[href], button, input, textarea, select, [role=button], [role=link], [onclick], [contenteditable=""], [contenteditable="true"]';
  const isVisible = (el) => {
    const r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    if (r.bottom < 0 || r.top > innerHeight || r.right < 0 || r.left > innerWidth) return false;
    const s = getComputedStyle(el);
    return s.visibility !== 'hidden' && s.display !== 'none';
  };
  const CAPTION_TYPES = ['submit', 'button', 'reset'];
  const attr = (el, name) => el.getAttribute(name) || '';
  const isCaptionInput = (tag, type) =>
    tag === 'input' && CAPTION_TYPES.indexOf(type) !== -1;
  const labelFor = (el, tag, type) => {
    if (tag === 'select') {
      const opt = el.options ? el.options[el.selectedIndex] : null;
      const optText = opt ? (opt.textContent || '').trim() : '';
      if (optText) return optText;
    } else if (isCaptionInput(tag, type)) {
      const caption = (el.value || '').trim();
      if (caption) return caption;
    } else if (tag !== 'input' && tag !== 'textarea') {
      const own = (el.innerText || '').trim();
      if (own) return own;
    }
    const labelled = el.labels && el.labels[0];
    const labelText = labelled ? (labelled.innerText || labelled.textContent || '').trim() : '';
    if (labelText) return labelText;
    return (
      attr(el, 'aria-label') ||
      attr(el, 'title') ||
      attr(el, 'placeholder') ||
      attr(el, 'name')
    ).trim();
  };
  if (!window.__lichIdx) Object.defineProperty(window, '__lichIdx', { value: { next: 0, of: new WeakMap() } });
  const idx = window.__lichIdx;
  idx.next = Math.max(idx.next, base);
  document.querySelectorAll('[data-lich-idx]').forEach((el) => {
    if (String(idx.of.get(el)) !== el.getAttribute('data-lich-idx')) el.removeAttribute('data-lich-idx');
  });
  const els = [...document.querySelectorAll(sel)].filter(isVisible);
  const interactive = els.slice(0, 200).map((el) => {
    let i = idx.of.get(el);
    if (i === undefined) {
      i = idx.next++;
      idx.of.set(el, i);
    }
    el.setAttribute('data-lich-idx', String(i));
    const r = el.getBoundingClientRect();
    const tag = el.tagName.toLowerCase();
    const type = (el.getAttribute('type') || '').toLowerCase();
    const entry = {
      index: i,
      tag: tag,
      type: el.getAttribute('type') || undefined,
      role: el.getAttribute('role') || undefined,
      text: labelFor(el, tag, type).slice(0, 120),
      href: el.getAttribute('href') || undefined,
      rect: { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) },
    };
    if (type === 'checkbox' || type === 'radio') {
      entry.checked = el.checked === true;
    } else if (tag === 'textarea' || (tag === 'input' && !isCaptionInput(tag, type))) {
      entry.filled = typeof el.value === 'string' && el.value.length > 0;
    }
    return entry;
  });
  const labels = new Set(interactive.map((entry) => entry.text).filter((text) => text !== ''));
  return {
    url: location.href,
    title: document.title,
    text: (document.body ? document.body.innerText : '').split('\n')
      .filter((line) => !labels.has(line.trim())).join('\n').trim().slice(0, 1000),
    interactive: interactive,
    truncated: els.length > 200,
    next: idx.next,
  };
})`
