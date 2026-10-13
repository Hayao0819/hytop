package app

import (
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/conf"
)

func TestSettingsRetainCustomValuesAsChoices(t *testing.T) {
	t.Parallel()

	cfg := conf.Default()
	cfg.General.Interval = conf.Duration(250 * time.Millisecond)
	cfg.General.Span = conf.Duration(7 * time.Minute)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	r := &Root{config: cfg}
	want := map[string]string{"Graph history": "7m", "Collection period": "250ms"}
	for _, item := range r.settingItems() {
		if value, ok := want[item.Label]; ok {
			if got := item.Options[item.Chosen]; got != value {
				t.Errorf("%s displays %s, configured %s", item.Label, got, value)
			}
			delete(want, item.Label)
		}
	}
	if len(want) != 0 {
		t.Fatalf("settings are missing: %v", want)
	}
}

func TestSetupRetainsACustomCollectionPeriod(t *testing.T) {
	t.Parallel()

	cfg := conf.Default()
	cfg.General.Interval = conf.Duration(250 * time.Millisecond)
	s := &setup{config: cfg, step: setupInterval}
	item, ok := s.item()
	if !ok || item.Options[index(item.Options, item.Value(cfg))] != "250ms" {
		t.Fatalf("setup collection choices = %v", item.Options)
	}
	s.change(1)
	if s.config.General.Interval != conf.Duration(500*time.Millisecond) {
		t.Fatalf("period after custom value = %s, want 500ms", s.config.General.Interval)
	}
}
