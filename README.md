# ostyle — Office palette changer

A lightweight CLI that restyles **Microsoft Word** and **PowerPoint** documents
on macOS: color palette, fonts, Word heading styles, and PowerPoint deck
background — from your terminal, in about a second.

Themes are plain `.css` files, so they are diffable, shareable, and live in
one small directory. The tool is a single static Go binary with **zero
third-party dependencies**.

```
$ ostyle apply nord --active --app powerpoint
styling deck.pptx with theme "nord" (built-in)
✓ /Users/you/deck.pptx: theme applied (powerpoint, 1 part(s) patched, backup at /Users/you/deck.pptx.bak)
```

## How it works

`.docx`/`.pptx` files are zip packages whose theme lives in
`word/theme/theme1.xml` / `ppt/theme/theme1.xml`. ostyle rewrites the
`clrScheme` (the 12 standard Office palette slots) and the `fontScheme`
latin typefaces, and patches `word/styles.xml` so defaults, Normal, and
Heading 1–6 follow the theme fonts (plus your explicit heading overrides).
For decks it can also set the slide-master background. Everything else in the
package is copied through untouched — `[Content_Types].xml`, relationships,
and the format scheme (which is palette-driven in standard themes and
restyles itself).

There is no supported AppleScript command to apply a theme to a live
document, so for open documents ostyle drives the app just enough:
**save → patch the file on disk → close → reopen**. That round-trip takes
a second or two and is the entire "live" workflow.

## Install

From a clone (needs Go 1.22+):

```sh
git clone https://github.com/ansonzhang/office-palette-changer
cd office-palette-changer
go build -o /usr/local/bin/ostyle ./cmd/ostyle
```

Homebrew (once published — formula in `Formula/ostyle.rb`):

```sh
brew tap ansonzhang/office-palette-changer
brew install ostyle
```

## Usage

```
ostyle list                          list themes (built-ins + your dir)
ostyle show <theme>                  palette swatches, fonts, heading table
ostyle apply <theme> <file...>       patch closed .docx/.pptx files
ostyle apply <theme> --active --app word|powerpoint
                                     style the document open right now
ostyle new <name> [--from <file>]    scaffold a theme (or extract one)
ostyle export <theme> [out.thmx]     real .thmx for Office's theme gallery
ostyle validate <theme.css>          lint a theme
```

Useful flags on `apply`: `--no-backup`, `--no-reopen`, `--force`
(patch even if the file looks open in Word/PowerPoint).

Your themes live in `~/.config/ostyle/themes/` (override with
`$OSTYLE_THEME_DIR`). Built-ins: `paper`, `nord`, `midnight`, `warm`.

## Theme format

A CSS subset: a `:root` rule with the 12 palette slots plus fonts, an
optional `@word` block (heading overrides and Word-only palette tweaks),
and an optional `@ppt` block (deck background).

```css
:root {
  --text: #2E3440;          /* dk1  (aliases: --dk1, --text)      */
  --background: #ECEFF4;    /* lt1  (aliases: --lt1, --background)*/
  --dk2: #4C566A;  --lt2: #E5E9F0;
  --accent1: #81A1C1; /* … --accent6 */
  --hlink: #88C0D0;  --folhlink: #8FBCBB;
  --heading-font: "Avenir Next";   /* major font  */
  --body-font: "Helvetica Neue";   /* minor font  */
}
@word {
  h1 { size: 20pt; bold: true; color: var(--accent1); space-before: 18pt; }
  h2 { size: 15pt; bold: true; }
  /* :root { --text: #111; } — Word-only palette overrides also work */
}
@ppt {
  background: var(--lt1);   /* hex also works */
}
```

- Heading properties: `size`, `bold`, `italic`, `color`, `space-before`,
  `space-after`. `color` accepts `var(--accent1)` style references.
- Missing slots are derived (accents interpolate between text/background),
  so a minimal theme of `--text`, `--background`, and a few accents is fine.
- `ostyle new mytheme --from somefile.docx` reverse-engineers a theme from
  any existing document — handy for stealing a corporate palette.

## macOS permissions (one-time)

- **Automation**: the first `--active` run makes macOS ask
  "… wants to control Microsoft Word/PowerPoint" — click allow.
- **Folder access (授予访问权限)**: sandboxed Office apps need one-time
  access per folder. If an open/reopen fails with a permission error,
  ostyle prints a hint; approve the dialog or grant it under
  System Settings → Privacy & Security → Files and Folders.
- Headless `apply <file>` needs neither.

## Notes & limitations

- Content with **hard-coded colors** (text explicitly colored RGB, shapes
  filled with custom colors) does not change — same as applying a theme in
  the Office UI. Documents that stick to theme styles restyle fully.
- Legacy `.doc`/`.ppt` binaries are rejected; re-save as `.docx`/`.pptx`.
- Word style IDs are expected to be the canonical English ones
  (`Heading1`…`Heading6`), which Word-authored documents use; missing
  heading styles are created on demand.
- Cloud documents (SharePoint/OneDrive pseudo-paths) can't be patched
  in place — save a local copy first.
- `.bak` backups are written next to the file before every patch
  (`--no-backup` to skip).

## Security notes

- Zero third-party dependencies; stdlib only.
- All XML injection points (font names, theme names) are attribute-escaped.
- AppleScript arguments are passed via `on run argv`, never string
  interpolation — file paths cannot inject script code.
- Packages are rewritten entry-by-entry (never extracted to disk), so
  hostile entry names can't traverse the filesystem.
- Patches go to a temp file in the same directory and are atomically
  renamed; the original is never left half-written, and a `.bak` copy
  always exists until the next apply.

## Development

```sh
go test ./...      # unit + fixture round-trip tests
go vet ./...
```

Layout: `internal/css` (CSS-subset parser) → `internal/theme` (theme model)
→ `internal/ooxml` (surgical XML patchers + zip rewrite), plus
`internal/thmx` (.thmx export), `internal/applescript` (live-document
glue), `internal/cli`, and `themes/` (built-ins, embedded).
