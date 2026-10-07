//go:build !windows

package conf

import "path/filepath"

func systemConfigPath() string { return filepath.Join("/etc", name, file) }
