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
- **Panels are islands.** The window's regions (sidebar, rail, stage, dock, the Git and extension panels,
  Settings' two columns, Pulls, and the sidebar's and dock's peeks) are rounded surfaces floating on `Canvas`, separated by the `gutter`
  (6px, `cmd/lich-native/main.go`) instead of a rule between them, the stage and the dock running down to the
  window's foot as the sidebar does. The sidebar and the dock are resized from the gutter between them and the
  stage, all of its width (`ui.ResizeHandle.LayoutGap`), which shows only the resize cursor and paints nothing. Each carries a 1px hairline in `Border`
  just inside its `RadiusLG` edge (`ui.InsetBorder`, `ui.IslandBorder`): the one outline in the window that
  wraps a surface rather than a field. A layout has no stage island: each of its panes is an island of its
  own on `Canvas`, a gutter apart, running to the stage's edges (`layoutPanes`). The top bar sits on the
  canvas itself, unboxed: a project tab is a pill a gutter clear of the islands, `Accent` when active, the
  foreground at 4% on hover, taking hover and clicks on the pill alone, with one `Border` hairline where the
  pinned tabs end and none between the others. The footer is two balloons over the stage (see below).
- **Hairlines are seams inside an island, and its edge.** `th.Separator` splits a panel's own parts (a screen's header
  from its body, two panes, a folder header from its rule). It never wraps a list item, a badge or a button
  at rest. Inputs are the one exception: a field needs an edge to read as editable.
- **Badges are text and glyph**, never a filled pill: a pull request is its forge's logo, its state's glyph and
  `#N` (`!N` on GitLab), a diff is `+a -d` in the tones.
- **State is fill, not chrome.** Selection is a flat `Accent` fill: no side bar, no ring, no shadow.
- **What does not fit the line goes to the tooltip.** A row carries what you scan for; the rest (path,
  model, the tool a turn runs, how long) is one hover away, never a second line.

Inside an island, a change that adds a border, a pill, a second line or a nested surface is probably wrong. Reach for spacing,
a hover state or the tooltip first. The folder tray is the one nested surface, and the reason is written
under it; the dock's tab pill reuses its fill, for the same reason. The footer's balloons are the pills on `Sidebar`:
they float over the terminal and need a ground of their own to read on it. The extension and theme
catalogs, their Installed tabs included, are the screens of cards, and the reason is written under them.

## Foundations

### Units

- Every length is `ui.Space(n)`, Tailwind's spacing scale (`Space(2.5)` is `p-2.5`, 10dp). A `unit.Dp`
  literal is for a measure off the scale (a 1px border, a ring's radius, a mockup's odd value), named in a
  constant.
- Text sizes are `ui.Text2XS` (10.5), `ui.TextXS` (12), `ui.TextSM` (14) and `ui.TextBase` (16). A size off
  the scale is a named `unit.Sp` constant beside the screen that uses it (`pullsTitleSize`, `settingsTitle`),
  declared once: a second constant of the same value for the same role is a bug.
- Radius is `ui.RadiusSM` (4), `ui.RadiusMD` (6), `ui.RadiusLG` (8); `ui.RadiusXL` (12) is the dialogs' and the person's message's alone. Controls and
  rows are `RadiusMD`; islands and the folder tray are `RadiusLG`.
- A screen's geometry lives in one `var` block at the top of its file, each value named and, when it comes
  from a mockup or a Tailwind class, saying which. Never a bare number inside a layout.

### Color

Never a hex or OKLCH value in a widget: always a `Theme` token. Themes are JSON (`docs/themes.md`).

Three colors are literals on purpose, each named in a constant that says why: a toast's description is
sonner's own gray (`ui/toast_body.go`), the browser notes' tag and halo sit on the website and keep one
look on every page (`browser_notes_view.go`), and Windows' close button hovers in the system's red
(`caption.go`). A page's icon on its browser tab is the site's own picture, in the site's colors
(`browser_icon.go`). A fifth needs the same kind of reason.

| Token | Role |
| --- | --- |
| `Canvas` | the ground the islands float on |
| `Background` | full screens inside the stage (Settings, Pulls) and code blocks set inside a raised surface |
| `Sidebar`, `Card`, `Popover` | islands and raised parts: sidebar, dock, a thread or comment box, menus, dialogs |
| `Accent` | hover and selection fill |
| `MutedForeground` | secondary text, icons at rest, paths, meta, the tree's links and neutral rings |
| `Border` | seams, input edges and the island hairline |
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
which the eye cannot follow through a crossing in one gray: use it for that graph and nothing else. `ToneMerged`
is one color per scheme too, not a theme token.

| Token | Meaning |
| --- | --- |
| `TonePass` | done, passed, added lines |
| `ToneWait` | waiting on the user, running, outdated |
| `Destructive` | destructive action, failure, removed lines |
| `ToneMerged` | a merged pull request |

Added and removed lines in a code excerpt tint their row with `MixOKLab(Background, tone, 0.12)` and color
their text with the tone itself.

### Typography

- **Onest** for all interface text, **the system monospace** for code, paths, diffs, counts and dense meta.
- Body is `TextSM`, meta and paths `TextXS`, a group's uppercase label `Text2XS` semibold; headings
  named per screen.
- Markdown prose is 15sp: larger than interface body text for sustained reading, while leaving more room for
  long descriptions than `TextBase`. A line of it stands its size times 1.5 under the last (22.5 for 15sp, the
  composer's leading), a person's message at `TextSM` 21, a heading 1.25 and a code block 1.3. The leading is set
  from the cell's largest span size (`LineHeight`), never left to styledtext, which scales the glyph's ascent
  plus descent instead and put lines about 2.25 times the size apart, the extra all under each line, so a
  message's bubble sat lower in its box than it should.
- In Markdown a blockquote is `MutedForeground` text `Space(4)` behind a `Space(1)` bar in `Border`, and a code
  block sits across the body on the inline code's fill, `RadiusSM`, `Space(2)` padding (`ui/markdown`).
  A block whose fence names a language the code views know is highlighted with their palette (`ui.Theme.Code`),
  so it reads as the editor does; one with no language, or one they leave plain, stays in prose's code color.
  Its copy button is a ghost `SizeIconXS` one (Lucide `copy`, a check for 1.5s after a click) shown only while the
  pointer is over the block, as a message's copy is, with no tooltip; it stands on the first line inside the
  right padding, and the code wraps short of it, so it never covers text and nothing moves when it shows.
- Markdown text is selectable inside one cell (a paragraph, a heading, a list item's text, a table cell, a code
  block): a drag past 3dp selects by character, a double click a word, a triple click the cell, and the text
  cursor shows over it. The selection fills each rune's glyph box, not the line's leading, in the code views'
  `Theme.Code.Selection` composited on the surface, under the glyphs. A plain click still opens a link, a
  Ctrl+click a path; a press that becomes a drag selects instead. Ctrl+C copies what is read (no room around
  inline code or before a mark), Escape or a press anywhere else lets go, and the composer keeps the keyboard.
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
the rest, `VariantDestructive` for what cannot be undone. A disabled button says why in a tooltip. A button
whose action is still running is `Busy`: the kit's spinner in its icon's place, no click, and its neighbours
disabled until it ends (the /plugin detail's Update).

Buttons do not move when pressed; only their color changes.

Anything that answers a click shows the hand (`ui.Clickable`, `ui.Bool` and `ui.Enum` do it by themselves, a hand-drawn click area calls `ui.HandCursor`); editable text shows the text cursor; a disabled control keeps the arrow.

In the browser's drawing toolbar, the shape picker uses the outline variant so
the current tool reads as a button that can switch tools.

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
- **A popup never covers its anchor** (`popupPlace.box`, `popupPlace.limit`, base-ui's flip with its
  `--available-height`). A menu, a dropdown panel or a tooltip takes its preferred side when it fits whole,
  the opposite when that fits, else whichever has the more room; one that fits on neither is cut to that
  room along the side's axis (its height above or below, its width beside), never shifted over the anchor,
  and its content scrolls inside: a menu's rows with the wheel and the keyboard's highlight, a panel's content
  in the scroll area the kit gives it. A panel's content is laid out with the height it may take as its
  maximum (`DropdownPanelStyle.MaxHeight` caps it lower), so a panel with a fixed part, as the model panel's
  search and effort, keeps it and scrolls the rest itself. `Above()` makes a panel prefer the room over its
  trigger.

### Pickers, steppers and add rows

- **A select** is `th.SelectTrigger`: a 36px field with its edge, the value, an optional lead (a provider's mark,
  a profile's avatar) and the chevron, opening on `Popover` with the current choice checked. A new select uses
  it rather than drawing its own; the theme's trigger, with its miniature, is the one still drawn apart.
- **A stepper** is minus, the reading and plus as ghost buttons in one bordered track, the segmented control's,
  so it stands as tall as one beside it (`settings_stepper.go`). The reading is the reset to the default; there
  is no Reset button.
- **An add row** (New profile, Import theme, Add service) is a ghost `SizeSM` button with a plus, at the list's
  left under its last row, never a dashed card or an outline button across the column.

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
Settings pane its reading column. Seams change the hover: a Pulls row and a `PanelRow` light square across the
whole width, like the seams, since a rounded fill set in between them shows its corners against the lines. Matching a screen to the sidebar means its spacing, never its decoration.

### The backdrop

A session's island wears a field of dots behind its content (`backdrop.go`): long bands that flow from an
ember in the island's bottom-right corner toward the far corner, dithered by Bayer 8x8 on a lattice of 3dp
cells with 2dp dots. The dots are the theme's `Primary`, going toward white at the field's brightest, so each
theme's field wears the color of its own buttons. The ember sits in the corner so the bands run along the
edges and leave the middle, where the text is, calm.

- **Where.** The stage island of a session, each pane of a layout, Home and the empty stage, and the stage
  editor. The dock, Settings, Pulls, a service's console and a browser page have none. A terminal paints no
  ground of its own (`Snapshot.Bare`): the island is filled with the terminal's background and the dots show
  between its cells.
- **How much.** A session's field covers its island from the top to the middle, fading down (`backdropFade`),
  at `backdropOpacity` (0.3); the editor's holds still at the same opacity. A chat with nothing in it shows
  the whole field at full strength, its middle calm (`centreFloor` of it around the start screen's block) and
  lulled to `lineFloor` round the line under the composer; it folds back to the band as the start screen
  fades.
- **When it moves.** A session's field moves while its agent works (a chat's own turn; a terminal's status that
  spins the card) or its user types, and holds still `backdropAwake` (10s) after; a chat's start screen moves
  it for as long as it shows. A still field asks for no frame, and starting again it goes on from where it
  stopped.
- **The start screen's mark.** lich's logo, still, at half size, drawn in the field's own dots in the
  `Primary`'s tones: the bands lift its dots and drop some where they thin, so the mark keeps its shape and
  moves with the field. The question over the composer is 17sp semibold, 36dp above it.
- **What sits on it.** Text on the field reads in the foreground, not muted: a session's header names the
  provider in the name's ink and the name in semibold, and the line under the start screen's composer (the
  project, the branch and its line counts, no separator) is the foreground at 14sp. A control over it takes a
  ground of its own (the GUI | TUI selector). The composer's edge leans to `Primary`, its fill stays near the
  theme's ground.

### Header and strips

A screen's header and every strip under it (a toolbar, a file list's top bar) sit on the island's 8px edge.
The first control's box, not its glyph, sits on that edge, so an icon button's glyph lands at its own
padding inside it.

Each header line is a flex row: what the screen is about on the left, flexing; its actions on the right,
`Rigid`, separated by `ui.Gap(2)`. When the actions and the title no longer fit on one line, the actions take
their own line above the title rather than squeezing it.

A `th.Separator` closes the header off from the body.

### The dock's tabs

The dock's tabs (Code, Review, Artifacts and an extension's surfaces) are a pill of icon-only `BarIconButton`s
floating at its foot, centred, with the tab's name in a tooltip. The pill is `RadiusLG` on a faint `Accent` tray
(`folderTrayAlpha`; a `Sidebar` pill would vanish on the dock, which is `Sidebar` itself), `Space(1)` of padding, `Space(2)` off the dock's edges; the tab on screen is the
`Accent` fill, the others light on hover. The body stops above the strip, so nothing is covered. The header keeps
only actions at its right (Review's, collapse, full screen). A tab that holds something (Review with
changes, Artifacts with files) wears a 5px dot on its icon, never a count or a pill: `Foreground`, or `ToneWait`
on Review over a checkout with conflicts. The dock is never narrower
than the pill.

### The Code tab's tree

A changed file's name takes its git state's color, and git's own letter sits after its `+added -deleted`, in a
`Space(2)` column so the counts line up (`ui/filetree`):

| State | Token | Letter |
|---|---|---|
| modified | `ToneWait` | M |
| untracked | `TonePass` | U |
| added | `TonePass` | A |
| renamed | `TonePass` | R |
| conflict | `Destructive` | ! |

A folder takes the color of the most urgent file inside it (conflict, then modified, then the rest) and a
`Space(1.5)` dot in the letter's place. A deleted file and an ignored one get nothing: the tree lists only what is
on disk and not ignored. The state comes from the git status store (`DiffStats.Changes`), never from a read of the
tree's own.

The tab's search is out of sight at rest: a magnifier `IconAction` in the header's actions, before collapse and
full screen and only on this tab, wears the `Accent` fill while the bar is open. A click on it, or the search
hotkey, unfolds the bar (`ui.Collapse`, 200ms) over the tree and gives its field the keyboard. The bar is one
field under a `th.Separator`, with no mode switch: its placeholder says it reads names and content. Esc empties the
field and, once it is empty, folds the bar; a click on the magnifier folds it and clears the field, so no filter
stays on without showing.

With nothing typed the tree shows. A query replaces it with one list under two headings in `TextXS` caps and
`MutedForeground`, each with its count in mono at the right: **Files**, the paths whose names match, as soon as
typed and the first ten only, counted "10/52" once there are more, with the query marked in the name; and
**In content**, the lines holding it, grouped under their file, once typing settles, counted "500+" when the
answer was cut. Keeping Files to ten keeps In content on screen; typing more narrows them. Down in the field and
the arrows walk the files and then the lines, and Enter or a click opens the file, a line on its number.

### The footer's balloons

The footer is no strip: the two sides the footer editor arranges float as two balloons over the foot of the
stage, the left side in its bottom-left corner and the right side in its bottom-right (`footer_balloons.go`).
A balloon is a pill on `Sidebar` at `RadiusLG`, `Space(0.5)` around the footer's 32px items so their
`RadiusMD` hover sits concentric in it, and it stands on the island's 8px edge from the sides and the foot.
Each is one line: the right side is measured first, the left takes what is left, and an item that does not fit
is left out with the ones after it, never wrapped onto a second line. A side with nothing to show draws no pill.
A balloon is not an island and carries no `Border` hairline: the `Sidebar` ground already sets it off the terminal.

The balloons never cover the agent's prompt. A terminal under them gives up a band at its foot
(`balloonBand`, the balloon and the 8px edge above and below it), so the PTY has fewer rows and the
balloons stand in the room it left; the grid only loses whole rows. The band follows the session's kind, not the
agent live in its PTY, so the grid is resized once, not each time an agent starts or ends. In a layout the
balloons stand on the stage's foot, flush with its sides and its foot, under the panes, reading the focused
pane's session. Every pane stops a gutter over them (`layoutBalloonBand`, the balloon and a gutter), and the
band stays where it is however the focus moves, so no pane's grid is resized.

A plain terminal (a `shell` session), a service's console and Home carry no balloons and no band: the terminal
takes the island's full height. A session drawn as a chat carries none either: its footer stands in the
composer (see The chat view), and the chat takes the band back. A layout keeps the
balloons and the band while it holds an agent's terminal, whichever pane has the focus, a chat's included, so
moving the focus resizes no pane; a layout of chats carries none. Settings, Pulls and an
extension's screen cover the stage and carry none either. A project with no session keeps them, since its branch is the click into the Git panel.

The footer's items are the readings of the session under it and the ways into the screens it opens: by
default attach and the changes on the left, the branch and the context on the right; the model, the cost and
the hands-on time wait in the editor's pool. A fact another place already shows is no item: the pull request
is the card's chip, the project the tab, the plan's usage the sidebar's account row.

The Git panel and an extension's panel rise from the stage's foot over the band: while one is up the balloons
are not drawn and the terminal takes the band back, in the one resize the panel's slide already makes.

### The context tooltip

The context ring's tooltip is one component for the balloon and the chat's composer (`footer_context.go`): a
card with "Context" at the left and the share used at the right in medium, a bar of twenty rounded segments
(`SegmentedBar`; `WaveBar` is its form for work with no share to tell) whose lit ones are `Foreground` up to 70%, `ToneWait` up to 90% and `Destructive` past it, then
lines of a muted label at the left and its value at the right: Tokens (compact, "585.7K", the window muted after
it), Model window, Cache with its hit rate and Session cost, each only when the report has it. Numbers are
compact with the locale's decimal mark. There is no line for compactions or for what is left before one: lich
keeps no count of the first and no provider tells it the threshold of the second.

### The usage menu

The account row at the sidebar's foot is where the plan's usage lives; the footer has no plan item. It follows the
active card's provider (the default one for a terminal or no card) and shows while that provider has logins to
switch between or a plan reading for the account its new sessions spend, so one login, and a provider with no
profiles at all, keep it (`sidebar_account.go`). The row is the provider's glyph, the profile's avatar and name (the
provider's name with no profiles), the five-hour readout in mono at the right ("5h 28%", "wk Locked" when a locked
window outranks it) in the readout's tone, and the chevrons. It opens a panel on a click (`DropdownPanel`, rising
above the row, scrolling when the window is short); hovering it draws nothing. The top of the panel is the usage of
the shown profile's account, in blocks (`usage_tip.go`) built from the context tooltip's pieces
(`tipHead`, `tipRow`, `contextBarColor`, the bar of twenty segments). Under the plan's name ("Claude Code · Max 5x"),
a block per window, in the order session, weekly, then a model's own cap: the window's name (lich's own for a window
it names, with its length: "Session · 5h") at the left and the share spent at the right in medium, the bar in the
same tones as the context's (70%, 90%), then muted `Resets in 4h 12m` and, only when the pace so far empties a
weekly window before it resets, `Runs out in 1d 9h` in `ToneWait`. A locked window reads "Locked" with its bar and
the provider's reason in `Destructive`. After the windows, `Extra usage` with the amount against the cap at the right
("$12.50 of $50.00", "$141.57 · no limit") and a bar only when there is a cap, then a `Reset credits` line like a
context row ("1 available") for Codex, each only when the account has them. Amounts follow the locale (`money.go`):
its separators, the symbol before or after, the sign outside the symbol, the ISO code where there is no symbol. The
account's email sits under the plan's name. The chosen login carries no usage note in the list below: its blocks are above.

Claude Code's limit resets are a `Resets` block of their own: its title with "N left" (every reset of every live
grant) at the right, and a line per grant. The line is its name at the left ("Full" when it clears the weekly window,
the model's name for a model's own cap, the provider's text otherwise), "×2" and a muted "until Dec 10" (the month in
the interface language) at the right, and a `Use` button, a ghost `SizeXS` one (no outline or fill at rest), when the grant can be spent now. A
grant that cannot (paused, or not usable) is muted whole and has no button; with no live grant the block is not drawn.
Use never spends on the first click: it replaces its row, in place and never in a dialog, with the question ("Use one
Full reset?"), a muted line saying what it clears ("Clears the session and weekly limits.") and, at the right,
`Cancel` (outline) and `Use` (`Destructive`: a reset does not come back). Cancel gives the row back; Use sends the
spend, and the answer comes as a toast and an immediate re-read of the usage. While one is out every Use is disabled.
The panel closing drops a confirmation left open.

With logins to switch between, a separator under the usage, then what the footer's menu always held: the logins to switch to, each with its avatar
and its five-hour usage and the chosen one checked, "Use Global" while the project sets its own, and the other
providers' logins, each provider a label (its glyph and name) over its rows: the submenu of the old menu, opened in
place. Choosing a row runs it and closes the panel. A provider with no per-profile reading (anything but Claude Code
and Codex) on a profile other than Personal, the machine's own login, shows a single muted line in place of the usage,
saying usage is not measured per profile; while the first
reading is in flight there is no usage section at all, rather than one that says nothing. The rail's profile button
keeps the old menu.

### The chat view

A session whose view is chat draws its conversation in place of the terminal, on the same island and in a
layout's pane alike (`chat_view.go`, `chat_items.go`). The composer's box takes the pane's width less `Space(6)`
each side, up to 960px (`chatPadX`, `chatBoxMax`): wide enough for a long line of code, and no wider than a
paragraph can be read in. The conversation is a column on the same axis, `Space(4)` (16px, `chatTextInset`)
narrower each side, so its text lines up inside the box and not with its border. Its rows are `Space(3.5)` apart, and a turn starts after `Space(9)` (`chatTurnGap`), in the person's
message or the day line over it, so turns read as groups.
A conversation shorter than the view rests on the composer, its newest message at the bottom and the older
ones above, and a new one comes in under them and pushes them up, as in a messaging app; one taller than the
view stays pinned to its end while items arrive, until the person scrolls up. The list ends `Space(6)`
(24px, `chatComposerGap`) above the composer's box in both cases, so the last message and its time never touch it.
The list's scrollbar shows only while the rows outgrow the view: a message sent into a conversation that fits
draws none while it grows in, because the room above the rows is measured in the frame that draws them.

- **The person's message** is a balloon in `Accent` against the column's right edge, at most 70% of it:
  `RadiusXL` but for the corner by that edge, `RadiusSM`, its tail; `Space(4)`×`Space(3)` inside. It breaks "rows, not cards" on purpose: a turn starts where a message does, and the eye finds it by the
  box faster than by a rule. Its text is Markdown as the agent's is, at `TextSM` (the renderer's sp scaled
  down), every line the person broke kept broken.
- **The agent's text** is Markdown at the prose size (`ui/markdown`). A link is underlined, in its color; a link to github.com, a path
  in code (a folder when it ends in a slash, else a file) wears a 13px mark before it, GitHub's, a folder's or a file's
  (`decor.go`). A path the checkout's listing holds, from the checkout or as the tail of a file the way a
  terminal's path:line is found, is a link the terminal's way: while Ctrl (Cmd on macOS) is held the hovered one
  is underlined in its color and takes the hand, and a click with it opens a file in the stage editor
  (`followCodeRef`) and a folder in the dock's Code tab, unfolded down to it (`chat_paths.go`). A path it does not
  hold stays text. Anything else in code is plain. An answer has no header: who wrote it is the
  foot's (see A message's time); the provider's avatar marks no row of the chat: no answer, call, notification or closing line. A thought is a muted fold, closed, with the
  sparkles before its label.
- **A turn's tool calls** fold by run: the calls between two of the agent's texts (or the person's message and
  the first text) fold into one muted line where the first of them came. One line for the whole turn stood where
  the turn began while the agent went on working minutes below it; a run's line stands where its calls ran. A
  thought does not end a run. No avatar and no hairline: every ADE surveyed (Paseo, Orca, Sikemux, Cursor,
  Codex and Claude Code desktop) leaves the agent's mark off its calls. A run of one call is no fold: it is that
  call's row, its target in view, the complaint most heard of the apps that fold it. Several read as what they
  did, a clause per kind in one sentence ("Ran 4 commands, read 2 files", `chat_calls.go`: edited, ran, read,
  searched, fetched, then "used N other tools"), then, muted, how long the run took, from its first call to its
  last call's end, and "· 1 failed" in `Destructive` when one did (a call lich opened as a card is not a
  failure); the turn's total is on its closing line. While a call runs the line is that call instead, its
  running verb and target shimmering ("Running go test ./…", cut at 60 characters), the run's time counting,
  and "· 3 tool calls" muted. Open, each call is a 28px row under the line, indented past its chevron, with no
  connector. The line starts shut, whether the turn is in flight or ended, and only a click opens it, which
  holds as the person left it. Open, it shows only its last 5 calls (`chatCallsShown`): a turn can run fifty,
  which would push the view past its height. The rest wait behind a first row, muted `TextXS`
  "Show 12 earlier calls", lit on hover, whose click shows them all and turns it into "Show less". A row is its
  tool's glyph muted (a Lucide mark by tool name, Claude Code's, Codex's, ACP's kinds and the names Antigravity
  and Kiro CLI put in their place, one generic wrench for the rest), its verb muted (the tool's own name, an MCP
  tool's without its server, when it has no kind) in a column as wide as the run's widest verb, up to
  `Space(24)`, so the targets line up, what it acted on in mono, then at the right the time it ran in muted mono.
  A call that went well says nothing more; a failed one turns its glyph and verb `Destructive` and shows the
  output's first line in it. A call lich opened as a card (`chat_veto.go`) keeps that `Destructive` verb, the
  tone the provider's veto has always worn, but trades the glyph for the card's provider mark and the time and
  output for a pill per card at the right: the card's state dot, its name and `arrow-up-right` in
  `Destructive`, on a 8% fill of it with a 35% edge, both lit on hover (18% and full), and a click opens the
  card. The veto's own milliseconds are the hook answering, not work, so they are not shown. A time that stands still reads in milliseconds under a second ("476ms"), the line's
  own time too; a call whose end nobody reported (most histories) shows no time. A call that changed a file is
  not in the fold: it is an edit, a line of its own (see An edit). The group never opens on its own. Only
  the turn in flight is alive: a call an ended turn left running (interrupted) stops with it, its time up to the
  turn's end, or none where nobody saw it.
- **A turn's closing line** stands once, after the last item of the conversation's last turn, when that turn
  has ended: "Done" in `TextSM` medium, with no avatar,
  `Foreground`, then, when it ran any, muted "· 34 tool calls · 6m 4s", all its runs' calls and how long the turn
  took, then muted "· no changes" or "· 3 files" and the lines the turn added in `TonePass` and removed
  in `Destructive` ("+12 −4"), as the diffs draw them, and a 1px `Border` hairline to the column's edge.
  "Stopped" stands for a turn the person cancelled, and "Failed" in `Destructive` for one that ended in an
  error. The files and lines are the click: they open Review as the footer's changes item does. The line
  leaves when the person's next message begins a turn, and none stands while a turn runs, where the live line
  is; older turns have none, in a history read back too. A turn whose checkout lich could not read (not a
  repository, or a commit made during the turn) says its word alone, and so does one read back with the
  history, whose end nobody reported to the window: the line never invents a count.
- **A compaction** (`chat_compaction.go`). While the turn compacts, the live line says "Compacting the
  conversation" in place of the verb, shimmering with its clock, and the `fold-vertical` glyph stands in the
  flame's place, squeezing to 0.55 of its height every 1.4s; under the words, indented to them, a `WaveBar` of
  11 segments at most 220dp wide in `MutedForeground`, a lit wave sweeping it: it never fills, since no provider
  says how far along a compaction is. Once over, a marker stands where it happened, between two 1px `Border`
  hairlines: the glyph, the title in `TextSM` medium `Foreground` ("Conversation compacted", "Conversation
  compacted automatically" for one the provider ran on its own), then muted "· 182K → 9K tokens" when both
  counts are known. A failure reads "Couldn't compact" in `Destructive` with the provider's reason muted. The
  person's /compact ends its turn in the marker, with no closing line and none of the command's output; an
  automatic one leaves its turn to carry on and close as any other. What the compaction's hooks printed is the
  marker's tooltip, and its summary opens under it on a click, a chevron at the marker's end saying so. A cancelled
  compaction draws nothing.
- **What is alive moves.** The text of a live step shimmers: a light band crosses it from left to right once
  every 1.6s (`Label.Shimmer`, looping frames only while it is drawn), and the time it has run counts beside
  it, "6s", "1m 12s", redrawn each second and not each frame. That is "Thinking" for the thought being
  thought, "2 tool calls" for the turn's line while a call runs, and, in a group the person opened, the name
  and target of each call that runs, its time counting from when it was asked for, a wait on a permission included. A running call's name is muted
  under the band, which lights toward `Foreground` and would not show on it; ended, it is `Foreground` again. The
  answer being written is its text alone, coming in dry: no caret at its end, which blinked through every pause
  and read as noise; the live line under it already says the agent is writing. A row new to the conversation enters
  with `ui.Presence.LayoutFlat`, its height and opacity rising with no scale (160ms ease-out, out in 180ms
  ease-in), so its text and the flame never draw at a fractional size; the conversation read when the pane
  opens, or after it, or drawn again after the TUI, does not play as arrivals. A list at its end stays at its
  end while rows grow in.
- **A row that becomes another changes in its slot** (`chat_motion.go`). A turn ending turns the live line into
  the closing line where it stood: the flame, verb and clock fade out in 120ms ease-in, then the word
  and count fade in by 280ms and the hairline runs from the word to the column's edge by 360ms, both ease-out;
  the slot eases to the closing line's height in 160ms, since the two differ by a few pixels. Neither row
  plays its own entry or exit. A question answered leaves the composer at once, and its answer enters the
  conversation as a new row does. A fold, a thought's text, a group's calls or an edit's diff, opens and shuts with `ui.Collapse`, 200ms, its first draw landing where it
  stands, and a group opens and shuts that way at the edges of its run of calls; a call that comes into an open
  group enters as a row does. Sending a message while the list is scrolled up, queued or steered
  into a turn too, runs it back to its end in 220ms ease-out and pins it there. Scrolled up from the end it
  follows, the list carries a pill centred `Space(3)` over its foot (`chat_jump.go`): `Popover` with a
  `Border` hairline, fully rounded, an arrow down and muted `TextXS` "Latest", lit on hover; while the agent
  works, a `Primary` dot leads it and the words read "New activity", shimmering. Its click runs the list to its
  end as a send does, and it is gone there; Paseo and Orca carry the same. Switching between GUI and TUI fades the view that comes in
  from the background in 150ms ease-out, from the first frame that draws it; the one that leaves goes at once.
  The chevrons, the option highlight, the Send and Stop swap, the composer's height, a tool's state, the copy
  check and the footer's changes stay dry.
- **A permission** is a box on `Sidebar`, `RadiusMD`, `Space(3)` padding (see Boxes inside a screen): the tool,
  its diff, and a `SizeSM` button per answer the provider offers, the first allow filled. Enter takes the first
  allow and Esc the first reject. Answered, it shrinks to a muted line naming the answer.
- **An edit** (`chat_edit.go`), a call that changed a file, is a line of its own out of the turn's fold, and so
  is each file a call with no diff of its own changed (a shell command's `sed`, a script), right after that
  call (`ChatToolCall.Files`, `fileRows`). It stands on
  `Card` in `RadiusMD` with a 1px `Border` hairline: the file's icon (`th.FileIcon`), its folder muted and its
  name in mono `TextSM`, the chat's own size, with a `Space(4)` icon in a `Space(3)`×`Space(2)` inset, the path
  relative to the checkout, then `+a -d` (`th.DiffStat`). No tool name, glyph
  or time: the line is the file. While the call runs its path shimmers and no counts show; failed, "failed" in
  `Destructive` stands in their place. Hover fills it with the kit's hover; a chevron at its right shows on
  hover and while open. A click unfolds the diff under a hairline, inside the same box, and a second folds it;
  a Ctrl+click opens the file in the editor instead. The open diff is the Review panel's code view
  (`chat_edit_diff.go`, `ui/diffview`), unified, in the editors' code font on `Background`: line numbers, the
  code's colors, the words that changed, a bare separator between hunks, never git's header lines. It shows
  `chatDiffShown` (20) rows and a muted "Show N more lines" under them for the rest. A diff the code view
  cannot read, and a permission's, draws its lines in their tones without the header and `@@` lines.
- **A question** (`chat_question.go`, `chat_question_draw.go`) waits over the composer, not where the agent
  stopped: it is what holds the turn, so it stands where the person types and does not scroll away. It is
  Claude Code's AskUserQuestion shape, which every provider's request for input is read into, in a box on
  `Sidebar`, `RadiusLG` with a 1px `Border` hairline as the composer has, `Space(2)` over the composer. Its
  head is the ask's header, muted `TextXS` in its own case on no fill, and a ghost `SizeIconXS` × at the right
  that declines; several asks turn the headers into pill tabs, as a project tab is (the one on show filled in
  `Accent`, each led by its muted number, which a `TonePass` check replaces once answered, each a click back to
  it; the pill's padding lines its text up with the question's), with no count. Under it the question in `TextSM` medium. Each option is a row: a keycap with its
  number (a checkbox when it takes several), the label in `TextSM` regular and the description muted under it;
  the row the keys stand on is lit on `Accent`, `RadiusMD`. A label ending in Claude Code's "(Recommended)"
  is drawn without it, a `Primary` `TextXS` "Recommended" beside it; the reply keeps the label as it came.
  The last option row is "Other…", a bare field whose placeholder reads as its label. There is no Send button
  and no summary, as in Claude Code's terminal: a single choice answers on a click, its digit or Enter on the
  lit row, and that row carries Enter's keycap (↵) at its right; several are marked with a click, Space or
  Enter, and a last row, an arrow and "Continue" with "2 marked" muted, sends them, muted until something is
  marked or written. Answering an ask moves to the first one left, and answering the last one left sends them
  all. An ask with no options is a textarea with "Enter sends · Esc declines" muted under it. When the options
  carry a preview, the list takes the left half and the highlighted one's preview stands at the right in mono
  on `Background`. The keys go to the card while it waits, Esc declines, and the composer stays clickable: a
  message sent from it declines the questions and reaches the agent. While it waits the flame is left out, as
  for a permission. A plan's question waits over the composer too, the plan itself staying in the
  conversation. Answered, it is the person's own message at the column's right, the bubble of what they type:
  a `TextSM` line per ask, the header muted and the answer, their own words in quotes; declined, a muted
  "Not answered" at the right.
- **A notice** is a muted line; the turn in flight ends the column with a pixel flame (`chat_flame.go`: an 11x14 sprite of 1.5dp pixels, drawn as rectangles, in 16 frames of 100ms; only its 11 bottom rows take the line, the tips and sparks burning over the line above; a window without focus looks in at `StillFrame` instead) and a verb of the turn, shimmering like "Thinking" with the time it has run beside it; the verb is picked from the language's own list (`chat.view.verbs`, `|`-separated, each language writes its own length) by the session and the turn, so it holds for the whole turn; left out
  while a permission or a question waits: its box already says so.
  The flame's colors are tokens, never a fixed hue: the core is `Primary` halfway to the lighter of `Foreground` and
  `Background`, the body `Primary`, the edge
  `Primary` at 0.65 over `Background`, and the base and the sparks going out `Primary` at 0.35. The flame burns in
  the theme's own color, as the sidebar's souls do. It does not use `ToneWait`: the turn running is not the
  turn waiting on the person, and that tone already says the second.
- **An empty chat** (its conversation read and holding nothing: a new session, or after `/new`) draws its start
  screen (`chat_start.go`): lich's mark in pixel art, the Welcome screen's still frame (48x68, pixels of 1.5dp,
  its five tones from the tokens: `Foreground`, a mix of it and `MutedForeground`, `MutedForeground`, and two
  mixes of `MutedForeground` toward `Background`), then a question in `TextBase` medium naming the project in
  mono, then the composer and its line, the block a little over the middle of the pane (`chatStartRise`). The
  question is one of the language's own list (`chat.start.lines`, `|`-separated, one per verb of the live line),
  picked by the session as the turn's verb is, so it holds while the screen is up. The first item fades the mark
  and the question (120ms ease-in) and runs the composer down to the foot (220ms ease-out) while the
  conversation grows over it. A pane too low for the block keeps the composer at its foot, with neither.
- **The composer** stands at the column's foot: a textarea (Enter sends, Shift+Enter breaks the line) in a
  `RadiusLG` box, three lines of text tall at least, growing with the text to a ceiling and scrolling past it.
  Its text and its placeholder, "Ask {provider} about this project", are in the interface font at the body size
  and leading of the messages (15sp at 1.5), `Space(4)` in from the left and `Space(3.5)` down from the top.
  A suggested next message takes the placeholder's place, in its muted ink and at its spot, with no key hint
  drawn, only while the box is idle (empty, no file, no turn, nothing queued or waiting to go); Tab puts it in
  the box as text; typing hides it and an emptied box shows it back, until a message is sent or queued
  (`chat_suggest.go`).
  Inside the box stand only the provider's controls, one line under the text: the footer's attach as a "+",
  the model with its effort in one chip ("Opus 5.5 · high") and the mode, each a ghost chip with a chevron.
  After them the footer's context ring with its percentage, the same readout as the terminal's balloon, when the
  session reports a window. Last, while the session has open sessions it opened (`OriginSessionID`, the sidebar
  tree's), a wide ghost chip of them (`chat_children.go`, Paseo's "subagents" chip, made to say more): up to three
  of their providers' pixel marks, "3 sessions" and, in `ToneWait`, "· 1 waiting". It opens a panel the shells'
  way: a row per child, its mark with a dot of its state on the corner (`ToneWait` waiting, `Primary` working,
  `TonePass` done), its name, "provider · model · branch · 4m" muted and, while it waits, what on in `ToneWait`;
  those waiting first, then those working, then the rest under a hairline, muted. A row opens its session.
  The mode's menu lists the provider's choices, the current one checked, and picking one sets it
  (`ChatSetOption`). It is a `DropdownMenu` on `Popover`, the surface of the model panel and every other menu;
  a theme whose `popover` sits darker than `background` makes both look sunken, and the fix is the theme's token.
  Every mode is named by lich in the window's language, in sentence case like the rest of the menu, never with
  the provider's raw id or English name (`chat_modes.go`, per provider); only an agent the person made keeps the
  name they gave it. The chip's tooltip is lich's "Mode", not the provider's option name. A `Danger` choice (Claude
  Code's bypass, Codex's full access) is drawn in the rainbow, the theme's `GraphLanes` in hue order red to
  magenta: its name and a filled bolt on the chip, its name in the menu, with the filled bolt in the icon's place
  when it is not the checked one. The model chip opens one panel upward, in two columns with a hairline between
  (`chat_picker.go`): at the left a search that has the focus, which filters the models as it is typed, then
  32px rows with the name, "Default" muted when the provider's default has a name that does not say so, a
  `Ctrl+1`..`Ctrl+4` keycap on the first four (the chord works with the panel open) and a check on the current
  one; past six models the rest wait behind "Show more", which a search does not need. The arrows walk the
  rows, Enter takes one, Escape closes. At the right, the effort when the provider offers it: its title with
  the model in force muted beside it, and a row per level under a signal glyph of five bars that rise with the
  level, the lit ones in `Foreground` and the rest muted, a check on the current. Choosing a model leaves the
  panel open for the effort; choosing an effort closes it. A provider with only one of the two has that
  column alone. The panel opens above the chip, a gap over it, and holds at 420px or the room above, whichever is
  less: the search and the effort stay put and the models scroll under the search. A category the provider does not offer has no chip. A
  dangerous choice (`Danger`, Claude Code's bypass mode) wears a bolt in `ToneWait` while it is active. At the
  right end, Send, a `Primary` icon button (arrow up), or Stop, an outline one (square), each named by a
  tooltip and no word; while the agent works the chips take no click and a tooltip says why. What does not
  fit is left out with the items after it, never wrapped. The rest of the footer stays out of a chat. Under
  the box, outside it with no fill, one line, `Space(2)` (`chatCrumbsPad`) under the box and `Space(2)` clear of the island's
  bottom edge, over the composer's own pad, in a lone session and in each pane of a split, its items `chatCrumbGap` (`Space(3)`) apart, one rhythm
  along the line: at the left a `TextXS` muted project, a slash and the branch,
  which opens the Git panel as its balloon did, then the changes, only while there are some
  (files above 0): in the left's style, a small file-diff glyph, the file count and `+added -deleted` (`TonePass`,
  `Destructive`) in `TextXS`, on the same line and height, no fill at rest and the kit's hover fill under the
  pointer, which stands `Space(1.5)` by `Space(1)` (`chatChangesPadX`, `chatChangesPadY`) round the text and grows
  outward from it: the item takes the room of its text alone, so the text ends `chatFootPad` in from the box's
  right edge, as the left side starts, and the fill reaches the edge. A click opens Review. It is never drawn pressed, as the footer's button is while the dock shows Review:
  on this line a lit item reads as a button switched on for no reason. Last, after the branch or the changes,
  the branch's pull request (`prBadge` in the line's size): a click opens it on Pulls or in the browser
  (`prTarget`), its title in a tooltip; it shows with no checkout or changes, and the branch truncates first, so
  neither it nor the changes is ever cut. Attach, changes and the branch show only when the user's footer layout has them, and
  in every pane, each for its own session (its checkout, branch and changes, with or without the focus, but only the
  followed pane's items take a click); the attach "+" and the context ring are the followed session's alone, so another
  chat pane of a layout has the provider's chips and Send.
  Ctrl+Z (Cmd+Z on macOS) undoes the last step of the text, bringing back the caret and selection it started from,
  and Ctrl+Shift+Z or Ctrl+Y redoes it (`ui.EditHistory`, which takes the keys from Gio's editor: its own undo is one
  keystroke a step and cannot be cleared). A run of characters typed or deleted side by side, less than a second
  apart, is one step; a paste, a cut or a replaced selection is a step of its own. Sending a message, or a refused
  one coming back, forgets the history. Only the focused composer takes the keys: the terminal's Ctrl+Z is still
  the agent's suspend, and the sketch, the browser pane and the resolver keep theirs under their own focus.
  `/` at the start of a line opens the provider's commands and skills over the box, as wide as it and `Space(2)`
  above it, on the popover surface (`ui.PopoverPanel`, which takes no focus: the box keeps the keyboard), up to
  320px and scrolling past it (`chat_slash.go`). A row is the model picker's: a glyph, `square-slash` for a command
  and `wand-sparkles` for a skill where the provider tells them apart, the command in medium with its arguments
  muted and the description muted on the rest of the line, lit on `Accent` under the cursor. Typing filters it,
  names that start with what was typed first; the arrows walk it, Enter or Tab put the command in place of what
  was typed, with a space, unsent, and Escape shuts it until the start changes. A path, a slash mid-line or a
  start that matches nothing opens nothing, so Enter sends. An Enter or Tab that comes in the same frame as the
  last keys typed waits for the editor to hold them before it picks, so "/mcp" typed fast never leaves "cp"
  behind. lich's own commands lead the list: `/new` (and Claude Code's `/clear` and `/reset`, read as it) starts
  an empty conversation in the card (`chat.New`) instead of reaching the agent, the conversation it replaces
  leaving at once with no row playing out; `/mcp`, `/skills`, `/plugin`, `/permissions`, `/hooks`, `/memory` and
  `/agents`, where the provider reports what they open (Claude Code, Codex), open a panel instead of being sent.
  The lists and the panels take the room above the box: on the start screen, where the box stands mid-pane,
  only what is above it there.
- **A panel over the composer** (`chat_tools*.go`, `chat_plugins*.go`, `chat_permissions*.go`, `chat_hooks.go`,
  `chat_memory*.go`) stands where the slash list does, on the same popover surface, as wide as the box and
  `Space(2)` above it, up to 460px and the room above, scrolling past it. It is the provider's own TUI screen for
  the command, drawn as rows: a head with the title in `TextSM` semibold and the count muted beside it (and a
  ghost `SizeXS` control at its end, a sort or nothing), tabs as ghost pills under it where the screen has tabs,
  then the rows grouped under `TextXS` semibold group labels with their file muted beside them. A row is the
  model picker's 32px row: a state glyph (a check in `TonePass` for on or connected, `circle-dashed` muted for
  connecting, a cross in `Destructive` for failed, `circle-alert` in `ToneWait` for a sign-in it needs, an empty
  circle muted for off), the name, and its facts muted at the right, a description under the name when the row
  has one. A search is a bare row of the panel (`Input` with `Bare` and the row's height), the model picker's
  search, never a filled box inside the panel: the panel is already the box, and two stacked rows of different
  heights side by side break the hairlines. A click or Enter opens a row's detail in the panel: Back, a ghost
  `SizeXS` button with a chevron, and the name; label and value lines, the label column `Space(32)` muted, a file
  or a command in mono and a word in the body font; then a hairline and its actions as `SizeSM` secondary
  buttons. The keys stay on the box (a panel never takes the focus: rows of a focused non-editor tag drop out of
  the semantic tree), the arrows walk the rows, Escape goes back from a detail and then shuts the panel, and
  typing in the box shuts it. A provider that reports nothing for the command says so in one muted line. lich's
  own server, plugin and the like show no action that would cut lich off.
  `@` at the start of a word opens the checkout's folders and files in the same place, on the same surface and
  rows (`chat_mention.go`): a folder's glyph or the file's icon, the name in medium and the folder it sits in
  muted on the rest of the line, so a deep path cuts its folder and never its name. It ranks as Claude Code does,
  its 15 best; Enter puts `@path ` in, Tab first puts in what every path shown shares and keeps the list open on
  it, and Escape shuts it until the word changes. An `@` mid-word, as in an email, opens nothing.
  A file dragged over a session drawn as a chat lights the terminal's drop hint over the whole session, and
  dropped, or picked with the attach "+", it stands over the text inside the box as a card (`chat_attach.go`): a
  `Space(30)` square, `RadiusLG`, the box's fill and a `Border` hairline, `Space(2)` apart in a row that scrolls
  sideways. An image is its thumbnail, covering the card; any other file its name without the extension,
  `TextXS` up to three lines, over its type in `Text2XS` semibold muted capitals at the card's foot. The ×, a
  `SizeIconXS` secondary button `Space(1.5)` in from the top right corner, shows while the pointer is on the card,
  and a click on the card opens a dialog with the file's name over its path and the image whole, or the file's
  size. The person's message shows its cards the same way, without the ×, at the column's right over the text.
  These cards are the chat's one exception to rows over boxes: a thumbnail needs an edge to read as a file, and
  Claude's web composer, which the person asked for, draws them so.
  Ctrl+V with an image and no text on the clipboard makes a card the same way.
- **What has not reached the agent yet stands over the box**, in the box's column, before the text: a message
  written while the turn runs waits in the queue (`chat_queue.go`): a head line, the `clock` glyph and "2 queued"
  in muted `TextXS`, with Send now (a ghost `SizeXS` button with `arrow-up`) and the Ctrl+Enter key cap at its end,
  always shown while there is a queue; Send now and Ctrl+Enter send the whole queue, and what the box holds after
  it, into the turn in one message. Under it one muted `TextSM` line per message, its text under the head's count,
  truncated, the kit's hover fill under the pointer, and × (a `SizeIconXS` ghost button) shown only on hover; a
  click on the line takes it back into the box, and "↑ edits
  the queue" stands muted in the box's foot while there is one. Above the queue, the agent's task list
  (`chat_plan.go`) is one compact line at the conversation's inset: the task in progress behind a 14dp 3x3, in
  semibold `Foreground` and shimmering as the live line's verb does, then "2/5" in muted `TextXS` and the
  chevron right after its words; a click unfolds the task before it and the next, the chevron staying on the
  running one, and a muted `TextXS` line under them counting the rest ("+2 more · 2 of 5 done"), and a second
  click folds it. The 3x3 is the sidebar's, drawn with `drawCells` at one side so a zoom keeps it whole: a
  block in `TonePass` done, with no strike, a diagonal wave in `Primary` running (110ms a step, the snake's
  fade and glide), the waiting ring still in `ToneWait` once the turn stops with it open, and faint cells
  pending. It never takes the live line's place
  and goes when every task is done. A message sent waits 2s in the window before it reaches the agent
  (`chat_hold.go`), drawn sent with its turn running from Enter, so the wait does not show; Esc or Stop in that
  time takes it back.
- **Shell mode**: a line that starts with `!` turns the box's edge and fill toward `ToneWait`, the tone of a mode
  that runs unchecked, and the box's foot shows "Shell mode · Enter runs the command" in that tone in the chips'
  place, which do not apply to a command; Send stays at the right. The command's call is a row of its own under
  the person's message, hung by the connector, its output whole in mono under it, 20 lines and "Show N more
  lines". Shift+Tab walks the mode chip's choices in the provider's order, never onto a `Danger` one.
- **Background shells** (`chat_shells*.go`): while one runs, a chip in the box's foot says "2 shells · 1 monitor";
  it and `/tasks` open a panel of them in the panel's place, each row a state glyph, the command in mono and its
  running time, the detail its output tail in mono and Stop. While the turn waits on a command Claude Code can
  move, the live line ends in a muted "· Ctrl+B to run in background", and Ctrl+B in the box moves it: the box
  holds that chord ahead of the window hotkey bound to it.
- **The end of plan mode** is a question card (`chat_proposal.go`): the plan in Markdown over it in the
  conversation, the card's header "PLAN" and the question, and the choices as the card's rows, the last a field
  for what to change.
  The box has no focus ring: its 1px border is the same with the keyboard and without (`TextareaStyle.Steady`; the
  kit's inputs keep their `ring-3`). The blinking caret already says where the keyboard is, and in a chat the
  composer is nearly always the focused thing, so a ring around the biggest box on the screen only reads as a
  loud frame that never goes away.
- **Copy** is an icon-only button that shows only while the pointer is over a message, on the line of its time and
  at the left of it: for the person's message the line is at the right edge, for an answer at the left. It copies the item's whole text, an answer's Markdown source, through the
  window's clipboard path, and becomes a check for 1.5s. It has no tooltip: the glyph says it. The line is as tall with it as
  without (`chatCopySlot`), so it pushes nothing; the button is a little larger than the line and reaches past it. Texts the
  agent writes one after another in a turn, nothing between them, read as one answer: only the last keeps the
  time and copy line, and its copy takes the whole run, so the texts stand a row gap apart, not a line apart.
- **A message's time** is a `TextXS` muted line, the window's short clock ("14:32") under a tooltip that holds
  the footer clock's full date: under the person's box at the right edge, always, led by its day when that is
  not today ("Nov 17 · 23:05", the year too past this one); under an answer's text at the
  left, its foot, only while the pointer is over the answer (with the copy button before it; the line is there at
  rest, empty, so nothing moves). The foot reads "Sonnet 5.5 · high · 14:32": the model and effort that wrote the
  answer as its own item says them (`ChatItem.Model`, `Effort`; the options are never read for it, so changing the
  model leaves old answers as they were; an empty one is left out), then the time. Its tooltip is the provider's
  name before the full date.
  Between two turns that start on different days a muted, centred line names the later day: Today, Yesterday,
  else its date, with the year when it is not this one. A message with no time draws none.

### The session's strip

The balloons' mirror: an agent session's terminal, or its chat, gives up a band at its top
(`sessionBandHeight`, `session_band.go`): the selector and `balloonInset` over and under it, so the PTY has fewer
rows and nothing floats over the agent's banner. It follows the session's kind like the foot's band, so the grid
is resized once, and the Git panel rising from the foot does not take it back. A plain terminal, a service's
console, Home and a split carry none. The chat starts under it.

The strip has no ground. At its left the provider as the window names it, then the session's label: `TextSM`,
the provider in `MutedForeground` and the label in `Foreground`, cut with an ellipsis when the room is short. At
its right the GUI | TUI selector (`chat_switch.go`), which takes its room first: the segmented control in its
`Border` track, items h-5 with a `Text2XS` label and a size-3 icon, the view on screen in `Accent`. Only a
provider `chat.Kinds` lists gets the selector, so a Codex or Crush session's strip is the name alone. While the
agent works, or a switch is on its way, it takes no click and says why in a tooltip.

A split's pane is headed by this same strip (`sessionHeader`), at the same height, with the same type and
spacing and no hairline under it, so a session reads the same alone and in a layout. Only the pane's own controls
are added at its right, after the selector: the delegates chip, then zoom and unsplit, or while zoomed the chip
that names what the zoom hides. Out of focus the label turns `MutedForeground`. In a layout whose panes come
from more than one project, the project leads the muted run, "lich · Claude Code · Claude 3", and the label is
the part cut first; a layout of one project names none.

A terminal that does not have the keyboard (a split's other panes, any pane while a field or dialog is typing, every
terminal while the window is out of focus) draws its cursor as a 1px outline in the cursor's color; the one that has it
draws the block.

### The session's browser

A page pane (`browser_pane.go`, `browser_tabs.go`) is three rows on the pane's fill, no seam between them.

One gap, `Space(2)` (8px, `browserChromeGap`), sets the whole rhythm: from the pane's top edge to the tabs, from
the tabs to the address, from the address to the page's card, and from the card to the pane's sides and foot. The
tabs also keep it from the pane's left side. `TestPageChromeKeepsOneGapBetweenItsRows` measures them; a row added
to the chrome takes the same gap rather than a number of its own.

- **Tabs** on the top row, the page's tabs always, even one, then the new-tab button, and the pane's expand,
  minimize and close at the right. A tab is its page's 12px icon (`BrowserTab.Icon`), a neutral 12px square
  where the page has none or it did not load, and the page's title in `TextXS`; while the tab's page loads
  (`BrowserTab.Loading`) the kit's `Spinner` takes the icon's place, same 12px, in the title's color; the front tab is filled `Accent`, its title `Foreground` in the focused
  pane and `MutedForeground` in the others. A tab behind has no fill until hovered. There is no pane title and
  no globe.
- **The toolbar** is back, forward and reload as icons in `MutedForeground`, the address, then the zoom (its
  percentage shows only off 100%) and DevTools. The address is a pill, `Accent` fill, `RadiusMD`, no border, in
  the interface font: at rest the page's host and path with no scheme (`shortURL`), while edited the whole URL,
  selected; on a blank page and on the new-tab page the placeholder. What a button says lives in its tooltip: the Select tool's is
  "Click an element to leave a note". Open in a window is not here: it is an item of the browser row's menu in
  the sidebar.
- **The page** is a card `Space(2)` under the toolbar and from the pane's left, right and bottom, `RadiusLG`, its edge the 1px
  `Border` hairline. The notes bar floats over it, as before, except on the new-tab page.
- **The new-tab page** is the backend's HTML, drawn with the window's tokens sent over `browser.NewTabPage`
  (`browser_newtab.go`): `Background` for the page, `Card` for the address field, `Accent` for a hovered row,
  `Border` for the field's edge and `MutedForeground` for the section labels, the commands and the ages. One
  column, 560px wide, centered, with nothing over it: the address field (42px, 9px radius, an inset `Border`) 104px
  under the page's top, then the sections, each a 10.5px uppercase `MutedForeground` label over 34px rows with no fill until hovered. The
  tab is named New tab, the toolbar's address is empty, and there is no Select and Draw bar.

### The editor

A file opened from the Code tab, a `path:line` in a terminal or an
agent's `show_code` takes the whole stage, in place of the session, as a service's console does
(`stage_editor*.go`). The dock stays beside it, its Code tab the tree and the search.

- The header is a `Space(10)` strip on the 8px edge: the folder in `MutedForeground` mono, the file's name in
  `Foreground` mono at `font.Medium`, and the unsaved dot (`ToneWait`, 6px) after the name while the text is not
  the disk's: between an edit and its save, or while a failed save or a disk change waits. There are no Undo or
  Save buttons: the editor saves itself (`autoSaveDelay`, 1 s after the last edit, and at once when it is left),
  and Ctrl+Z and Ctrl+S stay on the keyboard.
- A 1px rule in `Border` (`th.Over(th.Border)` on the editor's fill) runs down the whole code area on the
  gutter's last column (`Geometry().GutterWidth-1`), between the line numbers and the code, the usual half cell
  before the text. The `ToneWait` mark of a changed line stays in the gutter cell, left of it.
- The code is on the island's own fill. A line the user changed since the file was read or saved wears a
  `ToneWait` strip down its gutter cell, and a run of deleted lines marks the line that took their place.
- The cursor's line, and no other, ends in its blame, `blameGap` (32) past the code: "author, age · summary"
  or "You · Uncommitted changes" for a line not committed or not saved, `TextXS` in `MutedForeground`, leaning
  as italic (Onest has none, so it is sheared, `oblique`), cut with "…" where the code area ends and left out
  under `blameMinRoom` (80). Its tooltip is the short hash and full date, the author, then the whole message.
- An agent's note and the session comment box sit in a strip under the header, over the code, the note naming
  its lines: the editor hangs no widget under a line.
- A disk change under unsaved edits is a strip under the header tinted `ToneWait` at `editorBannerTint`
  (0.14): one sentence, then Keep mine (outline) and Reload (secondary), never a second `Primary`, and under it
  the diff of what the disk changed since the read, unchanged runs collapsed to git's three lines of context, at
  most `diskBannerDiff` (240) tall so the code stays in sight.
- Another file or Close asks nothing: the text is saved first. Only a failed save or an undecided disk change
  asks, in a dialog: Cancel (outline), Discard (destructive), Save (`Primary`).
- A Markdown file an agent left in the session's artifact folder (the dock's Artifacts tab) opens in the same
  editor, on the stage and already rendered, read only: the header carries a `TextXS` `MutedForeground` note after the
  name ("From the agent · read only"), nothing is saved and the unsaved dot never lights, the source view takes
  no edits, and the context menu that hands lines to a session is left out. Its row in the sidebar is the file's name
  like any other, closed from the same menu.
- In the sidebar the editor is a row under the session working in its checkout: the file glyph and
  the file's name in mono, the unsaved dot after the name, lit as an active session is while the editor is on the
  stage. Its only action is Close, in its menu and its `⋯`; there is no ✕ on the row. In the rail it is the glyph
  in the card's 32px square, the path and the unsaved state in its tooltip.

### The Welcome

The first launch's screen, and the menu's Welcome after it (`welcome_view.go`, `welcome_grid.go`). It covers
the window's whole content under the top strip, which keeps the window's controls: one island of `Background`
on the canvas, inside the `gutter`, as the screens under it are, never a box floating in the middle of the UI.
It takes every press and the focus off what is under it, cuts a browser page under it (`coverPages`), and
Escape, Enter and Continue close it, the first launch included. A sign-in dialog it opens draws over it and
has Escape first.

- **Two columns, laid out for the room there is.** The books take half the island or two 300dp cells, the
  larger; the intro the rest. With less than 360dp left the intro goes, and its foot (Esc's hint and
  Continue) ends the line under the books. Cells share the island's height and center their row in it.
- **The intro**: the logo at the largest whole pixel that fits, up to 5dp; the title at 54sp, the line with
  lich's name muted, where the language puts it; the line under it in `TextBase` muted; the foot, with
  Continue the view's one `Primary` button at `SizeSM`.
- **A cell is a row, not a card**: seams of 1px in `Border` between cells, no fill at rest. The book at 3dp a
  pixel, then the name at 19sp semibold, the state at 13sp in the muted color (`TonePass` once ready,
  `Destructive` for a failure, whose whole line is in the tooltip), and the state's one action as a
  `SizeXS` secondary button. The default provider's book says Default in text, never a button.
- **Choosing is the cell itself.** A ready book that is not the default is the whole cell's click, hovered
  in `Alpha(Accent, ghostHoverFill)` with its gem lit. The 46dp line under the books says how to choose
  only once two are ready.
- **The pixel art is symbols painted with tokens** (`native/grimoire`), the chat flame's four and 5:
  1 `Accent`, 5 `Accent` toward `MutedForeground` at 0.45, 3 `MutedForeground`, 2 `MutedForeground` toward
  `Foreground` at 0.55, 4 `Foreground`; a gray theme draws gray books. Every frame is a function of
  `gtx.Now` (a journey from the time its move was seen, the flame and the logo from the clock), never a
  goroutine that sleeps, so a test reaches any frame.

### Toasts

The window has one toast stack, in the bottom-right corner: the `gutter` over the right balloon and ending
at its right edge, or over the stage's foot at the window's edge where there are none (`layoutToasts`). It grows upward, and a toast comes in and leaves by the bottom edge. A
screen keeps its actions at the right of its header, so nothing the window raises stands in the top-right
corner; a question that waits for an answer goes on the same stack as a passing note, not in a corner of its
own. The stack stands over a session's browser page too: the page, a window of the system's, is cut under it
(`floatingBoxes`), as under the sidebar's and the dock's peeks.

### Body

A reading pane, a column of prose and settings rows under a title, is the one place that leaves the 8px
edge: the Settings pane sits at `settingsPanePadX` / `settingsPanePadY` (40px / 32px), its close button in
the corner at `settingsCloseInset` (14px). Every other screen body follows its rows (see the island's edge).

### Settings

The nav sorts its entries under group labels, General, Appearance, Agents and Workspace: `Text2XS`, semibold,
muted and uppercase, with a hairline from the label to the nav's edge (`navHeading`). Updates and Help stay in the
foot under a separator, with no label.

A section whose rows fall into groups gives each group an entry and a page of its own (`settingsSection.pages`):
General is Language, Window and Notifications; Appearance is Interface, Themes, Terminal (its font, its size
and Copy on select) and Footer. The entry is named after the group, the page's title is that name, and the page draws no
group heading under it. A section with a structure of its own (Providers, Sandbox, Extensions, Services, Version
Control, Hotkeys, Updates, Help) stays one entry. A search result's path names the section and the group.

The pane's content sits in a reading column centred at `settingsColumn` (760px), the title included; the
extension catalog and the Themes page keep the pane's width, since their grids take columns as the pane grows. A page title has no
subtitle; the Footer's alone holds its one line of how to use the editor, and Restore default at its right. A
count beside it, in muted mono, appears only on Notifications (switches on) and Footer (items shown).

Every row keeps its control at its right, a segmented control included; what the chosen answer does is a line
under the description (`settingRow.detail`), never under the control. The control drops under the text only
when the row is too narrow for both. Rows stand on seams, as the Welcome's cells do. A group of rows inside a
page is headed by the nav's own label, uppercase with its hairline to the column's edge (`groupHeading`).

A row a project changed says so in its scope line ("Global: Claude Code · Use Global") and nothing else: no side
bar, no tint. The search's flash is the one fill a row takes.

Ctrl+F (Cmd+F on macOS) focuses the search field while Settings is open, and its key cap sits at the field's
right end while the field is empty. It is Settings' own key, not a window hotkey, and not in the Hotkeys table.

#### The scope selector

The nav's top is the scope being edited, as one line of two (`settings_scope_menu.go`): the name over a muted
mono line, then what runs there and the chevrons, on the nav's hover fill while hovered or open, with no border.
Global's line is "Every project"; a project's is its path, its last parts behind "…" (`shortPath`). What runs is
the pixel mark of each provider its sessions run, once each and at most four, at the sidebar's 1dp a cell, led
by the sidebar's souls while one of them is mid-turn. The menu, on `Popover` and as wide as the trigger, holds
Global, a seam, then every open project in that same shape, with how many settings it changed as a muted mono
number after its marks ("· 1") and the check on the scope being edited. A project has no color here or anywhere:
the pane names the project in scope in a pill of its name alone.

#### Providers in Settings

A provider is its book's pixel mark wherever Settings names it, never the vector logo (`providerGlyph`): at 2dp
a cell in a box of 30, the room of Cursor's fifteen cells, so every name after one starts on the same line, on
the provider list, the Add provider list, a provider's title and the Sandbox's rows; at the sidebar's 1dp in a
picker's row, a select's lead and the plugin rows. A provider with no book keeps its vector mark in that box.
An installed provider in Add provider is its mark, its name and its switch on one line; a custom path or "not
found" stays on that line, muted.

Plan usage is the usage menu's blocks (`usageBlocks`): the plan's name muted, then a window a block, its share at
the right, the bar of twenty segments and when it resets. The account is on the profile's row, never repeated
under the bars.

A profile's row and a hotkey's row show their `⋯` under the pointer and while its menu is open, its room kept
at rest so nothing on the line moves.

#### Appearance, Hotkeys and the rest

- **Theme.** Interface has no Theme row: choosing, trying and installing a theme live on the Themes page alone.
- **Interface** is Zoom, the interface font and Chat width, a segmented control of Comfortable (the chat's
  960px column), Wide (1280px) and Full (the pane less its edges), which the chat's messages, its composer and
  its start screen follow (`chatColumnWidth`).
- **Version Control.** A tool is a row (`settings_vcs_setup.go`): its brand mark on a `Muted` square of 36px
  at `RadiusMD`, the name in semibold with its version in muted mono, and one detail line: who commits land as,
  or the account the CLI is signed in as, muted; what is left to set up (no account, no name or email) in
  `ToneWait`; a missing tool in `Destructive`. A ready tool ends on a `TonePass` check and a `⋯` (Edit identity,
  or Sign in with another account, then Open website); one with something left ends on its one Primary action:
  Install, which runs the package manager's command in a shell card with Website beside it as a ghost, or Sign
  in, which runs its auth login there. The path is the name's tooltip. Git's identity form opens under its row,
  inset to the text. The rows stand on seams.
- **Footer.** The preview is a stage on `Background` with the two balloons at its foot, each side's items read
  with example values and left out past the room, as over a session (`previewBalloon`). The editor's zones and
  chips stay as they are: dragging is the editor.
- **Hotkeys.** The chord is a `KeyCap` that records a new one on a click; recording, it is a bordered box in the
  ring. A key cap's edge leans from `Border` toward the foreground (`keyCapEdgeMix`), so it stays an edge in a
  theme whose `Border` is its `Muted`, as Dark's is. Reset is a ghost icon button only while the chord is not the default, and the row's `⋯` holds Record a
  new shortcut and Unassign.
- **Sandbox.** The machine's answer is a line, not a box: a shield in `TonePass` (a crossed one muted when it
  cannot confine), the sentence in medium and the backend in muted mono.
- **Help.** Open log folder stands at its row's right, the log's path as the row's detail; Repository is a link
  with the external glyph, as a provider's Documentation.
- **Services and Extensions** carry no line under their title.
- The diff layouts are Unified and Split.

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

### The extension catalog: cards and a detail dialog

Settings › Extensions draws the catalog as the site's extensions page does: a grid of cards, as many to a
line as keep each 260 to 360px wide (two in the pane at the window's default size; a wider pane takes columns,
never a bigger cover),
each a `Card` fill at `RadiusLG` with a 16:10 cover, and a click opens the listing in a dialog instead of
under its card. Both break rules above on purpose. The catalog is a shop window: the cover and the
screenshots are what someone chooses by, a row has no room for a picture, and an inline detail would push
the rest of the grid out of sight while it is read. The site and the window drawing it the same way also
teaches it once. The installed extensions on the tab beside it are cards of the same grid, as the installed
themes are: the cover is how a listed one is known, and the tab reads as the catalog it came from. Keep the
exception to these two screens; a list of things you have anywhere else stays rows, and an action anchored on
a row still opens beside it.

The catalog is what the screen opens on, every time. What is not the catalog stays off its page: the
installed extensions are a second tab, on a line of `TabsVariantLine` tabs over a `th.Separator`, with the
actions that belong to one tab at the right of that line and only while it shows; a rare way in (a GitHub
URL, a folder) is a `DropdownPanel` off a named button on the title line. A tab out of sight says what waits
on it with a count and a dot in a semantic tone, never by opening itself.

A card's text never changes its height: the name is one line and the description always holds its two, so
every card on a line stands as tall as the next and the skeleton matches them (a test pins it). The tokens,
the text sizes and the buttons are the window's.

The detail is sized by the window's height, not a dialog width: a screenshot at 16:10 across it takes about
45% of the window, so a tall or ultrawide window gets a wider detail. It is never narrower than the site's
736. An installed extension the catalog lists opens the same detail from its
card on the Installed tab.

An installed extension's card (`settings_extensions_list.go`) is the listing's cover, or its icon on the
empty cover's ground when the catalog has none, then its name and version in mono with the `⋯` at the line's
end, two lines of description (or why the catalog took it down, in `Destructive`), and at the foot its status,
the dot and its word with the source, the error and the permissions in the tooltip, with Restart, Remove or
Update at the right as `SizeXS` outline buttons. The foot keeps a button's height with no button in it, so the
cards on a line stand as tall.

A card's press sits under its buttons, never around them: the card lays out first, its click and hover areas
go under the content, and a press on Update or the `⋯` is theirs alone. Wrapping the buttons in the card's
clickable makes one press do both, as it once chose a theme while updating it.

### The theme catalog

Settings › Appearance › Themes is the extension catalog's shape for themes, and takes its exception for cards
for the same reason: a theme is chosen by looking at it, the ones you have included. The catalog tab is what the
page opens on, with All, Dark and Light (a pack is in each scheme one of its themes is in), the search and the
order over the grid; the installed themes are the second tab, as cards too. Install from… on the title line opens the import dialog (a
repository URL, a file, the template).

- **A cover is drawn, never a picture.** The catalog carries every theme in full, so a card's 16:10 cover is a
  small lich window in the pack's first dark theme: a sidebar, a person's message on the theme's accent, a code
  block in its syntax colors, the Done line in its tones and a strip of its terminal colors
  (`settings_themes_cover.go`). Under it, the name with In use or Installed, the publisher in mono, a dot per
  variant in its background, the count of variants and their schemes, and the stars.
- **The detail** is the pack's whole window in the chosen variant (`settings_themes_preview.go`): the tabs, a
  sidebar of sessions, a chat with its code block, the closing line, a terminal with all sixteen colors and the
  composer. The variants are a segmented control over the description; the one Primary is Install, or Use once
  installed. A pack written for a newer lich keeps Install disabled, the version it needs in the tooltip.
- **Try** paints the window in the variant and stores nothing: a bar on `Popover` at the window's foot says
  "Trying <name>" with Revert and Keep (Primary). Keep installs the pack if it must and chooses the theme;
  everything else gives the window its theme back: Revert, Escape, closing Settings, a selection from outside,
  the system's light and dark switching, and a Keep that failed.
- **Installed** opens on the theme in use, so nobody scrolls to find it: In use is one wide card on `Accent`,
  its cover larger, In use, the name, the pack and its version in mono, the theme it pairs with and Auto light/dark
  at its foot (`settings_themes_installed.go`). Under Other themes, every other theme is a card of the catalog's
  grid, one per theme (a theme, not a pack, is what is worn): its cover, its name, the pack and version in mono
  ("imported from a file" for one with no repository), Update while a newer version waits and the `⋯` holding
  Remove, which asks first. A click on a card wears the theme; its press sits under Update and the `⋯`. The imported themes come first, then System and
  the bundled ones; the themes that cannot load end the page. Check for updates stands at the right of the tabs'
  line while this tab shows, and the tab wears a dot while an update or a broken theme waits on it.

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

## The sidebar: a list of rows

The sidebar is the window's reference list; its geometry lives in `sidebar_card.go` and
`sidebar_tree_draw.go`. It is a list, not a tree of worktrees: the sessions filed nowhere come first, with no
header, then the folders, and inside each the sessions stand in their stored order, whatever checkout they run
in and whatever their status: a row never moves because its session changed phase.

### A row

Every row is one line, 34dp tall: `Space(1.5)` above and below a 22px node. Rows stand flush, no gap
(`cardGap` is 0).

- **A session row**: the provider's mark, the shield and fork glyphs it earns, its name, its branch in
  `TextXS` mono `MutedForeground` (at most `branchShare`, 35%, of the line, so the name keeps the room), and
  the corner slot. Neither the checkout's changes nor its pull request is on the line: the row's width belongs
  to the name.
- **A row's tooltip** keeps the branch on a line of its own, whole (it wraps, never cut), and under it the
  pull request and then the changes.
- **A pull request** (`prBadge`, in a row's tooltip and on the chat composer's line): the forge's logo at 12px in
  `MutedForeground` (`Foreground` on hover where it is a link), then the state's glyph at 12px and the number in
  the line's text size, both in the state's tone: open `git-pull-request` in `TonePass`, draft `git-pull-request-draft` muted, merged `git-merge` in
  `ToneMerged`, closed `git-pull-request-closed` in `Destructive`. A merged or closed one stays while the
  checkout is on its branch. The forge is the checkout's, from the window's forge store; while it is unknown
  no pull request is drawn, so the wrong logo never flashes.
- **The provider's mark** is its book's medallion in pixel art (`grimoire.Mark`), 1dp a pixel, in the
  provider's brand color (`markColors`) or `Foreground` for a logo in black and white; a provider with no
  book keeps its vector mark. Nothing is drawn around it or over it: no ring, no status.
- **A browser row**: a session's running browser, under it: the globe, the page's title as its name (its host
  while it has none), the corner slot. The name is `MutedForeground` while the page is off the stage. A
  folded mother shows a globe on her own row instead.
- **An editor row**: the file a session's checkout holds in the stage editor, its node the file's icon (`th.FileIcon`, as the Files tree), under the session working in
  that checkout (the active one, else the first drawn), after its browser row.
- **A session in a layout** carries the columns glyph before its corner slot in its project's sidebar. The
  layout's own sidebar draws none: every card there is in it.
- **A layout's sidebar**: the layout's name where New Session stands, then its sessions under each
  project's name in `TextXS` `MutedForeground`, in the same list of session, browser and editor rows, with their motion. No
  card there wears the active fill: the focused pane's header says which session has the keys, its name in
  `Foreground` and the others' in `MutedForeground`. A pane wears no frame for focus, only its 1px hairline.
- **A pane's motion**: a pane new to the stage grows out of the edge it was split on in 200ms ease-out, and
  one that leaves shrinks back into it in 180ms ease-in, the neighbours giving and taking the room over the
  same time. A zoom, a divider and a change of tab move at once.
- **A group header** (a folder, the pinned block) is a row too, the same 34dp, so its hover lights as a
  row's does: the glyph in the node's column, the uppercase label, its count, a rule to the `⋯`.

The corner slot (`cardSlotSize`, 22px, the mark's box) is always reserved, so nothing on the line moves when
it fills: the `⋯` under the pointer or while its menu is open; at rest the session's status, else a pin on a
pinned session, else nothing. The status is pixel art, 11x11 at `soulsSize` (16dp, under the mark's box), each
cell's edges on whole device pixels, each status in a palette of its own: while the session runs, three souls
diving to the center and back (`sidebar_spinner.go`, 20 frames of 70ms) in the theme's `Primary`, heads full and
tails fading over the row's surface; while it waits on you, an hourglass in gray wood whose `ToneWait` sand runs
down and that turns over; for a turn nobody read, a check in `TonePass` that asks for no frame, gone once the
turn is read. A status reached on screen from working plays out of the spinner when its souls meet in
the center: they become the hourglass, or an ember that stretches into the check (`sidebar_status_sprites.go`);
one read at startup is drawn at once. Idle shows nothing. Opening a session in the checkout a card runs in is the first entry of its menu.

### The node

- A child is indented `sessionTreeIndent` (16px) a generation, with no line to its mother.
- A node that folds (a mother, a group header) shows the fold's chevron in place of its glyph under the
  pointer; there is no separate disclosure column. A folded node keeps a count and the mark of what it hides:
  `ToneWait` for a session waiting, `TonePass` for an unread turn.

### The folder tray

A folder is the one nested surface: its header and rows sit on a tray, `RadiusLG`, so where a folder ends
reads against the rows after it. The tray is `Alpha(Accent, 0.45)`, or the
folder's color at `tintRest`; a card painted the folder's color paints nothing at rest on it. Rows run to
the tray's edges, clipped to its corners, so a lit row fills it to the end. A closed folder is its header
alone, no tray.

### The Services footer

The project's services sit at the sidebar's foot, above the profile, in a place of their own: the sessions
scroll over them and they never move when a session opens (`sidebar_services.go`). A `Border` hairline parts
them from the list, then a header drawn as a folder's is (the services glyph in the node's column, turning to
the fold's chevron under the pointer, SERVICES, how many run of how many, the rule to the `⋯`), which opens
and closes the rows under it. Its `⋯` and right click hold Start all, Stop all and New service. A service is a
row as a session is: its glyph where the mark sits, the name (`MutedForeground` while it is down), how long it
has run at the end or its exit code in `Destructive`, and the corner slot, the `⋯` under the pointer and the
souls spinner while it stops. No dot, and no button on the row: its actions are its menu; a click shows its
console. The open list scrolls past `serviceRowsShown` rows. Adding a service and editing one happen in a dialog over the screen
(`settings_service_dialog.go`), from the footer's New service, a row's Edit or Settings › Services alike,
whose rows open it on a click: the name in a narrow field beside the command, one line on how a command runs
with the rest in a help glyph's tooltip, Start with lich as a switch, then Delete for a saved service that is
down, Cancel and Save.

### The rail

The collapsed sidebar is an island like the open one (`sidebar_rail.go`): `Sidebar`, `RadiusLG`, the same `Border` hairline, its content
on the 8px edge around a column as wide as a row is tall (34), so it is 50px wide and every node centres
where it does in the open sidebar. Expand sits in a row of the open header's height
(36), where collapse sits in the open sidebar; new session, Services and the profile sit in its footer, so
the list starts where the open sidebar's does; blocks stand `Space(1.5)` apart as the open ones do, and every
node keeps its place across the morph. The list is
the open one in one column: rows of the same 34dp, a folder or the pinned block led by its glyph, named in a
tooltip, a child's mark drawn smaller, and a folder on its tray, with no line between the nodes. A folded
block or mother keeps only its node. A session's mark is lit by a 32px square, `Accent` while active. A card drags in the rail as
in the open sidebar, and the filter is the open one's field and chips in a panel off the search button in
the footer; it outlives the panel, and the button stays lit with a dot while it narrows the tree.

### The peek

A collapsed panel comes out when the pointer rests on it (`panel_peek.go`): the sidebar from its rail's tree,
the dock from the window's right gutter once its collapse button took it off the stage. The hover only decides
when it opens and closes: it runs the same expansion its button runs, with the same timing and curve, the
sidebar's morph from the rail and the dock's slide from the edge, and the same collapse back. It is drawn over
the stage, which keeps its width, so the terminal is not resized and nothing the agent printed rewraps; only
keeping the panel open, from its button or the hotkey, gives it the stage's room. Through the dock's slide
beside the stage, the chat follows the stage's edge on every frame and a terminal keeps its grid until the slide
lands, so the agent redraws once, after the motion. A page in the stage is a window of the system's that nothing
the window draws covers, so the panel's box each frame (`floatingBoxes`) is what the page will cut out of itself
once `internal/webview` can. The sidebar comes out after `peekOpenDelay` (150ms) of rest on the rail; the
dock comes out the moment the pointer touches the window's edge, even a flick that goes on past it within one frame,
since the edge is somewhere you throw the pointer, not something it crosses on its way. A pointer gone out of the
window across that edge still counts as on it until it comes back onto the stage. A panel goes back
`peekCloseDelay` (300ms) after the pointer left it, never while a menu, a field or a drag in it is in use. Only
the rail's tree brings the sidebar out: over expand or the footer's buttons it would cover the button on its way
to a click. The button that collapses a panel keeps it open from the peek, in place, with no morph or slide. The
dock never closes: it is docked or collapsed, and opens on the tab of lich's it was last on (`lich.dock.tab`,
Review until one is stored; an extension's tab is not stored, as its surface is placed only after the first
frames). A tab opened from a balloon brings a collapsed dock out until the pointer has been in it.

## Motion

Motion says where something came from and where it went. It never holds anything up: a session's pane opens
at once, whatever its row is still doing.

- **Rows move in and out of the list.** Coming in (a mother or a folder opens, a session appears), the row
  opens with its name already on it and the mark comes in after; leaving (a mother or a folder folds, a
  session closes) plays the other way. A folder's last session leaving fades the folder's header and closes
  its tray after it; a new folder plays that backwards. The rail moves the same way, in the same timings, its
  marks without the names. Timings live in `sidebar_tree_motion.go`. A folded folder keeps its tray until its
  rows have left.
- **A dragged card lands where the list keeps it.** While it is in the air its slot draws a dashed ring
  (`sidebar_drag_links.go`); a mother in the air folds over her children, and the slot only stops where the
  list will keep the card (`treeSnap`). A block the card would go to lights its whole tray, a border in its
  color, in the open sidebar and the rail alike.
- **Other lists** enter and leave with `ui.Presence`: 160ms in, 180ms out, from a 0.6 scale at the top
  edge. The chat's rows use `LayoutFlat`, the same timings with no scale (see The chat view). A fold is
  `ui.Collapse`, 200ms.
- **One motion per arrival.** An item another motion brings in is settled in `Presence`
  (`Presence.Settle`), or Presence plays its own entry once the first one hands the item back, and the row
  jumps.
- **A moving item takes no input**, but advances its tween before `gtx.Disabled()`: a disabled context drops
  the redraw request, and the motion freezes on its first frame.
- **A click shows in the frame that took it.** Gio draws one frame per event, so a click read after the
  thing it changes was laid out leaves the old state on screen until the mouse moves. Read the click before
  that layout, as `actGit` does for the Git panel, whose row comes before the balloon that opens it.
  A control that opens, closes or resizes something laid out elsewhere gets a `frameRig.click` test
  (`stale_*_test.go`), which fails on exactly this.

## Fixed by decision: do not "fix" these

- **The bundled themes stay achromatic.** A colored palette is a custom theme, not a widget's color.
- **The layout**: top tabs, left sidebar, terminal, right dock always there (docked or collapsed), the footer's
  balloons over the stage's foot, and the Git panel rising from the stage's foot across the terminal and the
  dock. An extension's panel takes the Git panel's place, one panel at a time, and its
  dock surface is a tab of the dock. It takes height, never width: fewer terminal rows repaint, while fewer columns
  would rewrap what the agent printed.
- **Pinned project tabs** stay at the left of the top strip, grouped before unpinned tabs with one hairline
  between the groups; they carry a pin glyph and close from their context menu only.
- **Semantic color and the diff language badges**: they carry meaning.
- **Buttons do not move when pressed**; only their color changes.
- **A sidebar row is one line.** Pull request and changes stay on it; a name too long is cut, not wrapped.
