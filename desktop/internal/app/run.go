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
	"github.com/jamesread/biscuittin/internal/tray"
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
	err     error
	source  string
	actions []integrations.ListedAction
}

// Options are the command-line choices for a run.
type Options struct {
	ConfigPath string
	ScriptPath string
	Size       image.Point
	Windowed   bool
}

// Run opens the launcher window and runs it until the user quits or the
// script finishes.
func Run(cfg *config.Config, opts Options) error {
	s, err := startSession(cfg, opts)
	if err != nil {
		return err
	}
	defer s.close()
	return s.loop()
}

type session struct {
	cfg           *config.Config
	settings      *settings.Settings
	u             *ui
	in            *input
	sc            *script
	bg            *backgrounds.Fetcher
	iconResults   chan icons.Result
	actionLists   chan actionListResult
	actionEvents  chan integrations.Event
	iconRequested map[string]bool
	bridge        *webcontrol.Bridge
	web           *webcontrol.Server
	debugBridge   *debugctl.Bridge
	debugSrv      *debugctl.Server
	tray          *tray.Host
	exits         chan launch.Exit
	started       map[int]time.Time
	scriptPath    string
	webURL        string
	bgNext        float64
	bgShownAt     float64
	lastInput     float64
	idleFade      float64
	hideAt        float64
	waitingFor    int
	iconsPending  atomic.Int32
	bgPending     bool
	hidden        bool
	forcedIdle    bool
	hideOnLaunch  bool
	fullscreen    bool
}

func startSession(cfg *config.Config, opts Options) (*session, error) {
	sc, err := sessionScript(opts.ScriptPath)
	if err != nil {
		return nil, err
	}
	fullscreen := sessionFullscreen(cfg, opts)
	openWindow(cfg, sessionSize(opts), fullscreen)
	st := settings.Load(cfg)
	s := newSession(cfg, opts, st, sessionUI(cfg, st, opts), sc, fullscreen)
	s.loadRecentBackground()
	s.goIcon(func() { icons.Resolve(cfg, entryKey, s.iconResults) })
	s.requestActionIcons()
	s.startDebug()
	s.startTray()
	s.lastInput = rl.GetTime()
	lg().Info("started", zap.Int("entries", len(cfg.Entries)), zap.String("config", opts.ConfigPath))
	return s, nil
}

func sessionSize(opts Options) image.Point {
	if opts.Size != (image.Point{}) {
		return opts.Size
	}
	return image.Point{X: 1280, Y: 720}
}

func sessionScript(path string) (*script, error) {
	if path == "" {
		return nil, nil
	}
	return loadScript(path)
}

func sessionFullscreen(cfg *config.Config, opts Options) bool {
	if opts.Windowed {
		return false
	}
	if opts.Size != (image.Point{}) {
		return false
	}
	return config.BoolOr(cfg.Fullscreen, true)
}

func sessionUI(cfg *config.Config, st *settings.Settings, opts Options) *ui {
	u := newUI(cfg, st)
	u.configPath = opts.ConfigPath
	u.wantSize = opts.Size
	u.rss = rss.NewFetcher(cfg)
	return u
}

func newSession(cfg *config.Config, opts Options, st *settings.Settings, u *ui, sc *script, fullscreen bool) *session {
	s := &session{
		cfg:           cfg,
		scriptPath:    opts.ScriptPath,
		settings:      st,
		u:             u,
		in:            &input{},
		sc:            sc,
		bg:            backgrounds.NewFetcher(cfg),
		iconResults:   make(chan icons.Result, 32),
		actionLists:   make(chan actionListResult, 4),
		actionEvents:  make(chan integrations.Event, 16),
		iconRequested: map[string]bool{},
		bridge:        webcontrol.NewBridge(),
		debugBridge:   debugctl.NewBridge(),
		exits:         make(chan launch.Exit, 16),
		started:       map[int]time.Time{},
		waitingFor:    -1,
		hideOnLaunch:  config.BoolOr(cfg.HideOnLaunch, true),
		fullscreen:    fullscreen,
		idleFade:      20,
	}
	u.input = s.in
	if cfg.IdleFadeSeconds != nil {
		s.idleFade = *cfg.IdleFadeSeconds
	}
	return s
}

func (s *session) loadRecentBackground() {
	if !s.settings.BackgroundsEnabled {
		return
	}
	img, ok := backgrounds.LoadRecent(s.cfg.CacheDir)
	if !ok {
		return
	}
	s.u.setBackground(img.Pix, img.Grid, img.Credit)
	s.bgShownAt = rl.GetTime()
}

func (s *session) startDebug() {
	debugSrv, debugErr := debugctl.Start("", s.debugBridge)
	if debugErr != nil {
		lg().Warn("debug socket", zap.Error(debugErr))
		return
	}
	s.debugSrv = debugSrv
	s.u.debugSocket = debugSrv.Path()
}

func (s *session) startTray() {
	s.tray = tray.Start()
}

func openWindow(cfg *config.Config, size image.Point, fullscreen bool) {
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
	}
	rl.InitAudioDevice()
	rl.SetExitKey(0)
	rl.SetTargetFPS(60)
}

func (s *session) close() {
	if s.web != nil {
		s.web.Stop()
	}
	if s.debugSrv != nil {
		s.debugSrv.Stop()
	}
	s.tray.Stop()
	s.u.unload()
	rl.CloseAudioDevice()
	rl.CloseWindow()
}

func (s *session) goIcon(f func()) {
	s.iconsPending.Add(1)
	go func() {
		defer s.iconsPending.Add(-1)
		f()
	}()
}

func (s *session) requestActionIcons() {
	for _, t := range s.u.tiles {
		if t.kind == tileAction && t.action.Icon != "" && !s.iconRequested[t.key] {
			s.iconRequested[t.key] = true
			s.goIcon(func() { icons.ResolveIconify(s.cfg, t.key, t.action.Icon, s.iconResults) })
		}
	}
}

func (s *session) show() {
	rl.ClearWindowState(rl.FlagWindowHidden)
	if s.fullscreen && !rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
		rl.ToggleBorderlessWindowed()
	}
	rl.SetWindowFocused()
	s.hidden = false
	s.waitingFor = -1
	// Reset key repeat and Guide-hold state, keeping the primary device.
	*s.in = input{primary: s.in.primary, pad: s.in.pad, usedInput: s.in.usedInput, padOverride: s.in.padOverride}
	s.lastInput = rl.GetTime()
	lg().Info("launcher shown")
}

func tryRecv[T any](ch <-chan T) (T, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		var z T
		return z, false
	}
}

func (s *session) loop() error {
	for !rl.WindowShouldClose() {
		if done, err := s.tick(); done {
			return err
		}
	}
	return nil
}

func (s *session) tick() (bool, error) {
	s.drainAsync()
	s.fetchRequestedLists()
	s.applySettingsChange()
	s.requestBackground()
	s.handleExits()
	s.syncServers()
	s.drainTray()
	if s.u.quit {
		return true, nil
	}
	return s.advanceFrame()
}

func (s *session) drainTray() {
	if s.tray == nil {
		return
	}
	ev, ok := tryRecv(s.tray.Events)
	if !ok {
		return
	}
	s.applyTray(ev)
}

func (s *session) applyTray(ev tray.Event) {
	if ev == tray.EventQuit {
		s.u.quit = true
		return
	}
	s.hideAt = 0
	s.show()
}

func (s *session) advanceFrame() (bool, error) {
	if s.hidden {
		s.pollWhileHidden()
		return false, nil
	}
	if s.hideIfDue() {
		return false, nil
	}
	return s.handleVisibleFrame()
}

func (s *session) drainAsync() {
	s.drainResults()
	s.drainBackground()
}

func (s *session) drainResults() {
	if r, ok := tryRecv(s.iconResults); ok {
		s.u.setIcon(r.Key, r.Path)
		return
	}
	if l, ok := tryRecv(s.actionLists); ok {
		s.applyActionList(l)
		return
	}
	if ev, ok := tryRecv(s.actionEvents); ok {
		s.applyActionEvent(ev)
	}
}

func (s *session) applyActionList(l actionListResult) {
	lst := s.u.list(l.source)
	lst.loading = false
	if l.err != nil {
		lg().Warn("action list failed", zap.String("source", l.source), zap.Error(l.err))
		lst.err = integrations.ShortErr(l.err)
		return
	}
	lst.actions = l.actions
}

func eventColor(failed bool) color.RGBA {
	if failed {
		return colorBad
	}
	return colorOK
}

func (s *session) applyActionEvent(ev integrations.Event) {
	if ev.Done {
		s.u.actionsRunning[ev.Key]--
	}
	if ev.Msg == "" {
		return
	}
	s.u.setToast(ev.Msg, eventColor(ev.Failed), 5)
}

func (s *session) drainBackground() {
	img, ok := tryRecv(s.bg.Out)
	if !ok {
		return
	}
	s.applyBackgroundImage(img)
}

func (s *session) applyBackgroundImage(img backgrounds.Image) {
	s.bgPending = false
	if img.Err != nil {
		lg().Warn("no background image", zap.Error(img.Err))
		s.bgNext = rl.GetTime() + 60
		return
	}
	if !s.settings.BackgroundsEnabled {
		return
	}
	s.u.setBackground(img.Pix, img.Grid, img.Credit)
	s.bgShownAt = rl.GetTime()
	s.bgNext = s.bgShownAt + float64(s.settings.BackgroundInterval)
}

func (s *session) fetchRequestedLists() {
	for _, src := range s.u.sources {
		if l := s.u.list(src.ID()); l.requested {
			l.requested = false
			go func(src integrations.Source) {
				acts, listErr := src.List()
				s.actionLists <- actionListResult{source: src.ID(), actions: acts, err: listErr}
			}(src)
		}
	}
}

func (s *session) applySettingsChange() {
	if !s.u.settingsChanged {
		return
	}
	s.u.settingsChanged = false
	s.requestActionIcons()
	if saveErr := s.settings.Save(); saveErr != nil {
		lg().Error("save settings", zap.Error(saveErr))
	}
	s.applyBackgroundSetting()
}

func (s *session) applyBackgroundSetting() {
	if !s.settings.BackgroundsEnabled {
		s.u.clearBackground()
		s.bgShownAt = 0
		return
	}
	if s.bgShownAt == 0 {
		s.loadRecentBackground()
		s.bgNext = 0
		return
	}
	s.bgNext = s.bgShownAt + float64(s.settings.BackgroundInterval)
}

func (s *session) requestBackground() {
	if !s.hidden && s.settings.BackgroundsEnabled && !s.bgPending && rl.GetTime() >= s.bgNext {
		s.bgPending = true
		s.bg.Request(rl.GetScreenWidth(), rl.GetScreenHeight())
	}
}

func (s *session) handleExits() {
	ev, ok := tryRecv(s.exits)
	if !ok {
		return
	}
	s.noteExit(ev)
}

func (s *session) noteExit(ev launch.Exit) {
	e := s.cfg.Entries[ev.Index]
	s.u.pids[ev.Index] = removePID(s.u.pids[ev.Index], ev.PID)
	ranFor := time.Since(s.started[ev.PID])
	delete(s.started, ev.PID)
	lg().Info("app exited", zap.String("title", e.Title), zap.Int("pid", ev.PID), zap.Duration("ranFor", ranFor.Round(time.Millisecond)), zap.Error(ev.Err))
	if exitFailedQuickly(ev, ranFor) {
		s.u.setToast(describeExit(e, ev.Err), colorBad, 6)
		s.hideAt = 0
		s.showIfHidden()
		return
	}
	s.showIfLastWaiting(ev)
}

func exitFailedQuickly(ev launch.Exit, ranFor time.Duration) bool {
	if ev.Err == nil {
		return false
	}
	return ranFor < quickFailure
}

func (s *session) showIfHidden() {
	if s.hidden {
		s.show()
	}
}

func (s *session) showIfLastWaiting(ev launch.Exit) {
	if !s.hidden {
		return
	}
	if s.waitingFor != ev.Index {
		return
	}
	if hasLivePID(s.u.pids[ev.Index]) {
		return
	}
	s.show()
}

func (s *session) syncServers() {
	s.web, s.webURL = syncWebControl(s.web, s.settings.WebControl, s.cfg, s.bridge, s.u)
	s.u.webURL = s.webURL
	s.bridge.Publish(s.u.remoteState(s.hidden))
	if s.debugSrv != nil {
		s.debugBridge.Publish(s.u.debugSnapshot(s.hidden))
	}
}

func (s *session) pollWhileHidden() {
	drainRemote(s.bridge) // stale taps must not fire when the launcher returns
	rl.PollInputEvents()
	if s.in.guideHeldFor(s.cfg.RestoreHoldSeconds) {
		s.show()
	}
	rl.WaitTime(0.03)
}

func (s *session) hideIfDue() bool {
	if s.hideAt == 0 || rl.GetTime() < s.hideAt {
		return false
	}
	s.hideAt = 0
	s.hidden = true
	// GLFW ignores hide requests for full-screen windows, so leave
	// full screen first. show() re-enters it after mapping.
	if rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
		rl.ToggleBorderlessWindowed()
	}
	rl.SetWindowState(rl.FlagWindowHidden)
	lg().Info("launcher hidden")
	return true
}

func (s *session) handleVisibleFrame() (done bool, err error) {
	a, remoteTap := s.nextAction()
	a, done, err = s.maybeScript(a)
	if done {
		return true, err
	}
	a = s.applyIdle(a, remoteTap)
	syncCursor(s.u.idle)
	if a == actionQuit {
		return true, nil
	}
	return s.drawAfterAction(a), nil
}

func syncCursor(idle bool) {
	if idle {
		rl.HideCursor()
		return
	}
	rl.ShowCursor()
}

func (s *session) maybeScript(a action) (action, bool, error) {
	if s.sc == nil {
		return a, false, nil
	}
	if a != actionNone {
		return a, false, nil
	}
	a, scriptDone, err := s.stepScript()
	if err != nil {
		return a, true, err
	}
	return a, scriptDone, nil
}

func (s *session) drawAfterAction(a action) bool {
	s.u.playAction(a)
	if s.u.screen != screenHome {
		s.u.handleMenu(a)
		if s.u.quit {
			return true
		}
		s.u.draw()
		return false
	}
	s.handleHome(a)
	s.u.draw()
	return false
}

func (s *session) nextAction() (action, bool) {
	a := s.in.poll()
	if a != actionNone {
		return a, false
	}
	return s.u.nextRemoteAction(s.bridge)
}

func (s *session) stepScript() (a action, done bool, err error) {
	a, done, err = s.sc.step(s.u, s.in, s.scriptReady())
	if err != nil {
		return 0, true, fmt.Errorf("script %s: %w", s.scriptPath, err)
	}
	if done {
		lg().Info("script finished", zap.String("script", s.scriptPath))
	}
	return a, done, nil
}

func (s *session) bgReady() bool {
	if !s.settings.BackgroundsEnabled {
		return true
	}
	if s.u.bgTex.cur.ID == 0 {
		return false
	}
	return !s.bgPending
}

func (s *session) scriptReady() bool {
	if !s.bgReady() {
		return false
	}
	if s.iconsPending.Load() != 0 {
		return false
	}
	return len(s.iconResults) == 0
}

func (s *session) applyIdle(a action, remoteTap bool) action {
	s.noteActivity(a)
	if a != actionNone {
		lg().Debug("action", zap.Int("action", int(a)), zap.Int("screen", int(s.u.screen)), zap.Int("selected", s.u.selected), zap.Float32("alpha", s.u.bodyAlpha), zap.Bool("wakeOnly", s.wakeOnly(a, remoteTap)))
	}
	a = s.suppressWake(a, remoteTap)
	s.hideTilesIfBack(a)
	s.u.idle = s.shouldIdle()
	return a
}

func (s *session) noteActivity(a action) {
	if !s.in.active {
		if a == actionNone {
			return
		}
	}
	s.lastInput = rl.GetTime()
	s.forcedIdle = false
}

func (s *session) suppressWake(a action, remoteTap bool) action {
	if !s.wakeOnly(a, remoteTap) {
		return a
	}
	return actionNone
}

func (s *session) wakeOnly(a action, remoteTap bool) bool {
	if remoteTap {
		return false
	}
	if a == actionNone {
		return false
	}
	if a == actionQuit {
		return false
	}
	return s.u.bodyAlpha < 0.5
}

func (s *session) hideTilesIfBack(a action) {
	if a != actionBack {
		return
	}
	if s.u.screen != screenHome {
		return
	}
	s.forcedIdle = true
	s.u.bodyAlpha = 0 // hide instantly rather than fading
}

func (s *session) shouldIdle() bool {
	if s.forcedIdle {
		return true
	}
	return s.idleTimedOut()
}

func (s *session) idleTimedOut() bool {
	if s.idleFade <= 0 {
		return false
	}
	if !s.hasIdleBackdrop() {
		return false
	}
	if s.blocksIdle() {
		return false
	}
	return rl.GetTime()-s.lastInput >= s.idleFade
}

func (s *session) hasIdleBackdrop() bool {
	if s.u.bgTex.cur.ID != 0 {
		return true
	}
	return s.u.hasRSS()
}

var idleBlocking = map[screen]bool{
	screenGamepads: true,
	screenConfirm:  true,
	screenRunning:  true,
}

func (s *session) blocksIdle() bool {
	return idleBlocking[s.u.screen]
}

func (s *session) handleHome(a action) {
	switch a {
	case actionActivate:
		s.activateSelected()
	case actionExit:
		s.u.promptExit()
	case actionNone, actionBack:
	default:
		s.u.move(a)
	}
}

func (s *session) activateSelected() {
	if s.hideAt > 0 {
		return
	}
	t := s.u.tiles[s.u.selected]
	if t.kind == tileMore {
		s.u.openMore()
		return
	}
	if t.kind == tileAction {
		s.runActionTile(t)
		return
	}
	s.launchEntry(t)
}

func (s *session) runActionTile(t tile) {
	src := s.u.sourceByID(t.action.SourceID())
	if src == nil {
		s.u.setToast(t.title+": "+t.action.SourceID()+" is not configured", colorBad, 5)
		return
	}
	s.u.actionsRunning[t.key]++
	s.u.setToast("Starting "+t.title+"...", colorOK, 3)
	go src.Run(t.action, s.actionEvents)
}

func (s *session) launchEntry(t tile) {
	if hasLivePID(s.u.pids[t.entry]) {
		s.openRunningMenu(t.entry)
		return
	}
	s.startEntry(t.entry)
}

func (s *session) openRunningMenu(entry int) {
	s.u.openRunning(s.cfg.Entries[entry].Title, func() { s.killEntry(entry) }, func() { s.startEntry(entry) })
}

func (s *session) startEntry(entry int) {
	s.u.screen = screenHome
	e := s.cfg.Entries[entry]
	pid, err := launch.Start(s.cfg, entry, s.exits)
	if err != nil {
		lg().Warn("launch failed", zap.String("title", e.Title), zap.Error(err))
		s.u.setToast("Could not launch "+e.Title+": "+err.Error(), colorBad, 6)
		return
	}
	s.noteLaunch(e, entry, pid)
}

func (s *session) killEntry(entry int) {
	s.u.screen = screenHome
	title := s.cfg.Entries[entry].Title
	if err := killPIDs(livePIDs(s.u.pids[entry])); err != nil {
		s.u.setToast("Could not stop "+title+": "+err.Error(), colorBad, 6)
		return
	}
	s.u.setToast("Stopping "+title+"...", colorOK, 3)
}

func (s *session) noteLaunch(e config.Entry, entry, pid int) {
	lg().Info("launched", zap.String("title", e.Title), zap.Int("pid", pid))
	s.started[pid] = time.Now()
	s.u.pids[entry] = append(s.u.pids[entry], pid)
	s.u.setToast("Launching "+e.Title+"...", colorOK, 3)
	if !s.hideOnLaunch {
		return
	}
	s.hideAt = rl.GetTime() + 1.2
	if config.BoolOr(e.RestoreOnExit, true) {
		s.waitingFor = entry
	}
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("app") }
