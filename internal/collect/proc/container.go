//go:build linux

package proc

import (
	"strings"
	"unicode"

	"github.com/prometheus/procfs"
)

// containerOf derives a stable container identity from kernel cgroup names
// without querying a runtime for every process.
func containerOf(groups []procfs.Cgroup) (runtime, key string) {
	for _, group := range groups {
		if runtime, id := containerPath(group.Path); id != "" {
			return runtime, runtime + ":" + shortID(id)
		}
	}

	return "", ""
}

func containerPath(path string) (runtime, id string) {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' }) {
		name := strings.TrimSuffix(part, ".scope")

		for _, prefix := range []struct {
			name    string
			runtime string
		}{
			{"docker-", "docker"},
			{"cri-containerd-", "containerd"},
			{"crio-", "cri-o"},
			{"libpod-", "podman"},
		} {
			if candidate := strings.TrimPrefix(name, prefix.name); candidate != name && containerID(candidate) {
				return prefix.runtime, candidate
			}
		}

		if containerID(name) {
			switch {
			case strings.Contains(path, "/docker/"):
				return "docker", name
			case strings.Contains(path, "/libpod"):
				return "podman", name
			case strings.Contains(path, "kubepods"):
				return "containerd", name
			}
		}

		if machine := strings.TrimPrefix(name, "machine-"); machine != name && machine != "" {
			return "nspawn", machine
		}

		if lxc := strings.TrimPrefix(name, "lxc-"); lxc != name && lxc != "" {
			return "lxc", lxc
		}
	}

	return "", ""
}

func containerID(text string) bool {
	if len(text) < 12 {
		return false
	}

	for _, r := range text {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}

	return true
}

func shortID(id string) string {
	if len(id) > 12 && containerID(id) {
		return strings.ToLower(id[:12])
	}

	return id
}
