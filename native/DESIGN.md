# lich: design language

How the native window looks and behaves. The tokens live in `ui/theme.go`, the components in `ui/`, and
`ui-gallery` shows every one of them. When this file and the code disagree, the code is a holdout to migrate,
unless a line here says the code is the exception.

Read it before drawing a new screen or changing an old one. A pattern the maintainer corrected in review
belongs here, in the same PR as the fix, so the next screen starts right.

## The one idea: open surfaces, not boxes

Elements are defined by space, hover and islands, not by enclosure.

- **Rows, not cards.** A list item is one line with no border and no fill of its own. Hover and selection
  paint it; at rest it paints nothing.
- **Panels are islands.** The window's regions (sidebar, stage, dock) are rounded surfaces floating on
  `Canvas`, separated by the `gutter` (6px, `cmd/lich-native/main.go`) instead of a hairline. The top bar
  and the footer sit on the canvas itself, unboxed.
- **Hairlines are seams inside an island.** `th.Separator` splits a panel's own parts (a screen's header
  from its body, two panes, a folder header from its rule). It never wraps a list item, a badge or a button
  at rest. Inputs are the one exception: a field needs an edge to read as editable.
- **Badges are text and glyph**, never a filled pill: a pull request is its icon and `#N`, a diff is `+a -d`
  in the tones.
- **State is fill, not chrome.** Selection is a flat `Accent` fill: no side bar, no ring, no shadow.
- **What does not fit the line goes to the tooltip.** A row carries what you scan for; the rest (path,
  model, the tool a turn runs, how long) is one hover away, never a second line.

A change that adds a border, a pill, a second line or a nested surface is probably wrong. Reach for spacing,
a hover state or the tooltip first. The folder tray is the one nested surface, and the reason is written
under it.

## Foundations

### Units

- Every length is `ui.Space(n)`, Tailwind's spacing scale (`Space(2.5)` is `p-2.5`, 10dp). A `unit.Dp`
  literal is for a measure off the scale (a 1px border, a ring's radius, a mockup's odd value), named in a
  constant.
- Text sizes are `ui.Text2XS` (10.5), `ui.TextXS` (12), `ui.TextSM` (14) and `ui.TextBase` (16). A size off
  the scale is a named `unit.Sp` constant beside the screen that uses it (`pullsTitleSize`, `settingsTitle`),
  declared once: a second constant of the same value for the same role is a bug.
- Radius is `ui.RadiusSM` (4), `ui.RadiusMD` (6), `ui.RadiusLG` (8); dialogs alone use 12. Controls and
  rows are `RadiusMD`; islands and the folder tray are `RadiusLG`.
- A screen's geometry lives in one `var` block at the top of its file, each value named and, when it comes
  from a mockup or a Tailwind class, saying which. Never a bare number inside a layout.

### Color

Never a hex or OKLCH value in a widget: always a `Theme` token. Themes are JSON (`docs/themes.md`).

Three colors are literals on purpose, each named in a constant that says why: a toast's description is
sonner's own gray (`ui/toast_body.go`), the browser notes' tag and halo sit on the website and keep one
look on every page (`browser_notes_view.go`), and Windows' close button hovers in the system's red
(`caption.go`). A fourth needs the same kind of reason.

| Token | Role |
| --- | --- |
| `Canvas` | the ground the islands float on |
| `Background` | full screens inside the stage (Settings, Pulls) and code blocks set inside a raised surface |
| `Sidebar`, `Card`, `Popover` | islands and raised parts: sidebar, dock, a thread or comment box, menus, dialogs |
| `Accent` | hover and selection fill |
| `MutedForeground` | secondary text, icons at rest, paths, meta, the tree's links and neutral rings |
| `Border` | seams and input edges |
| `Primary` | the one high-emphasis button of a view |
| `Foreground` | primary text |

A translucent token goes through `th.Over` before it is painted, and a widget that paints a background
hands its children `th.On(background)`. Gio blends in linear light; `Over` composites the way a browser
does. A tint that must read on a known surface is an opaque `ui.MixOKLab(surface, tone, weight)`.

`th.Over` of a transparent color is the surface itself, opaque. A row at rest that paints `th.Over(layer)`
covers whatever it sits on, a folder's tray included: paint the translucent layer, or nothing, unless the
fill is meant to hide what is under it.

Interaction fills: rest transparent, hover `Alpha(Accent, 0.6)` (`hoverFill`), active or selected `Accent`.
A card painted a color mixes it into the accent: `tintRest` 0.10 at rest, `tintHover` 0.22, `tintActive`
0.38 (`cardcolor.go`).

### Semantic color

The only chromatic color besides a card's own paint and the history graph's lanes. It encodes state and never
decorates. A lane's color (`th.GraphLanes`, one set per scheme, not a theme token) carries which branch a line is,
which the eye cannot follow through a crossing in one gray: use it for that graph and nothing else.

| Token | Meaning |
| --- | --- |
| `TonePass` | done, passed, added lines |
| `ToneWait` | waiting on the user, running, outdated |
| `Destructive` | destructive action, failure, removed lines |

Added and removed lines in a code excerpt tint their row with `MixOKLab(Background, tone, 0.12)` and color
their text with the tone itself.

### Typography

- **Onest** for all interface text, **the system monospace** for code, paths, diffs, counts and dense meta.
- Body is `TextSM`, meta and paths `TextXS`, a group's uppercase label `Text2XS` semibold; headings
  named per screen.
- Markdown prose is 15sp: larger than interface body text for sustained reading, while leaving more room for
  long descriptions than `TextBase`.
- In Markdown a blockquote is `MutedForeground` text `Space(4)` behind a `Space(1)` bar in `Border`, and a code
  block sits across the body on the inline code's fill, `RadiusSM`, `Space(2)` padding (`ui/markdown`).
- Text a person reads goes through `i18n.T`; nothing literal (see the `lich-i18n` skill).
- Text from the outside world (a diff hunk, a PR body, a commit message) is cleaned before it is shaped:
  tabs become four spaces, because the fonts have no glyph for a tab and draw a missing-glyph box.
- A name too long for its row is cut with an ellipsis. Nothing leaves the row to make room for it: the
  sidebar's minimum width (`sidebarMinRem`) is the lever.

### Icons

Lucide through `icons.Lucide`, at `Space(3.5)` or `Space(4)`; `Space(3)` (12) inside dense meta and rows.
Provider marks and file-type logos have their own sets in `ui/icons`. An icon centered in a box of odd
leftover goes through `ui.CenteredIcon`, at a fractional offset, or it lands half a pixel off.

## Controls

### Buttons

`th.Button` with a variant and a size; never a hand-drawn clickable for something that is a button.

| Size | Height | Use |
| --- | --- | --- |
| `SizeSM` / `SizeIconSM` | 32 | **the default for actions on a screen**: headers, thread and comment actions, dialogs |
| `SizeDefault` / `SizeIcon` | 36 | a form's main row, beside a 36px input |
| `SizeXS` / `SizeIconXS` | 24 | inline inside dense rows only (a file row, a binary preview, a folder header's `⋯`) |
| `SizeLG` | 40 | rare; a lone call to action |

Buttons side by side share one size. A row that mixes 24px and 32px buttons is a bug.

Variants: `VariantDefault` (primary fill) for the one action a view exists for, `VariantSecondary` for the
next most likely, `VariantOutline` for an action that should stand out from ghost ones, `VariantGhost` for
the rest, `VariantDestructive` for what cannot be undone. A disabled button says why in a tooltip.

Buttons do not move when pressed; only their color changes.

### What goes where

- **The action a state calls for is a button, in reach.** When something is wrong and lich can hand it to an
  agent (Resolve conflicts, Fix checks), the button sits in the header beside the main actions, not in an
  overflow menu.
- **The `⋯` menu holds the routine and the rare**: Approve, Refresh, Open on GitHub. Never the action the
  current state calls for.
- **A row's actions are its menu.** A list row has no buttons of its own at rest. Its menu opens two ways,
  the right click and a `⋯` that shows in the row's corner under the pointer, and both hold the same
  entries. Opening something in a group (a session in a folder or a checkout) is the menu's first entry,
  never a `+` on the row.
- A menu never opens or ends on a separator, and never shows two in a row.
- A submenu stands `menuSubGap` (4px) off its parent's edge, level with the row that opens it, and never
  over the parent. At the window's edge it flips to the parent's other side, the same gap away.

### Tooltips

`th.Tooltip`, in a few words: what an icon-only control does, or why a control is disabled. What a row left
off its line (a session's path, model and turn; a checkout's branch in full, path, base and pull request)
is the card variant, `th.TooltipCard`, on the row's right side.

## Screens

### The island's edge

Every island keeps its content `Space(2)` (8px) from its rounded edge, on all four sides: the sidebar
(`sidebar.go`, `UniformInset(Space(2))`), the Settings nav (`settings_nav.go`) and the Pulls header strip
(`pullsToolbarPad`). Controls and rows sit flush on that edge, box first: the sidebar's New session button,
the Settings search field, a nav row, the Pulls back button and tree toggle all start at 8px, and their
hover fill spans the 8px edge to edge.

Every list row is padded `Space(1.5)` (6px, `rowInset`; `navRowPadX` and `pullsRowPadX` are the same), so
the window's lists read on one grid, the sidebar's tree's:

- **The node column** starts at 14px from the island's edge and is 22px wide: a tree row's ring sits in it,
  and so does a list row's leading icon, centred in a box of the ring's width (`inNodeColumn`), its centre at
  25px with the ring's.
- **The label** follows the column `Space(1.5)` after it, at 42px, in a tree row and an icon row alike.
- **A row with no icon** (a Pulls row, a search hit) starts its text in the node column, at 14px.

A screen body follows its rows: the Pulls body sits at 14px (`pullsPadX`), where its rows start.

Rows of a text list stand `Space(0.5)` to `Space(1.5)` apart, groups of rows `Space(2)`; tree rows stand
flush (see the sidebar).

A scrolling list lays its bar over the rows (`material.Overlay`), never beside them: a bar that takes its
own width pushes the rows' right edge in, and their hover lands farther from the right edge than the left.

The geometry is shared; the look is not. Every screen lays out on the island's edge, its rows' padding and
the sidebar's hover shape, and keeps what makes it itself: the Pulls list keeps its seams between rows, the
Settings pane its reading column. Matching a screen to the sidebar means its spacing, never its decoration.

### Header and strips

A screen's header and every strip under it (a toolbar, a file list's top bar) sit on the island's 8px edge.
The first control's box, not its glyph, sits on that edge, so an icon button's glyph lands at its own
padding inside it.

Each header line is a flex row: what the screen is about on the left, flexing; its actions on the right,
`Rigid`, separated by `ui.Gap(2)`. When the actions and the title no longer fit on one line, the actions take
their own line above the title rather than squeezing it.

A `th.Separator` closes the header off from the body.

### Toasts

The window has one toast stack, in the bottom-right corner, the `gutter` over the footer however many rows
the footer wraps to (`layoutToasts`). It grows upward, and a toast comes in and leaves by the bottom edge. A
screen keeps its actions at the right of its header, so nothing the window raises stands in the top-right
corner; a question that waits for an answer goes on the same stack as a passing note, not in a corner of its
own.

### Body

A reading pane, a column of prose and settings rows under a title, is the one place that leaves the 8px
edge: the Settings pane sits at `settingsPanePadX` / `settingsPanePadY` (40px / 32px), its close button in
the corner at `settingsCloseInset` (14px). Every other screen body follows its rows (see the island's edge).

### Loading

A skeleton is the loaded screen with a bar where each piece of text will land, faithful to it in every
measure, so nothing moves when the content arrives:

- the same insets, columns and separators, laid out by the same constants the loaded screen uses;
- a row the loaded row's exact height, its bars centred in the text's line height (`TextSM` in 20px, `TextXS`
  in 16px) and as tall as the font size, never thinner;
- a list as many rows as fill the area, not a handful stacked at the top;
- a fixed column (a row's checks, a facts column) keeps its width and its bar where the value sits.

A test pins the skeleton row's height to the loaded row's. A generic block of bars is a bug, and so is a
skeleton that only looks close: compare it with a shot of the loaded screen at the same size.

### Fixed headings

A heading that stays put while the content under it scrolls (the Overview's Description) sits on a
`th.Separator` across its column, so the edge the content scrolls under reads as one.

### Boxes inside a screen

A thread, a pending comment or a composer is a box on `Sidebar` with `RadiusMD` and `Space(3)` padding. Its
children draw on `th.On(th.Sidebar)`.

Code quoted inside such a box (a diff hunk) never shares the box's fill: it sits in its own block on
`Background`, `RadiusSM`, `Space(2)` padding, so code and prose read as two things.

Vertical rhythm inside a box: `Space(2)` above a code block, `Space(2.5)` between comments and before the
action row. Comments one after another inside a box are split by a `th.Separator`, `Space(2.5)` on each side.

A comment, in a box or on the screen's ground, sits beside its author's avatar: a `Space(6)` disc in a gutter
`Space(2.5)` from the comment, its byline and body in the column to the right. Until the picture arrives, and
for an account with none, the disc is `Accent` with the initial in `MutedForeground`, never a color.

## The sidebar: a tree of rows

The sidebar is the window's reference list; its geometry lives in `sidebar_tree_draw.go` and
`sidebar_card.go`.

### A row

Every row is one line, 34dp tall: `Space(1.5)` above and below a 22px node. Rows stand flush, no gap
(`cardGap` is 0), so the tree's links run from row to row unbroken.

- **A session row**: its status ring around the provider mark, the shield and fork glyphs it
  earns, its name, then its pull request and changes unless a checkout row above already shows them, and
  the corner slot.
- **A browser row**: a session's running browser, first under it: the globe in a neutral ring, the page's
  host as its name, the corner slot. The name is `MutedForeground` while the page is off the stage. A folded
  mother shows a globe on her own row instead.
- **A session in a layout** carries the columns glyph before its corner slot in its project's sidebar. The
  layout's own sidebar draws none: every card there is in it.
- **A layout's sidebar**: the layout's name where New Session stands, then its sessions under each
  project's name in `TextXS` `MutedForeground`, in the same tree of checkout, session and browser rows, with their motion. No
  card there wears the active fill: the focused pane's ring says which session has the keys.
- **A pane's motion**: a pane new to the stage grows out of the edge it was split on in 200ms ease-out, and
  one that leaves shrinks back into it in 180ms ease-in, the neighbours giving and taking the room over the
  same time. A zoom, a divider and a change of tab move at once.
- **A checkout row**: the branch glyph in a neutral ring, the branch, its pull request and changes, the
  corner slot. A project's checkout is the root its sessions hang from; a child running in another
  checkout than its mother hangs from a checkout row of its own inside her group.
- **A group header** (a folder, the pinned block) is a row too, the same 34dp, so its hover lights as a
  row's does: the glyph in the node's column, the uppercase label, its count, a rule to the `⋯`.

The corner slot (`cardSlotSize`, 18px) is always reserved, so nothing on the line moves when it fills: the
`⋯` under the pointer or while its menu is open, a pin at rest on a pinned session, empty otherwise.

### The node

- The ring is 22px (`2 × sessionTreeRingR`), stroke `sessionTreeStroke` (1.25), the glyph inside it 13px
  (12 for a checkout's branch). It spins while busy, is `ToneWait` while waiting on you, `TonePass` when a
  turn is done (faded once read), and neutral (`MutedForeground` at `neutralRingAlpha`, 0.35) otherwise.
- A node that folds (a mother, a checkout, a group header) shows the fold's chevron in place of its glyph
  under the pointer; there is no separate disclosure column. A folded node keeps a count and the mark of
  what it hides: `ToneWait` for a session waiting, `TonePass` for an unread turn.

### The links

Git-graph lines in `MutedForeground` at `treeLinkAlpha` (0.45), `sessionTreeStroke` wide. Each level is
indented `sessionTreeIndent` (16px); a child's elbow leaves its parent's ring column and curves
(`sessionTreeElbow`, 5px) into the left of its ring. A mother with open children draws a stub from the
bottom of her ring to the row's end.

### The folder tray

A folder is the one nested surface: its header and rows sit on a tray, `RadiusLG`, so where a folder ends
reads against the checkouts after it, which share its tree. The tray is `Alpha(Accent, 0.45)`, or the
folder's color at `tintRest`; a card painted the folder's color paints nothing at rest on it. Rows run to
the tray's edges, clipped to its corners, so a lit row fills it to the end. A closed folder is its header
alone, no tray.

### The rail

The collapsed sidebar is an island like the open one (`sidebar_rail.go`): `Sidebar`, `RadiusLG`, its content
on the 8px edge around a column as wide as a row is tall (34), so it is 50px wide and every node centres
where it does in the open sidebar. Expand sits in a row of the open header's height
(36), where collapse sits in the open sidebar; new session, Services and the profile sit in its footer, so
the tree starts where the open sidebar's does; blocks stand `Space(1.5)` apart as the open ones do, and every
node keeps its place across the morph. The tree is
the open one in one column: rows of the same 34dp, each block starting with its node (a checkout's ring, or
a folder's or the pinned block's glyph, named in a tooltip), the line down the column through every ring of
a tree, a child's ring 16px on its mother's line, and a folder on its tray. A folded block or mother keeps
only its node. A session's ring is lit by a 32px square, `Accent` while active. A card drags in the rail as
in the open sidebar, and the filter is the open one's field and chips in a panel off the search button in
the footer; it outlives the panel, and the button stays lit with a dot while it narrows the tree.

## Motion

Motion says where something came from and where it went. It never holds anything up: a session's pane opens
at once, whatever its row is still doing.

- **Rows of a tree move by their line.** Coming in (a mother or a block opens, a session or a worktree appears), the
  row opens with its name already on it, then the line runs down to it, draws its ring from the left where
  the link arrives, and the glyph comes in last. Leaving (a mother or a block folds, a session closes) plays the other
  way: the name and glyph go, the ring undraws, the line runs back up, the row closes. A worktree's last
  session leaving takes the worktree's row out after it; a folder's last session takes its checkout row,
  then the folder's header fades and its tray closes; a new folder plays that backwards. The rail moves the
  same way, in the same timings, its rings and line without the names. Timings live in `sidebar_tree_motion.go`.
  A folded folder keeps its tray until its rows have left.
- **A ring redraws when its session's status changes.** The ring of the status it left fades while the new
  one draws from the left, as a ring coming into the tree does, in `statusTurnDuration` (400ms). Busy and
  compacting are one ring, so nothing plays between them, and only a live report plays: a status read at
  startup is drawn at rest.
- **A dragged card lets go of its line and joins where it lands.** As it lifts, its elbow runs back to its
  parent's line; while it is in the air, the rows are linked in the order the list shows them, and its slot
  draws a dashed elbow and ring; let go, it settles and the line draws back to its ring
  (`sidebar_drag_links.go`). A mother in the air folds over her children, and the slot only stops where
  the tree will keep the card (`treeSnap`). A block the card would go to lights its whole tray, a border in its color, in
  the open sidebar and the rail alike.
- **Other lists** enter and leave with `ui.Presence`: 160ms in, 180ms out, from a 0.6 scale at the top
  edge. A fold is `ui.Collapse`, 200ms.
- **One motion per arrival.** An item another motion brings in is settled in `Presence`
  (`Presence.Settle`), or Presence plays its own entry once the first one hands the item back, and the row
  jumps.
- **A moving item takes no input**, but advances its tween before `gtx.Disabled()`: a disabled context drops
  the redraw request, and the motion freezes on its first frame.
- **A click shows in the frame that took it.** Gio draws one frame per event, so a click read after the
  thing it changes was laid out leaves the old state on screen until the mouse moves. Read the click before
  that layout, as `actGit` does for the Git panel, whose row comes before the footer button that opens it.
  A control that opens, closes or resizes something laid out elsewhere gets a `frameRig.click` test
  (`stale_*_test.go`), which fails on exactly this.

## Fixed by decision: do not "fix" these

- **The bundled themes stay achromatic.** A colored palette is a custom theme, not a widget's color.
- **The layout**: top tabs, left sidebar, terminal, right dock, footer, and the Git panel rising from the footer
  across the terminal and the dock. It takes height, never width: fewer terminal rows repaint, while fewer columns
  would rewrap what the agent printed.
- **Pinned project tabs** stay at the left of the top strip, grouped before unpinned tabs with one hairline
  between the groups; they carry a pin glyph and close from their context menu only.
- **Semantic color and the diff language badges**: they carry meaning.
- **Buttons do not move when pressed**; only their color changes.
- **A sidebar row is one line.** Pull request and changes stay on it; a name too long is cut, not wrapped.
