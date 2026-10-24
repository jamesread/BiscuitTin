# Marketing screenshots

Repeatable screenshots of the BiscuitTin X11 window, used in the top-level
README and listed in [`screenshot-manifest.yaml`](screenshot-manifest.yaml)
for indexers.

```sh
make screenshots                   # from the repo root, or `make` here
```

## How it works

1. Builds BiscuitTin into `tmp/`.
2. Starts a private Xvfb display (`:97`, override with `XDISPLAY=:123`)
   exactly `width`x`height` from `screenshots.ini`. There is no window
   manager, so nothing resizes or decorates the window.
3. Runs BiscuitTin with `-size WxH -script tmp/steps`, using
   [`config.yaml`](config.yaml) and seeding [`settings.yaml`](settings.yaml)
   into a throwaway `XDG_STATE_HOME`. `generate-script.py` turns
   `screenshots.ini` into the steps. BiscuitTin presses the buttons, waits for
   icons, the background and every animation to settle, saves each PNG from
   its own framebuffer (only the window, always exactly WxH) and quits.
4. `generate-manifest.py` writes `screenshot-manifest.yaml`.
5. Stops Xvfb, even if a step failed.

Runs are pixel-identical: the clock is fixed, the background is the committed
[`seed/backgrounds`](seed/backgrounds) photo, and gamepad hints are pinned to
Xbox glyphs whatever is plugged in.

## Requirements

- Go, gcc and X11/GL headers (as for `make build`)
- `Xvfb` (`dnf install xorg-x11-server-Xvfb` / `apt install xvfb`)
- Noto Sans Bold at `/usr/share/fonts/google-noto/NotoSans-Bold.ttf`
- Network access to thesvg.org and api.iconify.design for tile icons

## Changing the screenshots

Edit [`screenshots.ini`](screenshots.ini): one section per PNG, in order, in
one app session. `steps` are presses after the previous capture. Every section
needs `title` and `description`; the README captions are the descriptions.
Then run `make` and look at every PNG.

The data is fictional: the integration URLs are `example.com`, the token is
fake, and no entry is ever launched. Don't capture Diagnostics or the web
remote QR code: they show the host name, local paths and LAN address.
