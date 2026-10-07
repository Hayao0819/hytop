package app_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/ui/page"
)

type journal struct {
	entries []unitmodel.LogEntry

	mu       sync.Mutex
	followed []string
}

func newJournal(lines int) *journal {
	entries := make([]unitmodel.LogEntry, lines)

	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	for i := range entries {
		entries[i] = unitmodel.LogEntry{
			Time:     at.Add(time.Duration(i) * time.Second),
			Priority: 6,
			Message:  fmt.Sprintf("line %02d", i),
		}
	}

	return &journal{entries: entries}
}

func (j *journal) Follow(_ context.Context, unit string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.followed = append(j.followed, unit)
}

func (j *journal) Entries() []unitmodel.LogEntry { return j.entries }
func (j *journal) Err() error                    { return nil }

func (j *journal) units() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	return append([]string(nil), j.followed...)
}

func (j *journal) follower() func() page.LogFollower { return func() page.LogFollower { return j } }

func logRows(program *reactea.App) []string {
	var found []string

	for _, line := range strings.Split(plain(program), "\n") {
		if strings.Contains(line, "line ") {
			found = append(found, strings.TrimSpace(line))
		}
	}

	return found
}

func openLog(t *testing.T, lines int) *reactea.App {
	t.Helper()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.FollowLog = newJournal(lines).follower()
	})

	mode(t, program, "Services")
	press(t, program, "enter")

	return program
}

func TestScrollingTheJournalKeepsTheWindowFull(t *testing.T) {
	t.Parallel()

	program := openLog(t, 60)

	before := logRows(program)
	if len(before) < 5 {
		t.Fatalf("the journal did not open, got %d lines:\n%s", len(before), plain(program))
	}

	for range 80 {
		press(t, program, "k")

		if got := logRows(program); len(got) != len(before) {
			t.Fatalf("the window went from %d lines to %d:\n%s",
				len(before), len(got), plain(program))
		}
	}

	if got := logRows(program); got[0] == before[0] {
		t.Errorf("80 presses left the window on %q; it never moved", got[0])
	}
}

func TestTheJournalOpensOnTheNewestLine(t *testing.T) {
	t.Parallel()

	program := openLog(t, 60)

	rows := logRows(program)
	if len(rows) == 0 {
		t.Fatal("no journal lines")
	}

	if !strings.HasSuffix(rows[len(rows)-1], "line 59") {
		t.Errorf("the last line is %q, want the newest", rows[len(rows)-1])
	}

	press(t, program, "j")

	if got := logRows(program); got[len(got)-1] != rows[len(rows)-1] {
		t.Errorf("down at the newest scrolled to %q", got[len(got)-1])
	}

	press(t, program, "k")

	if got := logRows(program); got[len(got)-1] == rows[len(rows)-1] {
		t.Error("up did not walk back through the journal")
	}
}

func TestScrollingStopsAtTheOldestLine(t *testing.T) {
	t.Parallel()

	program := openLog(t, 30)

	for range 200 {
		press(t, program, "k")
	}

	rows := logRows(program)
	if len(rows) == 0 {
		t.Fatalf("scrolling emptied the journal:\n%s", plain(program))
	}

	if !strings.HasSuffix(rows[0], "line 00") {
		t.Errorf("the first line is %q, want the oldest", rows[0])
	}
}

func openProcessLog(t *testing.T) (*reactea.App, *journal) {
	t.Helper()

	reader := newJournal(20)

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.FollowLog = reader.follower()
	})

	mode(t, program, "Processes")
	press(t, program, "p", "p", "L")

	return program, reader
}

func following(reader *journal) string {
	units := reader.units()
	if len(units) == 0 {
		return ""
	}

	return units[len(units)-1]
}

func TestTheJournalPaneFollowsTheSelectedProcessUnit(t *testing.T) {
	t.Parallel()

	program, reader := openProcessLog(t)

	press(t, program, "j")

	if got := following(reader); got != "sshd.service" {
		t.Fatalf("the pane followed %q, want sshd.service:\n%s", got, plain(program))
	}

	screen := plain(program)

	if !strings.Contains(screen, "sshd.service") {
		t.Errorf("the pane does not name the unit it follows:\n%s", screen)
	}

	if !strings.Contains(screen, "line 19") {
		t.Errorf("the pane shows no journal lines:\n%s", screen)
	}
}

func TestMovingToAProcessInAnotherUnitFollowsThatOne(t *testing.T) {
	t.Parallel()

	program, reader := openProcessLog(t)

	press(t, program, "j", "j")

	if got := following(reader); got != "session-2.scope" {
		t.Fatalf("the pane followed %q, want session-2.scope:\n%s", got, plain(program))
	}
}

func TestAProcessWithNoUnitFollowsNothing(t *testing.T) {
	t.Parallel()

	program, reader := openProcessLog(t)

	press(t, program, "j")
	press(t, program, "k")

	if got := reader.units(); slices.Contains(got, "") {
		t.Fatalf("the pane was asked to follow the empty unit: %v", got)
	}

	if screen := plain(program); !strings.Contains(screen, "belongs to no unit") {
		t.Errorf("the pane still claims a unit:\n%s", screen)
	}
}

func TestTheFollowerIsNotRestartedForTheUnitItIsAlreadyOn(t *testing.T) {
	t.Parallel()

	program, reader := openProcessLog(t)

	press(t, program, "j")

	press(t, program, "e", "e", "e", "e")

	units := reader.units()

	for i := 1; i < len(units); i++ {
		if units[i] == units[i-1] {
			t.Fatalf("the follower restarted on the unit it was already on: %v", units)
		}
	}
}

func TestTurningTheJournalPaneOffStopsFollowing(t *testing.T) {
	t.Parallel()

	program, reader := openProcessLog(t)

	press(t, program, "j")

	if following(reader) == "" {
		t.Fatal("the pane never followed anything")
	}

	press(t, program, "L")

	if strings.Contains(plain(program), "line 19") {
		t.Errorf("the pane is still drawn after being turned off:\n%s", plain(program))
	}
}
