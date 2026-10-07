//go:build linux

package proc

import (
	"testing"
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
			path, runtime, key := cgroupInfo([]byte("0::" + test.path + "\n"))
			if path != test.path {
				t.Fatalf("cgroupInfo(%q) path = %q", test.path, path)
			}
			if runtime != test.runtime || key != test.key {
				t.Fatalf("cgroupInfo(%q) = %q, %q", test.path, runtime, key)
			}
		})
	}
}

func TestCgroupInfoUsesFirstPathAndFindsContainerInLaterHierarchy(t *testing.T) {
	t.Parallel()

	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	contents := []byte("2:cpu:/system.slice/work.service\n1:memory:/docker/" + id + "\n")

	path, runtime, key := cgroupInfo(contents)
	if path != "/system.slice/work.service" {
		t.Fatalf("path = %q", path)
	}
	if runtime != "docker" || key != "docker:0123456789ab" {
		t.Fatalf("container = %q, %q", runtime, key)
	}
}
