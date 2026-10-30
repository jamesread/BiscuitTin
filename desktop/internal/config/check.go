package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Problem is one thing wrong with a config file, found by Check.
type Problem struct {
	Where   string // e.g. "entries[2] (Jellyfin)", or "" for the whole file
	Message string
}

func (p Problem) String() string {
	if p.Where == "" {
		return p.Message
	}
	return p.Where + ": " + p.Message
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Check reads the config at path more strictly than Load: unknown keys
// (usually typos, which Load ignores) and malformed colours are problems
// too. It never writes a default config. The parsed config, with defaults,
// is returned when the file parses, so callers can check commands.
func Check(path string) (*Config, []Problem) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, []Problem{{Message: err.Error()}}
	}
	cfg := &Config{}
	problems, ok := decodeStrict(raw, cfg)
	if !ok {
		return nil, problems
	}
	problems = append(problems, checkColors(cfg)...)
	problems = append(problems, checkEntries(cfg.Entries)...)
	ApplyDefaults(cfg)
	return cfg, problems
}

// decodeStrict reports unknown keys and type errors. After those yaml.v3
// still fills in every field it could, so ok is true and the rest of the
// file is checked too; a syntax error stops there.
func decodeStrict(raw []byte, cfg *Config) ([]Problem, bool) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	err := dec.Decode(cfg)
	if err == nil || errors.Is(err, io.EOF) {
		return nil, true
	}
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return []Problem{{Message: err.Error()}}, false
	}
	out := make([]Problem, 0, len(te.Errors))
	for _, e := range te.Errors {
		out = append(out, Problem{Message: e})
	}
	return out, true
}

func checkColors(cfg *Config) []Problem {
	var out []Problem
	for _, c := range []struct{ key, value string }{
		{"background", cfg.Background},
		{"accent", cfg.Accent},
		{"tileColor", cfg.TileColor},
	} {
		if c.value != "" && !hexColor.MatchString(c.value) {
			out = append(out, Problem{Where: c.key, Message: fmt.Sprintf("%q is not a #rrggbb colour", c.value)})
		}
	}
	return out
}

func checkEntries(entries []Entry) []Problem {
	if len(entries) == 0 {
		return []Problem{{Where: "entries", Message: "no entries defined"}}
	}
	var out []Problem
	for i, e := range entries {
		if len(e.Command) == 0 {
			out = append(out, Problem{Where: EntryName(i, e), Message: "no command"})
		}
	}
	return out
}

// EntryName is how Check names an entry: its index and title.
func EntryName(i int, e Entry) string {
	return fmt.Sprintf("entries[%d] (%s)", i, e.Title)
}
