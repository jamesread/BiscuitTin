// Package app is the launcher itself: the window, tiles, menus, input
// handling and the main loop that ties the other packages together.
package app

import (
	"fmt"
	"image"
	"image/color"
	"sync/atomic"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/jamesread/biscuittin/internal/backgrounds"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/debugctl"
	"github.com/jamesread/biscuittin/internal/icons"
	"github.com/jamesread/biscuittin/internal/integrations"
	"github.com/jamesread/biscuittin/internal/launch"
	"github.com/jamesread/biscuittin/internal/rss"
	"github.com/jamesread/biscuittin/internal/settings"
	"github.com/jamesread/biscuittin/internal/webcontrol"
	"go.uber.org/zap"
)

var (
	colorOK  = color.RGBA{130, 220, 140, 255}
	colorBad = color.RGBA{255, 110, 100, 255}
)

// Launched commands that fail sooner than this are treated as launch failures
// and bring the launcher straight back with an error.
const quickFailure = 3 * time.Second

// actionListResult is an integration's list of actions, fetched in the
// background for More > Integrations.
type actionListResult struct {
	source  string
	actions []integrations.ListedAction
	err     error
}

// Options are the command-line choices for a run.
type Options struct {
	ConfigPath string // shown in Diagnostics
	Windowed   bool
	// Size is an exact window size, which implies Windowed. Zero means
	// 1280x720 when windowed.
	Size       image.Point
	ScriptPath string // steps to run, then return (see script.go)
}

// Run opens the launcher window and runs it until the user quits or the
// script finishes.
func Run(cfg *config.Config, opts Options) error {
	var err error
	size := opts.Size
	if size == (image.Point{}) {
		size = image.Point{X: 1280, Y: 720}
	}
	var sc *script
	if opts.ScriptPath != "" {
		if sc, err = loadScript(opts.ScriptPath); err != nil {
			return err
		}
	}
	fullscreen := config.BoolOr(cfg.Fullscreen, true) && !opts.Windowed && opts.Size == (image.Point{})
	hideOnLaunch := config.BoolOr(cfg.HideOnLaunch, true)

	rl.SetTraceLogLevel(rl.LogWarning)
	flags := uint32(rl.FlagVsyncHint | rl.FlagMsaa4xHint)
	if !fullscreen {
		flags |= rl.FlagWindowResizable
	}
	rl.SetConfigFlags(flags)
	rl.InitWindow(int32(size.X), int32(size.Y), cfg.Title)
	if fullscreen {
		// Borderless full screen at the monitor's native mode: no video mode
		// switch, and GLFW sets _NET_WM_STATE_FULLSCREEN, which i3 honours.
		m := rl.GetCurrentMonitor()
		rl.SetWindowSize(rl.GetMonitorWidth(m), rl.GetMonitorHeight(m))
		rl.ToggleBorderlessWindowed()
		rl.HideCursor()
	}
	defer rl.CloseWindow()

	rl.SetExitKey(0)
	rl.SetTargetFPS(60)

	settings := settings.Load(cfg)
	u := newUI(cfg, settings)
	u.configPath = opts.ConfigPath
	u.wantSize = opts.Size
	defer u.unload()

	u.rss = rss.NewFetcher(cfg)
	bg := backgrounds.NewFetcher(cfg)
	bgNext := 0.0 // GetTime when the next image is due
	bgPending := false
	bgShownAt := 0.0
	if settings.BackgroundsEnabled {
		if img, ok := backgrounds.LoadRecent(cfg.CacheDir); ok {
			u.setBackground(img.Pix, img.Grid, img.Credit)
			bgShownAt = rl.GetTime()
		}
	}

	iconResults := make(chan icons.Result, 32)
	// iconsPending counts lookups still running, so a scripted shot can wait
	// for every icon.
	var iconsPending atomic.Int32
	goIcon := func(f func()) {
		iconsPending.Add(1)
		go func() {
			defer iconsPending.Add(-1)
			f()
		}()
	}
	goIcon(func() { icons.Resolve(cfg, entryKey, iconResults) })

	// Integrations (OliveTin, Home Assistant): pin lists and the results of
	// actions started from tiles arrive here.
	actionLists := make(chan actionListResult, 4)
	actionEvents := make(chan integrations.Event, 16)
	iconRequested := map[string]bool{}
	requestActionIcons := func() {
		for _, t := range u.tiles {
			if t.kind == tileAction && t.action.Icon != "" && !iconRequested[t.key] {
				iconRequested[t.key] = true
				goIcon(func() { icons.ResolveIconify(cfg, t.key, t.action.Icon, iconResults) })
			}
		}
	}
	requestActionIcons()

	in := &input{}
	u.input = in

	bridge := webcontrol.NewBridge()
	var web *webcontrol.Server
	webURL := ""
	defer func() {
		if web != nil {
			web.Stop()
		}
	}()
	debugBridge := debugctl.NewBridge()
	debugSrv, debugErr := debugctl.Start("", debugBridge)
	if debugErr != nil {
		lg().Warn("debug socket", zap.Error(debugErr))
	} else {
		defer debugSrv.Stop()
		u.debugSocket = debugSrv.Path()
	}
	exits := make(chan launch.Exit, 16)
	started := map[int]time.Time{}

	hidden := false

	idleFade := 20.0
	if cfg.IdleFadeSeconds != nil {
		idleFade = *cfg.IdleFadeSeconds
	}
	lastInput := rl.GetTime()
	forcedIdle := false
	hideAt := 0.0
	waitingFor := -1

	show := func() {
		rl.ClearWindowState(rl.FlagWindowHidden)
		if fullscreen && !rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
			rl.ToggleBorderlessWindowed()
		}
		rl.SetWindowFocused()
		hidden = false
		waitingFor = -1
		// Reset key repeat and Guide-hold state, keeping the primary device.
		*in = input{primary: in.primary, pad: in.pad, usedInput: in.usedInput, padOverride: in.padOverride}
		lastInput = rl.GetTime()
		lg().Info("launcher shown")
	}

	lg().Info("started", zap.Int("entries", len(cfg.Entries)), zap.String("config", opts.ConfigPath))

	for !rl.WindowShouldClose() {
		select {
		case r := <-iconResults:
			u.setIcon(r.Key, r.Path)
		case l := <-actionLists:
			lst := u.list(l.source)
			lst.loading = false
			if l.err != nil {
				lg().Warn("action list failed", zap.String("source", l.source), zap.Error(l.err))
				lst.err = integrations.ShortErr(l.err)
			} else {
				lst.actions = l.actions
			}
		case ev := <-actionEvents:
			if ev.Done {
				u.actionsRunning[ev.Key]--
			}
			if ev.Msg != "" {
				c := colorOK
				if ev.Failed {
					c = colorBad
				}
				u.setToast(ev.Msg, c, 5)
			}
		default:
		}

		select {
		case img := <-bg.Out:
			bgPending = false
			if img.Err != nil {
				lg().Warn("no background image", zap.Error(img.Err))
				bgNext = rl.GetTime() + 60
			} else if settings.BackgroundsEnabled {
				u.setBackground(img.Pix, img.Grid, img.Credit)
				bgShownAt = rl.GetTime()
				bgNext = bgShownAt + float64(settings.BackgroundInterval)
			}
		default:
		}

		for _, src := range u.sources {
			if l := u.list(src.ID()); l.requested {
				l.requested = false
				go func(src integrations.Source) {
					acts, err := src.List()
					actionLists <- actionListResult{source: src.ID(), actions: acts, err: err}
				}(src)
			}
		}

		if u.settingsChanged {
			u.settingsChanged = false
			requestActionIcons()
			if err := settings.Save(); err != nil {
				lg().Error("save settings", zap.Error(err))
			}
			if !settings.BackgroundsEnabled {
				u.clearBackground()
				bgShownAt = 0
			} else if bgShownAt == 0 {
				if img, ok := backgrounds.LoadRecent(cfg.CacheDir); ok {
					u.setBackground(img.Pix, img.Grid, img.Credit)
					bgShownAt = rl.GetTime()
				}
				bgNext = 0
			} else {
				bgNext = bgShownAt + float64(settings.BackgroundInterval)
			}
		}

		if !hidden && settings.BackgroundsEnabled && !bgPending && rl.GetTime() >= bgNext {
			bgPending = true
			bg.Request(rl.GetScreenWidth(), rl.GetScreenHeight())
		}

		select {
		case ev := <-exits:
			e := cfg.Entries[ev.Index]
			u.running[ev.Index]--
			ranFor := time.Since(started[ev.PID])
			delete(started, ev.PID)
			lg().Info("app exited", zap.String("title", e.Title), zap.Int("pid", ev.PID), zap.Duration("ranFor", ranFor.Round(time.Millisecond)), zap.Error(ev.Err))

			if ev.Err != nil && ranFor < quickFailure {
				u.setToast(describeExit(e, ev.Err), colorBad, 6)
				hideAt = 0
				if hidden {
					show()
				}
			} else if hidden && waitingFor == ev.Index && u.running[ev.Index] == 0 {
				show()
			}
		default:
		}

		web, webURL = syncWebControl(web, settings.WebControl, cfg, bridge, u)
		u.webURL = webURL
		bridge.Publish(u.remoteState(hidden))
		if debugSrv != nil {
			debugBridge.Publish(u.debugSnapshot(hidden))
		}

		if hidden {
			drainRemote(bridge) // stale taps must not fire when the launcher returns
			rl.PollInputEvents()
			if in.guideHeldFor(cfg.RestoreHoldSeconds) {
				show()
			}
			rl.WaitTime(0.03)
			continue
		}

		if hideAt > 0 && rl.GetTime() >= hideAt {
			hideAt = 0
			hidden = true
			// GLFW ignores hide requests for full-screen windows, so leave
			// full screen first. show() re-enters it after mapping.
			if rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
				rl.ToggleBorderlessWindowed()
			}
			rl.SetWindowState(rl.FlagWindowHidden)
			lg().Info("launcher hidden")
			continue
		}

		a := in.poll()
		remoteTap := false
		if a == actionNone {
			a, remoteTap = u.nextRemoteAction(bridge)
		}
		if sc != nil && a == actionNone {
			bgReady := !settings.BackgroundsEnabled || (u.bgTex.cur.ID != 0 && !bgPending)
			ready := bgReady && iconsPending.Load() == 0 && len(iconResults) == 0
			var done bool
			if a, done, err = sc.step(u, in, ready); err != nil {
				return fmt.Errorf("script %s: %w", opts.ScriptPath, err)
			} else if done {
				lg().Info("script finished", zap.String("script", opts.ScriptPath))
				return nil
			}
		}
		now := rl.GetTime()
		if in.active || a != actionNone {
			lastInput = now
			forcedIdle = false
		}
		// The input that wakes a faded-out screen only brings it back; it
		// shouldn't also move or launch something the user can't see.
		// A tile tapped on the web remote was chosen on the phone, so it runs
		// even while the TV's tiles are faded out.
		woke := u.bodyAlpha < 0.5 && a != actionNone && a != actionQuit && !remoteTap
		if a != actionNone {
			lg().Debug("action", zap.Int("action", int(a)), zap.Int("screen", int(u.screen)), zap.Int("selected", u.selected), zap.Float32("alpha", u.bodyAlpha), zap.Bool("wakeOnly", woke))
		}
		if woke {
			a = actionNone
		}
		// B / Backspace on the home screen hides the tiles straight away.
		if a == actionBack && u.screen == screenHome {
			forcedIdle = true
			u.bodyAlpha = 0 // hide instantly rather than fading
		}
		u.idle = forcedIdle || (idleFade > 0 && (u.bgTex.cur.ID != 0 || u.hasRSS()) && u.screen != screenGamepads && u.screen != screenConfirm && now-lastInput >= idleFade)
		if a == actionQuit {
			return nil
		}
		if u.screen != screenHome {
			u.handleMenu(a)
			if u.quit {
				return nil
			}
			u.draw()
			continue
		}

		switch a {
		case actionActivate:
			if hideAt > 0 {
				break
			}
			t := u.tiles[u.selected]
			if t.kind == tileMore {
				u.openMore()
				break
			}
			if t.kind == tileAction {
				src := u.sourceByID(t.action.SourceID())
				if src == nil {
					u.setToast(t.title+": "+t.action.SourceID()+" is not configured", colorBad, 5)
					break
				}
				u.actionsRunning[t.key]++
				u.setToast("Starting "+t.title+"...", colorOK, 3)
				go src.Run(t.action, actionEvents)
				break
			}
			e := cfg.Entries[t.entry]
			pid, err := launch.Start(cfg, t.entry, exits)
			if err != nil {
				lg().Warn("launch failed", zap.String("title", e.Title), zap.Error(err))
				u.setToast("Could not launch "+e.Title+": "+err.Error(), colorBad, 6)
				break
			}
			lg().Info("launched", zap.String("title", e.Title), zap.Int("pid", pid))
			started[pid] = time.Now()
			u.running[t.entry]++
			u.setToast("Launching "+e.Title+"...", colorOK, 3)
			if hideOnLaunch {
				hideAt = rl.GetTime() + 1.2
				if config.BoolOr(e.RestoreOnExit, true) {
					waitingFor = t.entry
				}
			}
		case actionExit:
			u.promptExit()
		case actionNone, actionBack:
		default:
			u.move(a)
		}

		u.draw()
	}
	return nil
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("app") }
