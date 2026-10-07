//go:build linux

package proc

import (
	"testing"

	"github.com/prometheus/procfs"
)

func TestContainerIdentityFromCgroup(t *testing.T) {
	t.Parallel()

	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		path    string
		runtime string
		key     string
	}{
		{"/system.slice/docker-" + id + ".scope", "docker", "docker:0123456789ab"},
		{"/user.slice/libpod-" + id + ".scope", "podman", "podman:0123456789ab"},
		{"/kubepods.slice/cri-containerd-" + id + ".scope", "containerd", "containerd:0123456789ab"},
		{"/machine.slice/machine-buildbox.scope", "nspawn", "nspawn:buildbox"},
		{"/ordinary.service", "", ""},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			runtime, key := containerOf([]procfs.Cgroup{{Path: test.path}})
			if runtime != test.runtime || key != test.key {
				t.Fatalf("containerOf(%q) = %q, %q", test.path, runtime, key)
			}
		})
	}
}
