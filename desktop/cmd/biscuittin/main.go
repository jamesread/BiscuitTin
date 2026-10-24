// BiscuitTin is a full-screen launcher driven by a gamepad, remote control or
// keyboard arrows. Entries are defined in a YAML config file.
package main

import (
	"flag"
	"fmt"
	"image"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jamesread/biscuittin/internal/app"
	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/debugctl"
	"github.com/jamesread/biscuittin/internal/installationinfo"
)

// Set at build time with -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "nocommit"
	date    = "nodate"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "debug" {
		if err := debugctl.PrintDiagnostics(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "biscuittin debug: %v\n", err)
			os.Exit(1)
		}
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
func run(logger *zap.Logger, configPath string, windowed bool, sizeFlag, scriptPath string) int {
	defer func() { _ = logger.Sync() }()
	zap.ReplaceGlobals(logger)
	defer zap.RedirectStdLog(logger)()

	installationinfo.Build.Version = version
	installationinfo.Build.Commit = commit
	installationinfo.Build.Date = date
	logger.Info("biscuittin", zap.String("version", version), zap.String("commit", commit), zap.String("built", date))

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("config", zap.Error(err))
		return 1
	}
	var size image.Point
	if sizeFlag != "" {
		if size, err = app.ParseSize(sizeFlag); err != nil {
			logger.Error("bad -size", zap.Error(err))
			return 2
		}
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
