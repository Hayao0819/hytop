// Package version reports build metadata for the running binary.
package version

import (
	"runtime"
	"runtime/debug"
)

const (
	Repository = "https://github.com/Hayao0819/hytop"
	Developer  = "@Hayao0819"
	Twitter    = "https://twitter.com/Hayao0819"
)

// Release builds replace these values; Current fills gaps from Go build metadata.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
	Modified  bool
}

func Current() Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if build, ok := debug.ReadBuildInfo(); ok {
		info = supplement(info, build)
	}

	if missing(info.Version) || info.Version == "(devel)" {
		info.Version = "dev"
	}
	if missing(info.Commit) {
		info.Commit = "unknown"
	}
	if missing(info.Date) {
		info.Date = "unknown"
	}

	return info
}

func supplement(info Info, build *debug.BuildInfo) Info {
	if (missing(info.Version) || info.Version == "dev") && !missing(build.Main.Version) && build.Main.Version != "(devel)" {
		info.Version = build.Main.Version
	}

	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if missing(info.Commit) {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if missing(info.Date) {
				info.Date = setting.Value
			}
		case "vcs.modified":
			info.Modified = setting.Value == "true"
		}
	}

	return info
}

func missing(value string) bool { return value == "" || value == "unknown" }
