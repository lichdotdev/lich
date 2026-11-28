package browser

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// solidPNG is a side-pixel square of c.
func solidPNG(t *testing.T, side int, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

var (
	red  = color.NRGBA{R: 255, A: 255}
	blue = color.NRGBA{B: 255, A: 255}
)

// iconService is a browser beside pages that declare a red PNG, a blue one,
// lich's own SVG app icon, a broken one, or none.
func iconService(t *testing.T) (s *Service, base string) {
	t.Helper()
	svg, err := os.ReadFile("../../docs/media/appicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{
		"/red":    `<title>red</title><link rel="icon" href="/red.png">`,
		"/blue":   `<title>blue</title><link rel="shortcut icon" href="/blue.png">`,
		"/svg":    `<title>svg</title><link rel="icon" href="media/appicon.svg">`,
		"/broken": `<title>broken</title><link rel="icon" href="/broken.png">`,
		"/none":   `<title>none</title>`,
	}
	files := map[string][]byte{"/red.png": solidPNG(t, 64, red), "/blue.png": solidPNG(t, 16, blue), "/broken.png": []byte("not a png"), "/media/appicon.svg": svg}
	s = settleService(t, `<title>page</title>`, func(w http.ResponseWriter, r *http.Request) {
		if body, ok := pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(body))
			return
		}
		if data, ok := files[r.URL.Path]; ok {
			if r.URL.Path == "/media/appicon.svg" {
				w.Header().Set("Content-Type", "image/svg+xml")
			} else {
				w.Header().Set("Content-Type", "image/png")
			}
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	})
	tabs, err := s.Tabs("s")
	if err != nil {
		t.Fatal(err)
	}
	return s, tabs[0].URL
}

// frontIcon is the icon the pane's front tab shows, as the window reads it.
func frontIcon(t *testing.T, s *Service) string {
	t.Helper()
	sess, err := s.live("s")
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return frontOf(sess.paneTabs).Icon
}

// waitIcon waits for the front tab's icon to satisfy ok, and returns it.
func waitIcon(t *testing.T, s *Service, ok func(string) bool) string {
	t.Helper()
	var key string
	for deadline := time.Now().Add(pageWait); time.Now().Before(deadline); time.Sleep(downloadPoll) {
		if key = frontIcon(t, s); ok(key) {
			return key
		}
	}
	t.Fatalf("the front tab's icon never came: %q", key)
	return ""
}

// iconAt is the icon key names, decoded.
func iconAt(t *testing.T, s *Service, key string) image.Image {
	t.Helper()
	data, err := s.Icon(t.Context(), api.BrowserIconOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() > iconSide || b.Dy() > iconSide {
		t.Fatalf("the icon is %v, over %dpx", b, iconSide)
	}
	return img
}

// near: the canvas round-trips a color through its own space, a step off.
func near(a, b color.NRGBA) bool {
	d := func(x, y uint8) bool { return max(x, y)-min(x, y) <= 2 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

func centre(img image.Image) color.NRGBA {
	b := img.Bounds()
	return color.NRGBAModel.Convert(img.At(b.Dx()/2, b.Dy()/2)).(color.NRGBA)
}

func TestATabShowsTheIconItsPageDeclares(t *testing.T) {
	s, base := iconService(t)
	goTo(t, s, base+"red")
	redKey := waitIcon(t, s, func(k string) bool { return k != "" })
	if c := centre(iconAt(t, s, redKey)); !near(c, red) {
		t.Fatalf("the red page's icon is %v at its centre", c)
	}
	goTo(t, s, base+"blue")
	blueKey := waitIcon(t, s, func(k string) bool { return k != "" && k != redKey })
	if c := centre(iconAt(t, s, blueKey)); !near(c, blue) {
		t.Fatalf("the blue page's icon is %v at its centre", c)
	}
}

// lich's own site declares its app icon as an SVG that colors itself by
// prefers-color-scheme: black for a light window, white for a dark one.
func TestATabShowsAnSVGIconInTheWindowsScheme(t *testing.T) {
	t.Cleanup(func() { iconsDark.Store(false) })
	s, base := iconService(t)
	for _, dark := range []bool{false, true} {
		iconsDark.Store(dark)
		goTo(t, s, base+"none")
		waitIcon(t, s, func(k string) bool { return k == "" })
		goTo(t, s, base+"svg")
		img := iconAt(t, s, waitIcon(t, s, func(k string) bool { return k != "" }))
		light, darkInk := 0, 0
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); c.A > 128 && c.R > 128 {
					light++
				} else if c.A > 128 {
					darkInk++
				}
			}
		}
		if ink := max(light, darkInk); ink < iconSide*iconSide/8 || (light > darkInk) != dark {
			t.Fatalf("dark window %v: the SVG icon drew %d light and %d dark pixels", dark, light, darkInk)
		}
	}
}

// A page with no icon, or one that does not load, shows none: not the icon of
// the page before it.
func TestATabShowsNoIconWhereThePageHasNone(t *testing.T) {
	s, base := iconService(t)
	for _, page := range []string{"none", "broken"} {
		goTo(t, s, base+"red")
		waitIcon(t, s, func(k string) bool { return k != "" })
		goTo(t, s, base+page)
		waitIcon(t, s, func(k string) bool { return k == "" })
	}
}

// goTo loads url, again when Chrome aborted the load: a navigation started
// right after a load ended is now and then aborted and the page before
// reloaded (net::ERR_ABORTED, 1 in 40, with or without icons).
func goTo(t *testing.T, s *Service, url string) {
	t.Helper()
	var err error
	for range 3 {
		if _, err = s.Navigate("s", url); err == nil {
			return
		}
	}
	t.Fatalf("load %s: %v", url, err)
}
