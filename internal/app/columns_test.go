package app_test

import (
	"strings"
	"testing"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/conf"
	"github.com/Hayao0819/hytop/internal/domain/filter"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

func TestTheTableShowsTheColumnsThatWereAskedFor(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Processes.Columns = []string{"pid", "ppid", "nice", "vsz", "unit", "command"}

	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = c })

	mode(t, program, "Processes")

	got := plain(program)

	for _, want := range []string{"PID", "PPID", "NI", "VIRT", "UNIT", "COMMAND"} {
		if !strings.Contains(got, want) {
			t.Errorf("the header is missing %q:\n%s", want, got)
		}
	}

	for _, gone := range []string{"CPU%", "THR"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q was not asked for but is drawn:\n%s", gone, got)
		}
	}
}

func TestAnUnknownColumnIsRefused(t *testing.T) {
	t.Parallel()

	c := conf.Default()
	c.Processes.Columns = []string{"pid", "colour"}

	err := c.Validate()
	if err == nil {
		t.Fatal("an unknown column should be refused, not skipped")
	}

	for _, want := range []string{"colour", "cgroup", "command"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestEveryColumnInTheVocabularyCanBeDrawn(t *testing.T) {
	t.Parallel()

	for _, column := range procmodel.Columns() {
		c := settings(t)
		c.Processes.Columns = []string{column.Key}

		if err := c.Validate(); err != nil {
			t.Fatalf("%s: %v", column.Key, err)
		}

		program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = c })

		mode(t, program, "Processes")

		if got := plain(program); !strings.Contains(got, column.Title) {
			t.Errorf("column %q (%s) never appeared:\n%s", column.Key, column.Title, got)
		}
	}
}

func TestColumnsThatDoNotFitAreLeftOffNotWrapped(t *testing.T) {
	t.Parallel()

	c := settings(t)
	c.Processes.Columns = procmodel.ColumnKeys()

	program, _, _ := fixtureWith(t, func(env *app.Env) { env.Config = c })

	mode(t, program, "Processes")

	for i, line := range strings.Split(plain(program), "\n") {
		if width := len([]rune(line)); width > 100 {
			t.Fatalf("row %d is %d cells wide in a 100-cell terminal:\n%s", i, width, line)
		}
	}

	if got := plain(program); !strings.Contains(got, "›") {
		t.Errorf("columns were left off without saying so:\n%s", got)
	}
}

func TestKernelThreadsAreOffUntilAskedFor(t *testing.T) {
	t.Parallel()

	program, _, view := fixtureWith(t, func(env *app.Env) { env.Config = settings(t) })

	mode(t, program, "Processes")

	if got := plain(program); strings.Contains(got, "kthreadd") {
		t.Errorf("a kernel thread is drawn before anyone asked:\n%s", got)
	}

	press(t, program, "H")

	if !view.Kernel() {
		t.Fatal("H did not turn them on")
	}

	if got := plain(program); !strings.Contains(got, "kthreadd") {
		t.Errorf("H turned them on and nothing appeared:\n%s", got)
	}

	press(t, program, "H")

	if got := plain(program); strings.Contains(got, "kthreadd") {
		t.Errorf("H did not turn them back off:\n%s", got)
	}
}

func TestTheKernelToggleLeavesTheFilterAlone(t *testing.T) {
	t.Parallel()

	c := settings(t)

	program, _, view := fixtureWith(t, func(env *app.Env) { env.Config = c })

	const src = `user == "hayao"`

	expr, err := filter.Compile(src)
	if err != nil {
		t.Fatal(err)
	}

	view.SetFilter(src, expr)

	mode(t, program, "Processes")
	press(t, program, "H")

	if got, _ := view.Filter(); got != src {
		t.Errorf("the filter became %q", got)
	}

	if got := plain(program); !strings.Contains(got, "filter:") {
		t.Errorf("the filter is no longer in force:\n%s", got)
	}
}

func TestAZombieIsNotTakenForAKernelThread(t *testing.T) {
	t.Parallel()

	program, memory, _ := fixtureWith(t, func(env *app.Env) { env.Config = settings(t) })

	memory.WriteProcesses([]procmodel.Process{
		{PID: 1, Name: "systemd", User: "root", Cmdline: "/sbin/init"},
		{PID: 2, PPID: 0, Name: "kthreadd", User: "root", Kthread: true},
		{PID: 99, PPID: 1, Name: "gone", User: "hayao", State: "Z"},
	})

	mode(t, program, "Processes")

	got := plain(program)
	if !strings.Contains(got, "gone") {
		t.Errorf("the zombie was hidden along with the kernel threads:\n%s", got)
	}

	if strings.Contains(got, "kthreadd") {
		t.Errorf("the kernel thread is still drawn:\n%s", got)
	}
}
