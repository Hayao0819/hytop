package version

import (
	"runtime/debug"
	"testing"
)

func TestBuildInformationFillsDevelopmentDefaults(t *testing.T) {
	t.Parallel()

	build := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
			{Key: "vcs.time", Value: "2026-09-05T02:11:34Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	got := supplement(Info{Version: "dev"}, build)

	if got.Version != "v1.2.3" || got.Commit != "0123456789abcdef" || got.Date != "2026-09-05T02:11:34Z" {
		t.Errorf("supplemented info = %+v", got)
	}
	if !got.Modified {
		t.Error("the modified flag was lost")
	}
}

func TestInjectedReleaseInformationWins(t *testing.T) {
	t.Parallel()

	want := Info{Version: "v2.0.0", Commit: "release-commit", Date: "release-date"}
	build := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "vcs-commit"},
			{Key: "vcs.time", Value: "vcs-date"},
		},
	}

	got := supplement(want, build)

	if got.Version != want.Version || got.Commit != want.Commit || got.Date != want.Date {
		t.Errorf("supplemented info = %+v, want injected values %+v", got, want)
	}
}

func TestProjectLinksAreAbsolute(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{"repository": Repository, "twitter": Twitter} {
		if len(value) < len("https://") || value[:len("https://")] != "https://" {
			t.Errorf("%s URL = %q", name, value)
		}
	}
}
