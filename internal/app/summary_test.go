package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/reactea/v2"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
	"github.com/Hayao0819/hytop/internal/ui/page"
)

func unitReadings(failed, jobs float64) []metric.Sample {
	now := time.Now()

	samples := []metric.Sample{
		{Key: "units.failed", Value: failed, Time: now},
		{Key: "units.jobs", Value: jobs, Time: now},
	}

	for key, value := range map[series.Key]float64{
		"units.total":   627,
		"units.running": 63,
		"units.active":  470,
		"units.service": 181,
		"units.timer":   10,
		"units.socket":  40,
	} {
		samples = append(samples, metric.Sample{Key: key, Value: value, Time: now})
	}

	return samples
}

func services(t *testing.T, failed, jobs float64, state string) string {
	t.Helper()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Store.WriteSamples(unitReadings(failed, jobs))
		env.Store.WriteFacts(map[string]string{
			series.FactUnitsState: state,
			series.FactUnitsBoot:  "18.189s",
		})
	})

	mode(t, program, "Services")

	return plain(program)
}

func TestTheServiceHeadingCountsWhatSystemdIsRunning(t *testing.T) {
	t.Parallel()

	got := services(t, 0, 0, "running")

	for _, want := range []string{
		"627 units", "63 running", "470 active",
		"181 services", "10 timers", "40 sockets",
		"systemd running", "userspace came up in 18.189s",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the service heading is missing %q:\n%s", want, got)
		}
	}
}

func TestTheServiceHeadingNamesFailedUnitsEvenAtZero(t *testing.T) {
	t.Parallel()

	if got := services(t, 0, 0, "running"); !strings.Contains(got, "0 failed") {
		t.Errorf("a machine with nothing wrong should still say so:\n%s", got)
	}

	if got := services(t, 3, 0, "degraded"); !strings.Contains(got, "3 failed") {
		t.Errorf("the service heading is missing the failed count:\n%s", got)
	}
}

func TestTheServiceHeadingRepeatsSystemdsVerdict(t *testing.T) {
	t.Parallel()

	if got := services(t, 3, 0, "degraded"); !strings.Contains(got, "systemd degraded") {
		t.Errorf("the service heading should say the machine is degraded:\n%s", got)
	}
}

func TestTheServiceHeadingShowsAQueueThatHasNotDrained(t *testing.T) {
	t.Parallel()

	if got := services(t, 0, 4, "starting"); !strings.Contains(got, "4 jobs queued") {
		t.Errorf("the service heading is missing the job queue:\n%s", got)
	}
}

func diskHeading(t *testing.T, list []diskmodel.Device) string {
	t.Helper()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = func() []diskmodel.Device { return list }
		env.Scanner = func() page.Scanner { return &stubScanner{} }

		now := time.Now()

		env.Store.WriteSamples([]metric.Sample{
			{Key: "fs.total.used", Value: 440 << 30, Time: now},
			{Key: "fs.total.size", Value: 501 << 30, Time: now},
		})
	})

	mode(t, program, "Disk")

	return plain(program)
}

func TestTheDiskHeadingBreaksTheDrivesDownToTheirTotal(t *testing.T) {
	t.Parallel()

	got := diskHeading(t, drives())

	for _, want := range []string{"2 drives", "1 solid state", "1 spinning", "2 filesystems"} {
		if !strings.Contains(got, want) {
			t.Errorf("the disk heading is missing %q:\n%s", want, got)
		}
	}
}

func TestTheDiskHeadingLeavesOutAKindTheMachineHasNot(t *testing.T) {
	t.Parallel()

	if got := diskHeading(t, drives()); strings.Contains(got, "optical") {
		t.Errorf("a machine with no optical drive should not mention one:\n%s", got)
	}

	list := append(drives(), diskmodel.Device{Name: "sr0", Optical: true, Rotating: true})

	got := diskHeading(t, list)

	if !strings.Contains(got, "3 drives") || !strings.Contains(got, "1 optical") {
		t.Errorf("an optical drive should be counted once, as optical:\n%s", got)
	}

	if !strings.Contains(got, "1 spinning") {
		t.Errorf("the optical drive was counted twice:\n%s", got)
	}
}

func TestTheDiskHeadingNamesTheFullestFilesystem(t *testing.T) {
	t.Parallel()

	got := diskHeading(t, drives())

	if !strings.Contains(got, "fullest /") {
		t.Errorf("the disk heading should name the fullest mount:\n%s", got)
	}

	if !strings.Contains(got, "87.8 %") {
		t.Errorf("the disk heading is missing the storage share:\n%s", got)
	}
}

func TestTheDiskHeadingSeparatesUnreadSmartFromHealthy(t *testing.T) {
	t.Parallel()

	if got := diskHeading(t, drives()); !strings.Contains(got, "SMART unread on 1 drive") {
		t.Errorf("a drive that was never asked should be called unread:\n%s", got)
	}

	healthy := drives()
	for i := range healthy {
		healthy[i].Health = diskmodel.Passed
	}

	if got := diskHeading(t, healthy); !strings.Contains(got, "SMART all passing") {
		t.Errorf("drives that all answered should say so:\n%s", got)
	}

	failing := drives()
	failing[0].Health = diskmodel.Failing
	failing[1].Health = diskmodel.Passed

	if got := diskHeading(t, failing); !strings.Contains(got, "1 drive failing SMART") {
		t.Errorf("a failing drive outranks everything else in the heading:\n%s", got)
	}
}

func TestTheDiskHeadingStaysPutAcrossThePanes(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = drives
		env.Scanner = func() page.Scanner { return &stubScanner{} }
	})

	mode(t, program, "Disk")

	first := heading(program)

	press(t, program, "j")

	if got := heading(program); got != first {
		t.Errorf("the heading changed when the pane did:\n%s\n%s", first, got)
	}
}

func heading(program *reactea.App) string {
	lines := strings.Split(plain(program), "\n")
	if len(lines) < 6 {
		return ""
	}

	return strings.Join(lines[3:5], "\n")
}
