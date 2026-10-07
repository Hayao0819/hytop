//go:build linux

package proc

import (
	"bytes"
	"strings"
	"unicode"
)

func cgroupInfo(contents []byte) (first, runtime, key string) {
	for line := range bytes.SplitSeq(contents, []byte{'\n'}) {
		_, rest, ok := bytes.Cut(line, []byte{':'})
		if !ok {
			continue
		}
		_, rawPath, ok := bytes.Cut(rest, []byte{':'})
		if !ok {
			continue
		}

		path := string(rawPath)
		if first == "" {
			first = path
		}
		if runtime == "" {
			if found, id := containerPath(path); id != "" {
				runtime, key = found, found+":"+shortID(id)
			}
		}
	}

	return first, runtime, key
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
