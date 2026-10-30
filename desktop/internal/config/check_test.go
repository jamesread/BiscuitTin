package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkYAML(t *testing.T, yml string) (*Config, []string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, problems := Check(path)
	out := make([]string, 0, len(problems))
	for _, p := range problems {
		out = append(out, p.String())
	}
	return cfg, out
}

func TestCheckClean(t *testing.T) {
	cfg, problems := checkYAML(t, "accent: \"#f2b134\"\nentries:\n  - title: Kodi\n    command: [kodi]\n    windowClass: Kodi\n")
	if len(problems) != 0 || cfg == nil || cfg.Volume.GetCommand == nil {
		t.Fatalf("problems %v, cfg %v", problems, cfg)
	}
}

func TestCheckProblems(t *testing.T) {
	cases := []struct {
		name, yml string
		want      []string // substrings, one per problem
		parsed    bool
	}{
		{"typo in key", "colums: 4\nentries:\n  - title: A\n    command: [a]\n", []string{"field colums not found"}, true},
		{"typo in entry", "entries:\n  - title: A\n    comand: [a]\n", []string{"field comand not found", "entries[0] (A): no command"}, true},
		{"bad colour", "tileColor: blue\nentries:\n  - title: A\n    command: [a]\n", []string{`tileColor: "blue" is not a #rrggbb colour`}, true},
		{"no entries", "title: x\n", []string{"entries: no entries defined"}, true},
		{"syntax", "entries: [\n", []string{"yaml:"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, got := checkYAML(t, c.yml)
			if (cfg != nil) != c.parsed {
				t.Fatalf("parsed = %v", cfg != nil)
			}
			if len(got) != len(c.want) {
				t.Fatalf("problems %q; want %q", got, c.want)
			}
			for i, w := range c.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("problem %q lacks %q", got[i], w)
				}
			}
		})
	}
}

func TestCheckMissingFileNotWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	if _, problems := Check(path); len(problems) != 1 {
		t.Fatalf("problems %v", problems)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("Check must not write a default config")
	}
}

func TestDefaultConfigChecksClean(t *testing.T) {
	_, problems := checkYAML(t, string(defaultYAML))
	if len(problems) != 0 {
		t.Fatalf("default.yaml has problems: %v", problems)
	}
}
