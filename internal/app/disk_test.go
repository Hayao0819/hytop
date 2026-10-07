package app_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Hayao0819/reactea/v2"
	"github.com/Hayao0819/reactea/v2/testkit"

	"github.com/Hayao0819/hytop/internal/app"
	"github.com/Hayao0819/hytop/internal/domain/diskmodel"
	"github.com/Hayao0819/hytop/internal/ui/page"
)

func mounts() []diskmodel.Mount {
	return []diskmodel.Mount{
		{
			Path: "/", Device: "/dev/nvme0n1p2", FSType: "btrfs", Opts: "rw,relatime,ssd",
			Total: 500 << 30, Free: 60 << 30, Available: 50 << 30,
		},
		{
			Path: "/boot/efi", Device: "/dev/nvme0n1p1", FSType: "vfat", Opts: "rw",
			Total: 1 << 30, Free: 900 << 20, Available: 900 << 20,
			Inodes: 1000, InodesFree: 900,
		},
	}
}

func drives() []diskmodel.Device {
	return []diskmodel.Device{
		{
			Name: "nvme0n1", Model: "CT1000P3PSSD8", Bus: "NVMe", Size: 1 << 40,
			Health: diskmodel.Passed, Temperature: 41, Wear: 3,
			Attributes: []diskmodel.Attribute{
				{ID: 5, Name: "Reallocated Sector Ct", Value: 100, Worst: 100, Threshold: 10, Raw: "0"},
			},
		},
		{
			Name: "sda", Model: "HGST HDS724040AL", Bus: "SATA/SAS", Rotating: true,
			Size: 4 << 40, Wear: -1,
			Reason: "reading SMART needs privilege: run hytop as root",
		},
	}
}

type stubScanner struct {
	mu         sync.Mutex
	scan       diskmodel.Scan
	removed    []string
	restarts   int
	fail       error
	elevateErr error
	password   string
	noElevate  bool
}

func (s *stubScanner) Start(_ context.Context, root string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scan = diskmodel.Scan{
		Root: root,
		Done: true,
		Entries: []diskmodel.Entry{
			{Name: ".cache", Path: root + "/.cache", Size: 8 << 30, Items: 40000, Dir: true},
			{Name: "Videos", Path: root + "/Videos", Size: 2 << 30, Items: 12, Dir: true},
			{Name: "notes.txt", Path: root + "/notes.txt", Size: 4 << 10, Items: 1},
		},
		Total: 10 << 30,
		Items: 40013,
	}

	kept := s.scan.Entries[:0]
	for _, entry := range s.scan.Entries {
		removed := false
		for _, path := range s.removed {
			if entry.Path == path {
				removed = true
				break
			}
		}
		if !removed {
			kept = append(kept, entry)
		}
	}
	s.scan.Entries = kept
}

func (s *stubScanner) Restart(ctx context.Context, root string) {
	s.mu.Lock()
	s.restarts++
	s.mu.Unlock()

	s.Start(ctx, root)
}

func (s *stubScanner) Stop() {}

func (s *stubScanner) Scan() diskmodel.Scan {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.scan
}

func (s *stubScanner) Running() bool { return false }

func (s *stubScanner) CanElevate() bool { return !s.noElevate }

func cachePath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return home + "/.cache"
}

func (s *stubScanner) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail != nil {
		return s.fail
	}

	s.removed = append(s.removed, path)

	return nil
}

func (s *stubScanner) RemoveElevated(_ context.Context, path, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.password = password
	if s.elevateErr != nil {
		return s.elevateErr
	}
	s.removed = append(s.removed, path)

	return nil
}

func (s *stubScanner) RestartElevated(ctx context.Context, root, password string) error {
	s.mu.Lock()
	s.password = password
	err := s.elevateErr
	s.mu.Unlock()
	if err != nil {
		return err
	}

	s.Restart(ctx, root)
	return nil
}

func storage(t *testing.T, scanner page.Scanner) *reactea.App {
	t.Helper()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = drives
		env.Scanner = func() page.Scanner { return scanner }
	})

	mode(t, program, "Disk")

	return program
}

func TestTheFilesystemPageShowsWhatIsMounted(t *testing.T) {
	t.Parallel()

	program := storage(t, &stubScanner{})

	got := plain(program)

	for _, want := range []string{"/boot/efi", "btrfs", "vfat", "440.0 GiB", "/dev/nvme0n1p2"} {
		if !strings.Contains(got, want) {
			t.Errorf("the filesystem page is missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "0 of 0") {
		t.Errorf("a filesystem with no inode table should say so:\n%s", got)
	}
}

func TestTheFilesystemTableKeepsRowsTogetherInANarrowWindow(t *testing.T) {
	t.Parallel()

	program := storage(t, &stubScanner{})
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	got := plain(program)
	for _, line := range strings.Split(got, "\n") {
		if !strings.Contains(line, "/boot/efi") {
			continue
		}

		if !strings.Contains(line, "MiB") || !strings.Contains(line, "%") {
			t.Fatalf("the narrow row split its value from its mount:\n%s", got)
		}

		return
	}

	t.Fatalf("the narrow table lost /boot/efi:\n%s", got)
}

func TestTheDrivePageSaysWhySmartIsMissing(t *testing.T) {
	t.Parallel()

	program := storage(t, &stubScanner{})

	press(t, program, "]")

	got := plain(program)

	for _, want := range []string{"nvme0n1", "CT1000P3PSSD8", "SSD", "passed", "Reallocated Sector Ct"} {
		if !strings.Contains(got, want) {
			t.Errorf("the drive page is missing %q:\n%s", want, got)
		}
	}

	press(t, program, "l", "j")

	if got := plain(program); !strings.Contains(got, "needs privilege") {
		t.Errorf("a drive that could not be read should say why:\n%s", got)
	}
}

func TestTheUsagePageWalksAndDeletes(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)

	press(t, program, "]", "]", "l")

	got := plain(program)

	for _, want := range []string{".cache/", "Videos/", "notes.txt", "8.0 GiB"} {
		if !strings.Contains(got, want) {
			t.Errorf("the usage page is missing %q:\n%s", want, got)
		}
	}

	press(t, program, "enter")

	if root := scanner.Scan().Root; !strings.HasSuffix(root, "/.cache") {
		t.Errorf("root = %q, want the directory that was opened", root)
	}
}

func TestDeletingUsesAModalAndRefreshesTheScan(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)

	press(t, program, "]", "]", "l", "D")

	if len(scanner.removed) != 0 {
		t.Fatalf("one press deleted %v; it should only have asked", scanner.removed)
	}

	if got := plain(program); !strings.Contains(got, "Delete permanently?") ||
		!strings.Contains(got, cachePath(t)) || !strings.Contains(got, "enter delete") {
		t.Errorf("the delete dialog should identify its target and confirmation key:\n%s", got)
	}

	press(t, program, "enter")

	if len(scanner.removed) != 1 {
		t.Fatalf("the second press deleted %v", scanner.removed)
	}

	if !strings.HasSuffix(scanner.removed[0], "/.cache") {
		t.Errorf("deleted %q, which is not what was selected", scanner.removed[0])
	}

	if scanner.restarts != 1 {
		t.Errorf("delete refreshed with %d full rescans, want one", scanner.restarts)
	}

	if got := plain(program); strings.Contains(got, ".cache/") || !strings.Contains(got, "deleted "+cachePath(t)) {
		t.Errorf("the refreshed usage view still shows the deleted row:\n%s", got)
	}
}

func TestCancellingTheDeleteModalLeavesTheEntryAlone(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)

	press(t, program, "]", "]", "l", "D", "esc")

	if len(scanner.removed) != 0 {
		t.Errorf("cancelling the modal deleted %v", scanner.removed)
	}

	if got := plain(program); !strings.Contains(got, ".cache/") || strings.Contains(got, "Delete permanently?") {
		t.Errorf("cancel did not return to the unchanged usage view:\n%s", got)
	}
}

func TestPermissionDeniedDeleteCanAuthenticateAndRefresh(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{fail: os.ErrPermission}
	program := storage(t, scanner)
	press(t, program, "]", "]", "l", "D", "enter")

	if got := plain(program); !strings.Contains(got, "Administrator authentication") ||
		!strings.Contains(got, cachePath(t)) {
		t.Fatalf("permission failure did not open authentication:\n%s", got)
	}

	press(t, program, "s", "e", "c", "r", "e", "t")
	if got := plain(program); strings.Contains(got, "secret") || !strings.Contains(got, "••••••") {
		t.Fatalf("the password was not masked:\n%s", got)
	}

	press(t, program, "enter")

	if scanner.password != "secret" || len(scanner.removed) != 1 || scanner.restarts != 1 {
		t.Errorf("elevated delete password=%q removed=%v restarts=%d",
			scanner.password, scanner.removed, scanner.restarts)
	}
}

func TestPermissionDeniedScanOffersAndUsesElevation(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)
	press(t, program, "]", "]", "l")
	scanner.mu.Lock()
	scanner.scan.Err = "scanning /root: permission denied"
	scanner.mu.Unlock()

	if got := plain(program); !strings.Contains(got, "press e to retry as administrator") {
		t.Fatalf("the inaccessible scan did not offer elevation:\n%s", got)
	}

	press(t, program, "e", "p", "w", "enter")

	if scanner.password != "pw" || scanner.restarts != 1 {
		t.Errorf("elevated scan password=%q restarts=%d", scanner.password, scanner.restarts)
	}
}

func TestFailedElevationIsReported(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{elevateErr: errors.New("authentication failed")}
	program := storage(t, scanner)
	press(t, program, "]", "]", "l", "e", "p", "w", "enter")

	if got := plain(program); !strings.Contains(got, "authentication failed") {
		t.Fatalf("elevation failure is not visible:\n%s", got)
	}
	if scanner.restarts != 0 {
		t.Errorf("failed elevation restarted the scanner %d times", scanner.restarts)
	}
}

func TestUnavailableElevationDoesNotOpenAPasswordDialog(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{noElevate: true}
	program := storage(t, scanner)
	press(t, program, "]", "]", "l")
	scanner.mu.Lock()
	scanner.scan.Err = "scanning a directory: permission denied"
	scanner.mu.Unlock()

	press(t, program, "e")
	got := plain(program)
	if strings.Contains(got, "Administrator authentication") || !strings.Contains(got, "retry is unavailable") {
		t.Fatalf("unavailable elevation opened or advertised authentication:\n%s", got)
	}
}

func TestTheRailMovesWithTheSameKeysAsTheGraphRail(t *testing.T) {
	t.Parallel()

	program := storage(t, &stubScanner{})

	press(t, program, "j")

	if got := program.Route(); got != "/disk/drives" {
		t.Errorf("route = %q, want j to have stepped the rail", got)
	}

	press(t, program, "k")

	if got := program.Route(); got != "/disk/filesystems" {
		t.Errorf("route = %q, want k to have stepped back", got)
	}

	press(t, program, "l", "j")

	if got := program.Route(); got != "/disk/filesystems" {
		t.Errorf("route = %q; j moved the rail from inside a pane", got)
	}

	press(t, program, "h", "j")

	if got := program.Route(); got != "/disk/drives" {
		t.Errorf("route = %q; h did not give the keys back to the rail", got)
	}
}

func TestClickingTheDiskRailOpensAndFocusesIt(t *testing.T) {
	t.Parallel()

	program := storage(t, &stubScanner{})
	press(t, program, "l")

	var (
		x, y  int
		found bool
	)
	for row, line := range strings.Split(plain(program), "\n") {
		if column := strings.Index(line, "Drives"); column >= 0 {
			x, y, found = column, row, true
			break
		}
	}
	if !found {
		t.Fatalf("no Drives entry in the disk rail:\n%s", plain(program))
	}

	click(t, program, x, y)
	if got := program.Route(); got != "/disk/drives" {
		t.Fatalf("route = %q, want the clicked rail entry", got)
	}

	press(t, program, "j")
	if got := program.Route(); got != "/disk/usage" {
		t.Errorf("route = %q; clicking the rail did not give it focus", got)
	}
}

func TestOpeningAMountCountsWhatIsUnderIt(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)

	press(t, program, "l", "j", "enter")

	if got := program.Route(); got != "/disk/usage" {
		t.Fatalf("route = %q, want the usage pane", got)
	}

	if root := scanner.Scan().Root; root != "/boot/efi" {
		t.Errorf("root = %q, want the mount that was chosen", root)
	}
}

func TestClickingAMountOpensIt(t *testing.T) {
	t.Parallel()

	scanner := &stubScanner{}
	program := storage(t, scanner)

	if !testkit.ClickText(program, "/boot/efi") {
		t.Fatalf("no row for /boot/efi:\n%s", plain(program))
	}

	if got := program.Route(); got != "/disk/usage" {
		t.Fatalf("route = %q, want the click to have opened the mount", got)
	}

	if root := scanner.Scan().Root; root != "/boot/efi" {
		t.Errorf("root = %q, want the row that was clicked", root)
	}
}

func TestDriveListScrollsToKeepSelectionVisible(t *testing.T) {
	t.Parallel()

	list := make([]diskmodel.Device, 12)
	for i := range list {
		list[i] = diskmodel.Device{Name: fmt.Sprintf("disk%02d", i), Model: "model", Wear: -1}
	}

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = func() []diskmodel.Device { return list }
	})
	mode(t, program, "Disk")
	press(t, program, "]", "l")
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 11})
	press(t, program, "G")

	got := plain(program)
	if !strings.Contains(got, "disk11") {
		t.Fatalf("last selected drive is outside the viewport:\n%s", got)
	}
	if strings.Contains(got, "disk00") {
		t.Fatalf("drive viewport did not move from the first row:\n%s", got)
	}
}

func TestPermissionDeniedSMARTCanAuthenticateAndRefresh(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	list := []diskmodel.Device{{Name: "sda", Reason: "reading SMART needs privilege", Wear: -1}}
	password := ""

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = func() []diskmodel.Device {
			mu.Lock()
			defer mu.Unlock()

			return append([]diskmodel.Device(nil), list...)
		}
		env.RefreshDrives = func(_ context.Context, got string) error {
			mu.Lock()
			defer mu.Unlock()

			password = got
			list = []diskmodel.Device{{Name: "sda", Health: diskmodel.Passed, Wear: -1}}

			return nil
		}
	})
	mode(t, program, "Disk")
	press(t, program, "]", "l", "e")

	if got := plain(program); !strings.Contains(got, "Administrator authentication") {
		t.Fatalf("SMART elevation did not ask for authentication:\n%s", got)
	}
	press(t, program, "p", "w", "enter")

	mu.Lock()
	gotPassword := password
	mu.Unlock()
	if gotPassword != "pw" {
		t.Errorf("SMART elevation password = %q", gotPassword)
	}
	if got := plain(program); !strings.Contains(got, "SMART information refreshed") ||
		!strings.Contains(got, "passed") {
		t.Errorf("refreshed SMART information was not displayed:\n%s", got)
	}
}

func TestFailedSMARTRefreshIsReported(t *testing.T) {
	t.Parallel()

	program, _, _ := fixtureWith(t, func(env *app.Env) {
		env.Config = settings(t)
		env.Mounts = mounts
		env.Drives = func() []diskmodel.Device {
			return []diskmodel.Device{{Name: "sda", Reason: "reading SMART needs privilege", Wear: -1}}
		}
		env.RefreshDrives = func(context.Context, string) error {
			return errors.New("authorization denied")
		}
	})
	mode(t, program, "Disk")
	press(t, program, "]", "l", "e", "p", "w", "enter")

	if got := plain(program); !strings.Contains(got, "authorization denied") {
		t.Fatalf("SMART refresh failure is not visible:\n%s", got)
	}
}
