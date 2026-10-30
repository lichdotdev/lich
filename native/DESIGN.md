# lich: design language

How the native window looks and behaves. The tokens live in `ui/theme.go`, the components in `ui/`, and
`ui-gallery` shows every one of them. When this file and the code disagree, the code is a holdout to migrate,
unless a line here says the code is the exception.

Read it before drawing a new screen or changing an old one. A pattern the maintainer corrected in review
belongs here, in the same PR as the fix, so the next screen starts right.

## The one idea: open surfaces, not boxes

Elements are defined by space, hover and islands, not by enclosure.

- **Rows, not cards.** A list item has no border and no fill of its own. Hover and selection paint it.
- **Panels are islands.** The window's regions (sidebar, stage, dock) are rounded surfaces floating on
  `Canvas`, separated by the `gutter` (6px, `cmd/lich-native/main.go`) instead of a hairline. The top bar
  and the footer sit on the canvas itself, unboxed.
- **Hairlines are seams inside an island.** `th.Separator` splits a panel's own parts (a screen's header
  from its body, two panes). It never wraps a list item, a badge or a button at rest. Inputs are the one
  exception: a field needs an edge to read as editable.
- **Badges are text and glyph**, never a filled pill.
- **State is fill, not chrome.** Selection is a flat `Accent` fill: no side bar, no ring, no shadow.

A change that adds a border, a pill or a nested surface is probably wrong. Reach for spacing or a hover state
first.

## Foundations

### Units

- Every length is `ui.Space(n)`, Tailwind's spacing scale (`Space(2.5)` is `p-2.5`, 10dp). A `unit.Dp`
  literal is for a measure off the scale (a 1px border, a mockup's odd value), named in a constant.
- Text sizes are `ui.TextXS` (12), `ui.TextSM` (14) and `ui.TextBase` (16). A size off the scale is a named
  `unit.Sp` constant beside the screen that uses it (`pullsTitleSize`, `settingsTitle`).
- Radius is `ui.RadiusSM` (4), `ui.RadiusMD` (6), `ui.RadiusLG` (8); dialogs alone use 12. Controls and
  rows are `RadiusMD`; islands are `RadiusLG`.
- A screen's geometry lives in one `var` block at the top of its file, each value named and, when it comes
  from a mockup or a Tailwind class, saying which. Never a bare number inside a layout.

### Color

Never a hex or OKLCH value in a widget: always a `Theme` token. Themes are JSON (`docs/themes.md`).

| Token | Role |
| --- | --- |
| `Canvas` | the ground the islands float on |
| `Background` | full screens inside the stage (Settings, Pulls) and code blocks set inside a raised surface |
| `Sidebar`, `Card`, `Popover` | islands and raised parts: sidebar, dock, a thread or comment box, menus, dialogs |
| `Accent` | hover and selection fill |
| `MutedForeground` | secondary text, icons at rest, paths, meta |
| `Border` | seams and input edges |
| `Primary` | the one high-emphasis button of a view |
| `Foreground` | primary text |

A translucent token goes through `th.Over` before it is painted, and a widget that paints a background
hands its children `th.On(background)`. Gio blends in linear light; `Over` composites the way a browser
does. A tint that must read on a known surface is an opaque `ui.MixOKLab(surface, tone, weight)`.

Interaction fills: rest transparent, hover `Alpha(Accent, 0.5)` to `0.6`, active or selected `Accent`.

### Semantic color

The only chromatic color. It encodes state and never decorates.

| Token | Meaning |
| --- | --- |
| `TonePass` | done, passed, added lines |
| `ToneWait` | waiting on the user, running, outdated |
| `Destructive` | destructive action, failure, removed lines |

Added and removed lines in a code excerpt tint their row with `MixOKLab(Background, tone, 0.12)` and color
their text with the tone itself.

### Typography

- **Onest** for all interface text, **the system monospace** for code, paths, diffs and dense meta.
- Body is `TextSM`, meta and paths `TextXS`, headings named per screen.
- Markdown prose is 15sp: larger than interface body text for sustained reading, while leaving more room for
  long descriptions than `TextBase`.
- Text a person reads goes through `i18n.T`; nothing literal (see the `lich-i18n` skill).
- Text from the outside world (a diff hunk, a PR body, a commit message) is cleaned before it is shaped:
  tabs become four spaces, because the fonts have no glyph for a tab and draw a missing-glyph box.

### Icons

Lucide through `icons.Lucide`, at `Space(3.5)` or `Space(4)`; `Space(3)` inside dense meta. Provider marks
and file-type logos have their own sets in `ui/icons`.

## Controls

### Buttons

`th.Button` with a variant and a size; never a hand-drawn clickable for something that is a button.

| Size | Height | Use |
| --- | --- | --- |
| `SizeSM` / `SizeIconSM` | 32 | **the default for actions on a screen**: headers, thread and comment actions, dialogs |
| `SizeDefault` / `SizeIcon` | 36 | a form's main row, beside a 36px input |
| `SizeXS` / `SizeIconXS` | 24 | inline inside dense rows only (a file row, a binary preview) |
| `SizeLG` | 40 | rare; a lone call to action |

Buttons side by side share one size. A row that mixes 24px and 32px buttons is a bug.

Variants: `VariantDefault` (primary fill) for the one action a view exists for, `VariantSecondary` for the
next most likely, `VariantOutline` for an action that should stand out from ghost ones, `VariantGhost` for
the rest, `VariantDestructive` for what cannot be undone. A disabled button says why in a tooltip.

### What goes where

- **The action a state calls for is a button, in reach.** When something is wrong and lich can hand it to an
  agent (Resolve conflicts, Fix checks), the button sits in the header beside the main actions, not in an
  overflow menu.
- **The `⋯` menu holds the routine and the rare**: Approve, Refresh, Open on GitHub. Never the action the
  current state calls for.
- A menu never opens or ends on a separator, and never shows two in a row.

### Tooltips

`th.Tooltip`, in a few words: what an icon-only control does, or why a control is disabled. Several lines
about the thing under the pointer use the card variant.

## Screens

### The island's edge

Every island keeps its content `Space(2)` (8px) from its rounded edge, on all four sides: the sidebar
(`sidebar.go`, `UniformInset(Space(2))`), the Settings nav (`settings_nav.go`) and the Pulls header strip
(`pullsToolbarPad`). Controls and rows sit flush on that edge, box first: the sidebar's New session button,
the Settings search field, a nav row, the Pulls back button and tree toggle all start at 8px.

Text inside a row is indented by the row's own padding, not by the island: `Space(2.5)` (10px) on a sidebar
card (`sidebar_card.go`) and a Settings nav row (`navRowPadX`), so a row's label lands at 18px from the
island's edge and its hover fill spans the 8px edge to edge.

Rows in a list stand `Space(0.5)` to `Space(1.5)` apart; groups of rows `Space(2)`.

A scrolling list lays its bar over the rows (`material.Overlay`), never beside them: a bar that takes its
own width pushes the rows' right edge in, and their hover lands farther from the right edge than the left.

The geometry is shared; the look is not. Every screen lays out on the sidebar's edge, padding and hover
shape, and keeps what makes it itself: the Pulls list keeps its seams between rows, the Settings pane its
reading column. Matching a screen to the sidebar means its spacing, never its decoration.

### Header and strips

A screen's header and every strip under it (a toolbar, a file list's top bar) sit on the island's 8px edge.
The first control's box, not its glyph, sits on that edge, so an icon button's glyph lands at its own
padding inside it.

Each header line is a flex row: what the screen is about on the left, flexing; its actions on the right,
`Rigid`, separated by `ui.Gap(2)`. When the actions and the title no longer fit on one line, the actions take
their own line above the title rather than squeezing it.

A `th.Separator` closes the header off from the body.

### Body

A reading pane, a column of prose and settings rows under a title, is the one place that leaves the 8px
edge: the Settings pane sits at `settingsPanePadX` / `settingsPanePadY` (40px / 32px), its close button in
the corner at `settingsCloseInset` (14px). Every other screen body follows the sidebar: its content at
18px from the island's edge, the 8px edge plus a row's 10px (`pullsPadX` in Pulls).

### Loading

A skeleton mirrors the screen it stands for: the same insets, line heights and columns, a bar where each
piece of text will land, so nothing moves when the content arrives. A generic block of bars is a bug.

### Boxes inside a screen

A thread, a pending comment or a composer is a box on `Sidebar` with `RadiusMD` and `Space(3)` padding. Its
children draw on `th.On(th.Sidebar)`.

Code quoted inside such a box (a diff hunk) never shares the box's fill: it sits in its own block on
`Background`, `RadiusSM`, `Space(2)` padding, so code and prose read as two things.

Vertical rhythm inside a box: `Space(2)` above a code block, `Space(2.5)` between comments and before the
action row.

## Fixed by decision: do not "fix" these

- **The bundled themes stay achromatic.** A colored palette is a custom theme, not a widget's color.
- **The layout**: top tabs, left sidebar, terminal, right dock, footer.
- **Semantic color and the diff language badges**: they carry meaning.
- **Buttons do not move when pressed**; only their color changes.
