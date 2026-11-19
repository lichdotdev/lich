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
  window's foot as the sidebar does. Each carries a 1px hairline in `Border`
  just inside its `RadiusLG` edge (`ui.InsetBorder`, `ui.IslandBorder`): the one outline in the window that
  wraps a surface rather than a field. The active project tab that has grown out of its island takes the
  line with it, up its sides and round its fillets, and the island's top edge is left open under it
  (`topbar.tabGap`). The top bar sits on the canvas itself, unboxed; the footer is two balloons over the
  stage (see below).
- **Hairlines are seams inside an island, and its edge.** `th.Separator` splits a panel's own parts (a screen's header
  from its body, two panes, a folder header from its rule). It never wraps a list item, a badge or a button
  at rest. Inputs are the one exception: a field needs an edge to read as editable.
- **Badges are text and glyph**, never a filled pill: a pull request is its icon and `#N`, a diff is `+a -d`
  in the tones.
- **State is fill, not chrome.** Selection is a flat `Accent` fill: no side bar, no ring, no shadow.
- **What does not fit the line goes to the tooltip.** A row carries what you scan for; the rest (path,
  model, the tool a turn runs, how long) is one hover away, never a second line.

Inside an island, a change that adds a border, a pill, a second line or a nested surface is probably wrong. Reach for spacing,
a hover state or the tooltip first. The folder tray is the one nested surface, and the reason is written
under it; the dock's tab pill reuses its fill, for the same reason. The footer's balloons are the pills on `Sidebar`:
they float over the terminal and need a ground of their own to read on it. The extension catalog is
the one screen of cards, and the reason is written under it.

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
balloons stand at the foot of the stage, reading the focused pane's session, and every pane stands above the
band, which the layout keeps however the focus moves.

A plain terminal (a `shell` session), a service's console and Home carry no balloons and no band: the terminal
takes the island's full height. A session drawn as a chat carries none either: its footer stands in the
composer (see The chat view), and the chat takes the band back. In a layout the footer follows the focus, so
the balloons give way while a chat pane is focused and return when a terminal is. Settings, Pulls and an
extension's screen cover the stage and carry none either. A project with no session keeps them, since its branch is the click into the Git panel.

The Git panel and an extension's panel rise from the stage's foot over the band: while one is up the balloons
are not drawn and the terminal takes the band back, in the one resize the panel's slide already makes.

### The context tooltip

The context ring's tooltip is one component for the balloon and the chat's composer (`footer_context.go`): a
card with "Context" at the left and the share used at the right in medium, a bar of twenty rounded segments
(`SegmentedBar`) whose lit ones are `Foreground` up to 70%, `ToneWait` up to 90% and `Destructive` past it, then
lines of a muted label at the left and its value at the right: Tokens (compact, "585.7K", the window muted after
it), Model window, Cache with its hit rate and Session cost, each only when the report has it. Numbers are
compact with the locale's decimal mark. There is no line for compactions or for what is left before one: lich
keeps no count of the first and no provider tells it the threshold of the second.

### The usage tooltip

The tooltip over the profile at the sidebar's foot, and the plan quota's own in the footer, are one component
(`usage_tip.go`) built from the context tooltip's pieces (`tipHead`, `tipRow`, `contextBarColor`, the bar of twenty
segments, the width). A block per window, in the order session, weekly, then a model's own cap: the window's name
(lich's own for a window it names, with its length: "Session · 5h") at the left and the share spent at the right in
medium, the bar in the same tones as the context's (70%, 90%), then muted `Resets in 4h 12m` and, only when the
pace so far empties a weekly window before it resets, `Runs out in 1d 9h` in `ToneWait`. A locked window reads
"Locked" with its bar and the provider's reason in `Destructive`. After the windows, `Extra usage` with the amount
against the cap at the right ("$12.50 of $50.00", "$141.57 · no limit") and a bar only when there is a cap, and a
`Reset credits` line like a context row ("1 available"), each only when the account has them. Claude Code's limit
resets come as a `Resets` block, its title with "N left" (every reset of every live grant) at the right and a line per
grant: its name at the left ("Full" when it clears the weekly window, the model's name for a model's own cap, the
provider's text otherwise), "×2" and a muted "until Dec 10" at the right, in the interface language's month. A grant
that cannot be spent now (paused, or not usable) is muted whole; with no live grant the block is not drawn. It shows
and never spends: using one cannot be undone. The account's email
closes the plan quota's tooltip, under a separator. The profile tooltip heads the blocks with the plan ("Claude Code ·
Max 5x"); for any provider but Claude Code and Codex it is a single muted line saying usage is not measured per profile, and while the first reading is in flight there
is no tooltip at all, rather than one that says nothing. Amounts follow the locale (`money.go`): its separators, the
symbol before or after, the sign outside the symbol, the ISO code where there is no symbol.

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

- **The person's message** is a box in `Accent`, `RadiusLG`, against the column's right edge, at most 78% of
  it. It breaks "rows, not cards" on purpose: a turn starts where a message does, and the eye finds it by the
  box faster than by a rule.
- **The agent's text** is Markdown at the prose size (`ui/markdown`). A link is underlined, in its color; a link to github.com, a path
  in code (a folder when it ends in a slash, else a file) wears a 13px mark before it, GitHub's, a folder's or a file's
  (`decor.go`). Anything else in code is plain. The first answer of each turn opens
  with who speaks: the provider's mark at 20px on a `RadiusMD` square, in the session's card color (the
  agent cursor's color, `Primary` for a card with none; the window keeps no brand color per provider) over
  a fill of that color mixed 22% into the surface, then the provider's name in medium, then muted
  "· model · effort" as the answer's own item says them (`ChatItem.Model`, `Effort`: what wrote it, not what the
  chat is set to now; the options are never read for it, so changing the model leaves old answers as they were; an
  empty effort leaves it out, and with neither the header is the name alone). A thought is a muted fold, closed, with the
  sparkles before its label.
- **A turn's tool calls** fold into one muted line where the first of them came, with a 1px `Border` hairline
  running from its label to the column's edge. When the line opens the turn (no answer before it) it starts with
  the provider's avatar, and the answer's header that follows leaves the avatar out, keeping name and model. Finished, it reads
  "Worked for 41s · 6 tools · 09:15": how long the turn took, how many calls, when it ended (without the time
  when the history has none). Open, each call is a 28px row hung under the line by a connector, a 1px
  `Border` line under the fold's chevron with a stub into the row, ending at the last row as the sidebar's
  tree does. A row is its tool's glyph (a Lucide mark by tool name, Claude Code's own names and ACP's kinds,
  one generic wrench for the rest), its name in medium, what it acted on in mono, and how it ended at the
  right: a spinner in `ToneWait` while it runs, then a check in `TonePass` or a cross in `Destructive`
  with the output's first line. Its diff stands under it as a thread's hunk is drawn.
- **A turn's closing line** stands once, after the last item of the conversation's last turn, when that turn
  has ended: the provider's avatar as the tool-call line opens a turn with, "Done" in `TextSM` medium
  `Foreground`, then muted "· no changes" or "· 3 files" and the lines the turn added in `TonePass` and removed
  in `Destructive` ("+12 −4"), as the diffs draw them, and a 1px `Border` hairline to the column's edge.
  "Stopped" stands for a turn the person cancelled, and "Failed" in `Destructive` for one that ended in an
  error. The files and lines are the click: they open Review as the footer's changes item does. The line
  leaves when the person's next message begins a turn, and none stands while a turn runs, where the live line
  is; older turns have none, in a history read back too. A turn whose checkout lich could not read (not a
  repository, or a commit made during the turn) says its word alone, and so does one read back with the
  history, whose end nobody reported to the window: the line never invents a count.
- **What is alive moves.** The text of a live step shimmers: a light band crosses it from left to right once
  every 1.6s (`Label.Shimmer`, looping frames only while it is drawn), and the time it has run counts beside
  it, "6s", "1m 12s", redrawn each second and not each frame. That is "Thinking" for the thought being
  thought, "2 tool calls" for the turn's line while a call runs, and the name of the call that runs. The
  answer being written ends in a caret that blinks every half second. A row new to the conversation enters
  with `ui.Presence`, growing in from the top; the conversation read when the pane opens, or after it, does
  not play as arrivals. A list at its end stays at its end while rows grow in.
- **A permission** is a box on `Sidebar`, `RadiusMD`, `Space(3)` padding (see Boxes inside a screen): the tool,
  its diff, and a `SizeSM` button per answer the provider offers, the first allow filled. Enter takes the first
  allow and Esc the first reject. Answered, it shrinks to a muted line naming the answer.
- **A notice** is a muted line; the turn in flight ends the column with a pixel flame (`chat_flame.go`: a 9x11 sprite of 1.5dp pixels, drawn as rectangles, in 8 frames of 120ms; a window without focus looks in at `StillFrame` instead) and a verb of the turn, shimmering like "Thinking" with the time it has run beside it; the verb is picked from the language's own list (`chat.view.verbs`, `|`-separated, each language writes its own length) by the session and the turn, so it holds for the whole turn; left out
  while a permission waits: its box already says so.
  The flame's colors are tokens, never a fixed hue: the core is `Foreground`, the body a mix of `MutedForeground` toward
  `Foreground` (0.55), the edge `MutedForeground`, and the base and the sparks leaving it `Accent`. A gray theme
  draws a gray flame and a colored one colors it on its own. It does not use `ToneWait`: the turn running is not the
  turn waiting on the person, and that tone already says the second.
- **The composer** stands at the column's foot: a textarea (Enter sends, Shift+Enter breaks the line) in a
  `RadiusLG` box, three lines of text tall at least, growing with the text to a ceiling and scrolling past it.
  Its text and its placeholder, "Ask {provider} about this project", are in the interface font at the body size
  and leading of the messages (15sp at 1.5), `Space(4)` in from the left and `Space(3.5)` down from the top.
  Inside the box stand only the provider's controls, one line under the text: the footer's attach as a "+",
  the model with its effort in one chip ("Opus 5.5 · high") and the mode, each a ghost chip with a chevron.
  After them the footer's context ring with its percentage, the same readout as the terminal's balloon, when the
  session reports a window.
  The mode's menu lists the provider's choices, the current one checked, and picking one sets it
  (`ChatSetOption`). The model chip opens one panel upward, in two columns with a hairline between
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
  the box, outside it with no fill, one line: at the left a `TextXS` muted project, a slash and the branch,
  which opens the Git panel as its balloon did, and at the right the footer's changes item (files and diff),
  which opens Review. Attach, changes and the branch show only when the user's footer layout has them, and
  only for the session the footer follows; another chat pane of a layout has the provider's chips and Send alone.
- **Copy** is an icon-only button that shows only while the pointer is over a message, on the line of its time and
  beside it: at the left of the time for the person's message, so it sits at the right, at the right of the time
  for an answer, so it sits at the left. It copies the item's whole text, an answer's Markdown source, through the
  window's clipboard path, and becomes a check for 1.5s. It has no tooltip: the glyph says it. The line is as tall with it as
  without (`chatCopySlot`), so it pushes nothing; the button is a little larger than the line and reaches past it.
- **A message's time** is a `TextXS` muted line, the window's short clock ("14:32") under a tooltip that holds
  the footer clock's full date: under the person's box at the right edge, always, led by its day when that is
  not today ("Nov 17 · 23:05", the year too past this one); under an answer's text at the
  left, its foot, only while the pointer is over the answer (with the copy button beside it; the line is there at
  rest, empty, so nothing moves).
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

A split's pane has its header already, so it gets no strip: the selector, same size, stands at the right of the
header between the name and the zoom button, and the pane's terminal keeps every row.

A terminal that does not have the keyboard (a split's other panes, any pane while a field or dialog is typing, every
terminal while the window is out of focus) draws its cursor as a 1px outline in the cursor's color; the one that has it
draws the block.

### The session's browser

A page pane (`browser_pane.go`, `browser_tabs.go`) is three rows on the pane's fill, no seam between them.

- **Tabs** on the top row, the page's tabs always, even one, then the new-tab button, and the pane's expand,
  minimize and close at the right. A tab is a neutral 12px square where a favicon would go (the backend sends
  none) and the page's title in `TextXS`; the front tab is filled `Accent`, its title `Foreground` in the focused
  pane and `MutedForeground` in the others. A tab behind has no fill until hovered. There is no pane title and
  no globe.
- **The toolbar** is back, forward and reload as icons in `MutedForeground`, the address, then the zoom (its
  percentage shows only off 100%) and DevTools. The address is a pill, `Accent` fill, `RadiusMD`, no border, in
  the interface font: at rest the page's host and path with no scheme (`shortURL`), while edited the whole URL,
  selected; on a blank page the placeholder. What a button says lives in its tooltip: the Select tool's is
  "Click an element to leave a note". Open in a window is not here: it is an item of the browser row's menu in
  the sidebar.
- **The page** is a card inset `Space(2)` from the pane's left, right and bottom, `RadiusLG`, its edge the 1px
  `Border` hairline. The notes bar floats over it, as before.

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
  no edits, and the context menu that hands lines to a session is left out. Its node in the tree is the file's name
  like any other, closed from the same menu.
- In the tree the editor is a node of its checkout, the last row under it: the file glyph in a neutral ring and
  the file's name in mono, the unsaved dot after the name, lit as an active session is while the editor is on the
  stage. Its only action is Close, in its menu and its `⋯`; there is no ✕ on the row. In the rail it is the glyph
  in the card's 32px square, the path and the unsaved state in its tooltip.

### Toasts

The window has one toast stack, in the bottom-right corner: the `gutter` over the right balloon and ending
at its right edge, or over the stage's foot at the window's edge where there are none (`layoutToasts`). It grows upward, and a toast comes in and leaves by the bottom edge. A
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

### The extension catalog: cards and a detail dialog

Settings › Extensions draws the catalog as the site's extensions page does: a grid of cards, as many to a
line as keep each 260 to 360px wide (two in the pane at the window's default size; a wider pane takes columns,
never a bigger cover),
each a `Card` fill at `RadiusLG` with a 16:10 cover, and a click opens the listing in a dialog instead of
under its card. Both break rules above on purpose. The catalog is a shop window: the cover and the
screenshots are what someone chooses by, a row has no room for a picture, and an inline detail would push
the rest of the grid out of sight while it is read. The site and the window drawing it the same way also
teaches it once. Keep the exception to the catalog: a list of things you have, the installed extensions
on the tab beside it included, stays rows, and an action anchored on a row still opens beside it.

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
row on the Installed tab.

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
  card there wears the active fill: the focused pane's header says which session has the keys, its name in
  `Foreground` and the others' in `MutedForeground`. A pane wears no frame for focus, only its 1px hairline.
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

The collapsed sidebar is an island like the open one (`sidebar_rail.go`): `Sidebar`, `RadiusLG`, the same `Border` hairline, its content
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

### The peek

A collapsed panel comes out over the stage when the pointer rests on it (`panel_peek.go`): the sidebar from its
rail's tree, the dock from the window's right gutter once its collapse button took it off the stage. It is the
open panel, drawn over the terminal at the place it stands when open, so the terminal keeps its width and
nothing the agent printed rewraps. The sidebar comes out after `peekOpenDelay` (150ms) of rest on the rail; the
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
