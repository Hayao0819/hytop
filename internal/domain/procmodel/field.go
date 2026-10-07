package procmodel

import (
	"maps"
	"slices"
)

type FieldKind uint8

const (
	NumberField FieldKind = iota
	StringField
	BoolField
)

type Field struct {
	Key     string
	Kind    FieldKind
	Text    func(*Process) string
	Bool    func(*Process) bool
	Numbers func(*Process) []float64
}

var processFields = map[string]Field{
	"pid":       number("pid", func(p *Process) float64 { return float64(p.PID) }),
	"ppid":      number("ppid", func(p *Process) float64 { return float64(p.PPID) }),
	"pgid":      number("pgid", func(p *Process) float64 { return float64(p.PGID) }),
	"uid":       number("uid", func(p *Process) float64 { return float64(p.UID) }),
	"nice":      number("nice", func(p *Process) float64 { return float64(p.Nice) }),
	"threads":   number("threads", func(p *Process) float64 { return float64(p.Threads) }),
	"fds":       number("fds", func(p *Process) float64 { return float64(p.FDs) }),
	"cpu":       number("cpu", func(p *Process) float64 { return p.CPU }),
	"rss":       number("rss", func(p *Process) float64 { return float64(p.RSS) }),
	"vsz":       number("vsz", func(p *Process) float64 { return float64(p.VSZ) }),
	"io.read":   number("io.read", func(p *Process) float64 { return float64(p.IORd) }),
	"io.write":  number("io.write", func(p *Process) float64 { return float64(p.IOWr) }),
	"gpu.util":  number("gpu.util", func(p *Process) float64 { return p.GPUPc }),
	"gpu.mem":   number("gpu.mem", func(p *Process) float64 { return float64(p.GPUMB) }),
	"npu.util":  number("npu.util", func(p *Process) float64 { return p.NPUPc }),
	"name":      text("name", func(p *Process) string { return p.Name }),
	"cmdline":   text("cmdline", func(p *Process) string { return p.Cmdline }),
	"command":   text("command", Command),
	"exe":       text("exe", func(p *Process) string { return p.Exe }),
	"user":      text("user", func(p *Process) string { return p.User }),
	"state":     text("state", func(p *Process) string { return p.State }),
	"cgroup":    text("cgroup", func(p *Process) string { return p.Cgroup }),
	"unit":      text("unit", func(p *Process) string { return p.Unit }),
	"container": text("container", func(p *Process) string { return p.Container }),
	"vm":        text("vm", func(p *Process) string { return p.VM }),
	"kthread": {
		Key: "kthread", Kind: BoolField,
		Bool: func(p *Process) bool { return p.Kthread },
	},
	"port": {
		Key: "port", Kind: NumberField,
		Numbers: func(p *Process) []float64 {
			ports := make([]float64, len(p.Ports))
			for i, port := range p.Ports {
				ports[i] = float64(port)
			}

			return ports
		},
	},
}

func Command(p *Process) string {
	if p.Cmdline != "" {
		return p.Cmdline
	}

	return "[" + p.Name + "]"
}

func number(key string, read func(*Process) float64) Field {
	return Field{Key: key, Kind: NumberField, Numbers: func(p *Process) []float64 {
		return []float64{read(p)}
	}}
}

func text(key string, read func(*Process) string) Field {
	return Field{Key: key, Kind: StringField, Text: read}
}

func Fields() []string {
	return slices.Sorted(maps.Keys(processFields))
}

func LookupField(key string) (Field, bool) {
	field, ok := processFields[key]

	return field, ok
}
