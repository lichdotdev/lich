package cli

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

func (c *client) browser(args []string) error {
	if len(args) == 0 {
		return usageError("browser")
	}
	if args[0] == "-h" || args[0] == "--help" {
		printHelp(c.stdout, newFlagSet("browser"))
		return errHelpShown
	}
	switch args[0] {
	case "open":
		return c.browserOpen(args[1:])
	case "info":
		return c.browserInfo(args[1:])
	case "click":
		return c.browserClick(args[1:])
	case "type":
		return c.browserType(args[1:])
	case "press":
		return c.browserPress(args[1:])
	case "screenshot":
		return c.browserScreenshot(args[1:])
	case "navigate":
		return c.browserNavigate(args[1:])
	case "reload":
		return c.browserID(api.BrowserReload, args[1:])
	case "back":
		return c.browserID(api.BrowserBack, args[1:])
	case "forward":
		return c.browserID(api.BrowserForward, args[1:])
	case "scroll":
		return c.browserScroll(args[1:])
	case "list":
		return c.browserList(args[1:])
	case "close":
		return c.browserID(api.BrowserClose, args[1:])
	default:
		return usageError("browser")
	}
}

func (c *client) browserOpen(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	h, err := callMethod(context.Background(), c, api.BrowserOpen, api.BrowserOpenOptions{
		ID: id, URL: strings.Join(flags.Args(), " "),
	}, openCall)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, h.URL)
	return nil
}

func (c *client) browserInfo(args []string) error {
	id, err := c.browserOwnerArgs(args)
	if err != nil {
		return err
	}
	info, err := callMethod(context.Background(), c, api.BrowserInfo, api.SessionIDOptions{ID: id}, openCall)
	if err != nil {
		return err
	}
	return c.printPage(info)
}

// printPage prints a page the way the browser_* tools answer with it.
func (c *client) printPage(info api.BrowserPageInfo) error {
	_, err := fmt.Fprintln(c.stdout, pageText(info))
	return err
}

func (c *client) browserClick(args []string) error {
	flags := newFlagSet("browser")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	t := tf.target()
	if t.Index == nil && t.Selector == "" && (t.X == nil || t.Y == nil) {
		return fmt.Errorf("click needs --index, --selector, or --x and --y")
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	info, err := callMethod(context.Background(), c, api.BrowserClick, api.BrowserClickOptions{ID: id, BrowserTarget: t}, openCall)
	if err != nil {
		return err
	}
	return c.printPage(info)
}

func (c *client) browserType(args []string) error {
	flags := newFlagSet("browser")
	clear := flags.Bool("clear", false, "replace the field before typing")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	text := strings.Join(flags.Args(), " ")
	if text == "" {
		return usageError("browser")
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	_, err = callMethod(context.Background(), c, api.BrowserType, api.BrowserTypeOptions{
		ID: id, Text: text, Clear: *clear, BrowserTarget: tf.target(),
	}, shortCall)
	return err
}

func (c *client) browserPress(args []string) error {
	flags := newFlagSet("browser")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	key := strings.Join(flags.Args(), " ")
	if key == "" {
		return usageError("browser")
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	info, err := callMethod(context.Background(), c, api.BrowserPress, api.BrowserPressOptions{
		ID: id, Key: key, BrowserTarget: tf.target(),
	}, openCall)
	if err != nil {
		return err
	}
	return c.printPage(info)
}

func (c *client) browserScreenshot(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	dest := ""
	if flags.NArg() > 0 {
		dest = flags.Arg(0)
	}
	path, err := callMethod(context.Background(), c, api.BrowserScreenshot, api.BrowserScreenshotOptions{ID: id, Path: dest}, openCall)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, path)
	return nil
}

func (c *client) browserNavigate(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	raw := strings.Join(flags.Args(), " ")
	if raw == "" {
		return usageError("browser")
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	h, err := callMethod(context.Background(), c, api.BrowserNavigate, api.BrowserOpenOptions{ID: id, URL: raw}, openCall)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, h.URL)
	return nil
}

func (c *client) browserScroll(args []string) error {
	flags := newFlagSet("browser")
	dy := flags.Int("dy", 400, "pixels to scroll (positive is down)")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	id, err := c.browserOwner()
	if err != nil {
		return err
	}
	_, err = callMethod(context.Background(), c, api.BrowserScroll, api.BrowserScrollOptions{ID: id, DY: *dy}, shortCall)
	return err
}

func (c *client) browserList(args []string) error {
	id, err := c.browserOwnerArgs(args)
	if err != nil {
		return err
	}
	list, err := callMethod(context.Background(), c, api.BrowserList, api.SessionIDOptions{ID: id}, shortCall)
	if err != nil {
		return err
	}
	if list == nil {
		list = []api.BrowserHandle{}
	}
	return c.emit(list)
}

func (c *client) browserID(method api.Method[api.SessionIDOptions, api.None], args []string) error {
	id, err := c.browserOwnerArgs(args)
	if err != nil {
		return err
	}
	_, err = callMethod(context.Background(), c, method, api.SessionIDOptions{ID: id}, shortCall)
	return err
}

func (c *client) browserOwnerArgs(args []string) (string, error) {
	if err := c.browserNoArgs(args); err != nil {
		return "", err
	}
	return c.browserOwner()
}

func (c *client) browserNoArgs(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("browser")
	}
	return nil
}

type targetFlags struct {
	index    *int
	selector *string
	x, y     *float64
}

func registerTarget(flags *flag.FlagSet) *targetFlags {
	t := &targetFlags{}
	t.selector = flags.String("selector", "", "CSS selector")
	flags.Func("index", "element index from browser info", func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		t.index = &n
		return nil
	})
	flags.Func("x", "viewport X", func(s string) error {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		t.x = &n
		return nil
	})
	flags.Func("y", "viewport Y", func(s string) error {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		t.y = &n
		return nil
	})
	return t
}

func (t *targetFlags) target() api.BrowserTarget {
	sel := ""
	if t.selector != nil {
		sel = *t.selector
	}
	return api.BrowserTarget{Index: t.index, Selector: sel, X: t.x, Y: t.y}
}
