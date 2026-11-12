package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chromedp/chromedp"
)

// annotateTag names the overlay's host element. A custom tag no page styles,
// its labels in a shadow root the page's CSS does not reach.
const annotateTag = "lich-annotate"

// labelPx is a label's text height in the image's pixels. The image is
// scaled by shotScale, so the CSS size is labelPx divided by it: a fixed CSS
// size came out half as tall on a 2560 px wide pane, and digits under about
// 10 px are where reading them off an image starts to slip.
const labelPx = 12

// annotateLinger removes the overlay from the page itself when the call that
// drew it never could, a dialog opened mid-capture holding the page's
// script: past any capture, short enough that the person's pane loses it.
const annotateLinger = 5 * time.Second

// unannotateWait bounds the removal, which runs also when the call's own
// deadline has passed.
const unannotateWait = 2 * time.Second

var errAnnotating = errors.New("the agent's labels are drawn on the page")

// errTabChanged: another tab came to the front while Still captured, and the
// capture may be of either.
var errTabChanged = errors.New("another tab came to the front during the capture")

// annotateJS draws each listed element's index at its box, from the read
// Info just made, which stamped the indexes a click takes. The host is fixed, takes
// no pointer events and sits above everything, so the page lays out and
// answers the same under it.
const annotateJS = `(function(els, shotMax, labelPx, linger) {
  const scale = Math.min(1, shotMax / Math.max(innerWidth, innerHeight));
  const host = document.createElement('` + annotateTag + `');
  host.style.cssText = 'all:initial;position:fixed;inset:0;pointer-events:none;z-index:2147483647;display:block';
  const root = host.attachShadow({mode: 'closed'});
  const font = labelPx / scale;
  for (const el of els) {
    const l = document.createElement('span');
    l.textContent = String(el.index);
    l.style.cssText = 'position:absolute;box-sizing:border-box;padding:0 ' + (font / 4) + 'px;' +
      'font:bold ' + font + 'px/1.15 monospace;color:#fff;background:#d4145a;border:' + (1 / scale) + 'px solid #fff;' +
      'left:' + Math.max(0, el.rect.x) + 'px;top:' + Math.max(0, el.rect.y) + 'px';
    root.appendChild(l);
  }
  document.documentElement.appendChild(host);
  setTimeout(() => host.remove(), linger);
})`

const unannotateJS = `document.querySelectorAll('` + annotateTag + `').forEach((el) => el.remove())`

// AnnotatedScreenshot draws, captures and removes in one call, so the labels
// show on the person's pane, which the screencast mirrors, only for the
// capture.
func (p *chromePage) AnnotatedScreenshot(ctx context.Context, dest string) (w, h int, err error) {
	info, err := p.Info(ctx, "")
	if err != nil {
		return 0, 0, err
	}
	els, err := json.Marshal(info.Interactive)
	if err != nil {
		return 0, 0, err
	}
	p.annotating.Add(1)
	defer p.annotating.Add(-1)
	defer p.unannotate()
	script := annotateJS + "(" + string(els) + fmt.Sprintf(", %d, %d, %d)", shotMax, labelPx, annotateLinger.Milliseconds())
	if err := p.run(ctx, chromedp.Evaluate(script, nil)); err != nil {
		return 0, 0, err
	}
	return p.Screenshot(ctx, dest)
}

// unannotate is runFree: the page's own timer removes what a held dialog
// keeps this from reaching.
func (p *chromePage) unannotate() {
	ctx, cancel := context.WithTimeout(context.Background(), unannotateWait)
	defer cancel()
	if err := p.runFree(ctx, chromedp.Evaluate(unannotateJS, nil)); err != nil {
		slog.Warn("browser: annotation overlay left for the page to remove", "err", err)
	}
}
