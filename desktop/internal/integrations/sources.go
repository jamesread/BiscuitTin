// Package integrations connects to services whose actions can be pinned as
// tiles: OliveTin actions and Home Assistant automations.
package integrations

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"go.uber.org/zap"
)

// Source is an integration whose actions can be pinned as tiles from
// More: OliveTin actions, Home Assistant automations, and future ones.
type Source interface {
	// ID is stored with each pin, e.g. "olivetin".
	ID() string
	// Title names the list in the More menu.
	Title() string
	// Where describes the server, for loading and error messages.
	Where() string
	List() ([]ListedAction, error)
	// Run performs the action, reporting progress on events. The last event
	// has done set.
	Run(a PinnedAction, events chan<- Event)
}

// PinnedAction is an action shown as a tile. Title, group and icon are saved
// with it so tiles appear at startup even if the server is unreachable.
type PinnedAction struct {
	// Source is the Source ID. Empty means "olivetin" (older pins).
	Source    string     `yaml:"source,omitempty"`
	BindingID string     `yaml:"bindingId"`
	Title     string     `yaml:"title"`
	Group     string     `yaml:"group,omitempty"`
	Icon      string     `yaml:"icon,omitempty"`
	Args      []ArgValue `yaml:"args,omitempty"`
}

func (a PinnedAction) SourceID() string {
	if a.Source == "" {
		return "olivetin"
	}
	return a.Source
}

// Key identifies the action across sources, for tiles and running counts.
func (a PinnedAction) Key() string {
	return "act:" + a.SourceID() + ":" + a.BindingID
}

// ListedAction is an action offered for pinning.
type ListedAction struct {
	PinnedAction
	// NeedsInput is set when the action can't be run from a single button
	// press (e.g. an OliveTin argument with no default).
	NeedsInput bool
}

// Event reports progress of a running action. failed marks msg as an
// error.
type Event struct {
	Key    string
	Msg    string
	Failed bool
	Done   bool
}

// CatalogEntry is a known integration, whether or not it is configured.
type CatalogEntry struct {
	ID    string
	Title string
}

// Catalog is every integration the Integrations menu lists.
func Catalog() []CatalogEntry {
	return []CatalogEntry{
		{ID: "olivetin", Title: "OliveTin actions"},
		{ID: "homeassistant", Title: "Home Assistant automations"},
	}
}

func NewSources(cfg *config.Config) []Source {
	var out []Source
	if cfg.OliveTin.URL != "" {
		out = append(out, &oliveTinSource{client: newOliveTinClient(cfg.OliveTin.URL)})
	}
	if cfg.HomeAssistant.URL != "" {
		out = append(out, newHomeAssistantSource(cfg.HomeAssistant))
	}
	return out
}

type oliveTinSource struct {
	client *olivetinClient
}

func (s *oliveTinSource) ID() string    { return "olivetin" }
func (s *oliveTinSource) Title() string { return "OliveTin actions" }
func (s *oliveTinSource) Where() string { return s.client.base }

func (s *oliveTinSource) List() ([]ListedAction, error) {
	acts, err := s.client.ListActions()
	for i := range acts {
		acts[i].Source = s.ID()
	}
	return acts, err
}

const otWaitTimeout = 30 * time.Minute

func (s *oliveTinSource) Run(a PinnedAction, events chan<- Event) {
	key := a.Key()
	id, err := s.client.Start(a)
	if err != nil {
		lg("olivetin").Warn("start failed", zap.String("action", a.BindingID), zap.Error(err))
		events <- Event{Key: key, Msg: "Could not start " + a.Title + ": " + err.Error(), Failed: true, Done: true}
		return
	}
	lg("olivetin").Info("started", zap.String("title", a.Title), zap.String("action", a.BindingID), zap.String("trackingId", id))

	code, err := s.client.Wait(id, otWaitTimeout)
	switch {
	case err != nil:
		events <- Event{Key: key, Msg: a.Title + ": " + err.Error(), Failed: true, Done: true}
	case code != 0:
		events <- Event{Key: key, Msg: fmt.Sprintf("%s failed (exit %d)", a.Title, code), Failed: true, Done: true}
	default:
		events <- Event{Key: key, Msg: a.Title + " finished", Done: true}
	}
	lg("olivetin").Info("finished", zap.String("action", a.BindingID), zap.Int("exitCode", code), zap.Error(err))
}

func ShortErr(err error) string {
	s := err.Error()
	if i := strings.Index(s, "\n"); i > 0 {
		s = s[:i]
	}
	return s
}

// lg is the logger for one integration. main installs the global logger.
func lg(source string) *zap.Logger { return zap.L().Named(source) }
