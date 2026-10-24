---
name: x11-marketing-screenshots
description: >-
  Regenerate, add or change biscuittin's marketing screenshots: repeatable,
  fixed-size PNGs of the X11 window only, captured by the app itself on a
  private Xvfb display with fictional config, then indexed in
  var/marketing/screenshots/screenshot-manifest.yaml and shown in the README.
  The X11 counterpart of jwr-marketing-screenshots. Use when updating README
  screenshots, adding a screenshot of a new screen, after UI changes, or when
  the user mentions marketing screenshots, screenshot manifest or
  x11-marketing-screenshots.
---

# x11-marketing-screenshots

biscuittin is a raylib/OpenGL X11 app, not a web app, so browser tooling
(`repo-helper screenshot`) can't capture it. Instead the app is instrumented to
drive and capture itself:

- `-size WxH`: a window of exactly that size (implies windowed).
- `-script <file>`: runs steps (`clock`, `input`, `press`, `sleep`, `shot`;
  format in `desktop/internal/app/script.go`), then quits. `press` and `shot` wait until icons and
  the background have loaded and every animation has finished, so nothing
  depends on timing. `shot` saves the framebuffer before the buffer swap: only
  the window, no decorations, and it fails if the window isn't exactly `-size`.
- `backgrounds.providers: [local]` with `backgrounds.dir`: a committed photo
  instead of a random download.

Everything else lives in `var/marketing/screenshots/` (see its README). Run:

```sh
make screenshots
```

## Rules

1. **Only the X11 window, fixed size.** Captures come from `shot`, never from
   a desktop screenshot tool. Size comes from `width`/`height` in
   `screenshots.ini`, which sets both the Xvfb screen and `-size`. Never run
   it on the user's own display: their window manager (i3) tiles and resizes
   the window, and `shot` would fail.
2. **Repeatable.** Two runs must give pixel-identical PNGs. Check with
   `magick compare -metric AE a.png b.png null:` (expect `0`). Anything that
   varies (time, host name, random backgrounds, connected gamepads, your own
   `~/.local/state` settings) must be pinned by the config, the seed or a
   script step. If something still drifts, make the app settle exactly
   rather than adding `sleep`s; for example, tile zoom snaps to its target.
3. **Fictional, safe data.** Use `example.com` URLs and fake tokens, and never
   use the user's config, hosts or tokens. Never `select` a launcher entry
   (it would start the app) or open an integration list (it would contact
   the server). Don't capture Diagnostics or the QR code: they show the host
   name, paths and LAN address.
4. **Manifest and README stay current.** The manifest is generated (never
   hand-edited) from `screenshots.ini`; every section needs `title` and
   `description`. The README `## Screenshots` section has one `###` per
   manifest entry, in order, with the description as its caption. Update both
   whenever you add, rename or remove a section.

## Adding or changing a screenshot

1. Add a section to `screenshots.ini`. Sections run in order in one app
   session, and `steps` (`up down left right select back`) are presses after
   the previous capture. Work out tile indexes from `columns` in
   `config.yaml`: entries, then pinned actions from `settings.yaml`, then
   More. Use a section's `input = keyboard` / `playstation` for other hints.
2. If the screen needs state, change `config.yaml` or `settings.yaml`
   (fictional values). If the app can't be put in that state with config,
   settings or presses, add a script step in `desktop/internal/app/script.go`
   (with a test in `desktop/internal/app/script_test.go`) rather than special-casing the UI.
3. Run `make screenshots`, then **look at every PNG** with Read. Check for
   overlaps, blank tiles, letter tiles where an icon should be, and anything
   private.
4. Run it again and compare the PNGs to confirm they are identical.
5. Update the README section (rule 4) and run `make test`.

## Pitfalls

| Symptom | Cause / fix |
|---|---|
| `make` says Xvfb is missing | Ask the user to run `! sudo dnf install -y xorg-x11-server-Xvfb`. There is no fallback display. |
| "screen did not settle within 90s (icons or background image still loading)" | No network for thesvg/iconify, or a `local` background dir with no images. See `tmp/biscuittin.log`. |
| A press seems to be ignored | The first press after `back` on the home screen (tiles hidden) only wakes the screen. Add a harmless wake press, such as `up` on the top row. |
| Second row of tiles overlaps the footer hints | 9+ tiles in 5 columns don't fit at 1080p, which is a real layout bug. `columns: 6` avoids it here; report it rather than hiding it. |
| A tile is blank (no icon and no letter) | The SVG rasterised to a fully transparent PNG (seen with thesvg's Firefox). Check with `magick <png> -format '%[fx:mean.a]' info:`, choose another entry, and report it. |
| Letter instead of an icon | thesvg has no exact match for the title (e.g. Moonlight). Set `iconQuery` or pick another entry. |
| `Display :97 is in use` | `make XDISPLAY=:123`, or `make -C var/marketing/screenshots down` after an interrupted run. |

## Reporting back

Tell the user the regenerate command, which PNGs changed, that the manifest
and README were updated, whether two runs were identical, and any UI bugs the
images revealed (don't silently fix the UI to make images look better unless
asked).
