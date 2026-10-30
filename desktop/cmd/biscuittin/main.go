// BiscuitTin is a full-screen launcher driven by a gamepad, remote control or
// keyboard arrows. Entries are defined in a YAML config file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jamesread/biscuittin/internal/app"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/debugctl"
	"github.com/jamesread/biscuittin/internal/installationinfo"
	"github.com/jamesread/biscuittin/internal/instance"
	"github.com/jamesread/biscuittin/internal/launch"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "nocommit"
	date    = "nodate"
)

func handledDebugCommand() bool {
	if len(os.Args) <= 1 {
		return false
	}
	if os.Args[1] != "debug" {
		return false
	}
	if err := debugctl.PrintDiagnostics(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "biscuittin debug: %v\n", err)
		os.Exit(1)
	}
	return true
}

func main() {
	if handledDebugCommand() {
		return
	}
	if handledCheckCommand() {
		return
	}

	configPath := flag.String("config", config.DefaultPath(), "path to the YAML config file")
	windowed := flag.Bool("windowed", false, "run in a window instead of full screen")
	debug := flag.Bool("debug", false, "log at debug level, including input actions")
	sizeFlag := flag.String("size", "", "window size WIDTHxHEIGHT, e.g. 1920x1080 (implies -windowed)")
	scriptPath := flag.String("script", "", "run the steps in this file, e.g. to take screenshots, then quit")
	flag.Parse()

	logger, err := newLogger(*debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "biscuittin: logger: %v\n", err)
		os.Exit(1)
	}
	if code := run(logger, *configPath, *windowed, *sizeFlag, *scriptPath); code != 0 {
		os.Exit(code)
	}
}

// run is main without os.Exit, so the deferred log flush always happens.
func parseWindowSize(sizeFlag string) (image.Point, error) {
	if sizeFlag == "" {
		return image.Point{}, nil
	}
	return app.ParseSize(sizeFlag)
}

func run(logger *zap.Logger, configPath string, windowed bool, sizeFlag, scriptPath string) int {
	defer func() { _ = logger.Sync() }()
	zap.ReplaceGlobals(logger)
	defer zap.RedirectStdLog(logger)()

	installationinfo.Build.Version = version
	installationinfo.Build.Commit = commit
	installationinfo.Build.Date = date
	logger.Info("biscuittin", zap.String("version", version), zap.String("commit", commit), zap.String("built", date))
	// Scripted runs (screenshots) use their own display, so they may run
	// beside a live launcher.
	if scriptPath == "" {
		lock, ok := acquireInstance(logger)
		if !ok {
			return 0
		}
		defer lock.Release()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("config", zap.Error(err))
		return 1
	}
	size, err := parseWindowSize(sizeFlag)
	if err != nil {
		logger.Error("bad -size", zap.Error(err))
		return 2
	}

	err = app.Run(cfg, app.Options{
		ConfigPath: configPath,
		Windowed:   windowed,
		Size:       size,
		ScriptPath: scriptPath,
	})
	if err != nil {
		logger.Error("biscuittin stopped", zap.Error(err))
		return 1
	}
	return 0
}

// acquireInstance takes the pid file. When another BiscuitTin is running it
// asks that one to come to the foreground and returns false, so this one
// exits. A pid file that cannot be written is logged and ignored.
func acquireInstance(logger *zap.Logger) (*instance.Lock, bool) {
	path := instance.Path()
	lock, pid, err := instance.Acquire(path)
	if errors.Is(err, instance.ErrRunning) {
		logger.Info("already running, bringing it to the foreground", zap.Int("pid", pid), zap.String("pidFile", path))
		if err := instance.Raise(pid); err != nil {
			logger.Warn("could not signal the running instance", zap.Int("pid", pid), zap.Error(err))
		}
		return nil, false
	}
	if err != nil {
		logger.Warn("pid file", zap.String("path", path), zap.Error(err))
	}
	return lock, true
}

// newLogger logs human-readable lines to stderr, which usually goes to a
// log file when BiscuitTin runs on a TV box.
func newLogger(debug bool) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Sampling = nil // keep every line; volume is low
	if debug {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	return cfg.Build()
}

// handledCheckCommand runs `biscuittin check [-config path]`.
func handledCheckCommand() bool {
	if len(os.Args) <= 1 || os.Args[1] != "check" {
		return false
	}
	os.Exit(runCheck(os.Args[2:], os.Stdout, os.Stderr))
	return true
}

// installed is launch.Installed; tests replace it.
var installed = launch.Installed

// runCheck validates a config file without starting the launcher: unknown
// keys, bad colours and commands that aren't installed. It exits 1 if
// anything is wrong.
func runCheck(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(errOut)
	path := fs.String("config", config.DefaultPath(), "path to the YAML config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, problems := config.Check(*path)
	if cfg != nil {
		problems = append(problems, missingCommands(cfg)...)
	}
	for _, p := range problems {
		_, _ = fmt.Fprintf(out, "%s: %s\n", *path, p)
	}
	if len(problems) > 0 {
		return 1
	}
	_, _ = fmt.Fprintf(out, "%s: OK, %d entries\n", *path, len(cfg.Entries))
	return 0
}

func missingCommands(cfg *config.Config) []config.Problem {
	var out []config.Problem
	for i, e := range cfg.Entries {
		if len(e.Command) == 0 {
			continue
		}
		if ok, known := installed(e.Command); known && !ok {
			out = append(out, config.Problem{Where: config.EntryName(i, e), Message: notInstalledMessage(e.Command)})
		}
	}
	return out
}

// notInstalledMessage names the Flatpak app id when that is what's missing.
func notInstalledMessage(cmd []string) string {
	if p := launch.ProbeArgs(cmd); len(p) == 3 && p[0] == "flatpak" {
		return "Flatpak " + p[2] + " is not installed"
	}
	return "command not found: " + cmd[0]
}
