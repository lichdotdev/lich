---
name: lich-ux-declutter
description: Cut a native window screen down to what a visit needs, from a screenshot of it. Use before calling any UI change done in `native/`, after adding or moving a control, a label, a row or a dialog, and whenever you look at a shot of your own UI.
---

# Declutter a screen

`native/DESIGN.md` says how to draw. This skill says what to cut. A screen that works can still be noisy: every
capability visible at once, one fact said twice, several colors competing, edit controls where someone only came
to read. Run the pass on the shot, never on the code; code review does not show noise.

## 1. Shoot it

Take a `-shot` of the screen in its common state, the one most visits land on, at the window's default size.
`.claude/skills/lich-build-dev/` says how to take it without touching the user's lich. Look at it as someone who
opened the screen to do one thing, and name that thing.

## 2. Inventory

List every visible word, control, color and container, top to bottom. Put each one through four questions, in
this order, and stop at the first yes:

1. **Remove?** Does it earn its place on most visits? Delete copy that restates a label, a heading that repeats
   the values under it, a status that is obvious.
2. **Merge?** Is the same fact shown twice (a title and its field, a group label and its rows, a summary and
   the controls that set it)? Keep one.
3. **Disclose later?** Is it needed only sometimes? Move it down the ladder DESIGN.md already has: the tooltip
   (what a row left off its line), the row's menu, the `⋯` menu, a dialog or its own screen. Rare and
   destructive actions never repeat per row.
4. **Shrink?** Does an icon with a tooltip say it as well as a word? Can a value sit at the end of its row
   instead of taking a row?

Every element ends with a verdict. An item you cannot justify to a stranger who opened the screen once is a
remove.

## 3. Order by use

The control touched most goes first, then what the visit came to read, then secondary facts, and the way into
configuration last. A control the state calls for stays in reach (DESIGN.md › What goes where); do not demote
it to disclose.

## 4. Count the colors

Count the distinct colors in the shot. Ink, the grays, the `Accent` fill of hover and selection and the one
`Primary` button of the view are the whole budget; a `Tone*` color appears only where it reports state. More is
noise. DESIGN.md owns the tokens and what each is for; check against it, do not restate it here.

## 5. Reshoot and compare

Take the shot again and set the two side by side. Count elements and colors before and after, and say both
numbers in your report. Compare with a sibling screen at the same density when one exists. Stop when nothing
more can go without losing what the screen is for.

## Not done while

- A view screen offers an edit control on every field.
- The header's title is also in the body.
- A group's label says what its rows already say.
- Each row carries its own Delete or Remove button at rest.
- More than one `Primary` button is on screen.
- A history or log list is expanded inline where a link to it would do.
- Copy explains a control whose label already says it.
- A line of copy was added and neither removed nor shown only when relevant (the PR must say why, `CLAUDE.md`).

What DESIGN.md › Fixed by decision settles is not a candidate for the cut: the layout, the one-line sidebar row,
the semantic color. Do not reopen it here.
