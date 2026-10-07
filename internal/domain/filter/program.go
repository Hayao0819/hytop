package filter

import (
	"fmt"
	"strconv"

	exprlang "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

// programExpr embeds Expr predicates in the set-oriented process language.
// Tree relations remain hytop operations and can wrap this predicate.
type programExpr struct {
	source  string
	program *vm.Program
}

func compileProgram(source string) (Expr, error) {
	program, err := exprlang.Compile(source, exprlang.Env(processEnvironment{}), exprlang.AsBool())
	if err != nil {
		return nil, fmt.Errorf("filter: Expr: %w", err)
	}

	return &programExpr{source: source, program: program}, nil
}

func (e *programExpr) String() string { return "expr(" + strconv.Quote(e.source) + ")" }

func (e *programExpr) eval(s *scope) set {
	matched := make(set)

	for _, pid := range s.snapshot.PIDs() {
		process, ok := s.snapshot.Get(pid)
		if !ok {
			continue
		}

		value, err := exprlang.Run(e.program, newProcessEnvironment(process))
		if err != nil {
			s.err = fmt.Errorf("filter: Expr for PID %d: %w", pid, err)

			return matched
		}
		if value == true {
			matched.add(pid)
		}
	}

	return matched
}

type ioEnvironment struct {
	Read  uint64 `expr:"read"`
	Write uint64 `expr:"write"`
}

type gpuEnvironment struct {
	Util float64 `expr:"util"`
	Mem  uint64  `expr:"mem"`
}

type npuEnvironment struct {
	Util float64 `expr:"util"`
}

type processEnvironment struct {
	PID  int `expr:"pid"`
	PPID int `expr:"ppid"`
	PGID int `expr:"pgid"`

	Name    string `expr:"name"`
	Cmdline string `expr:"cmdline"`
	Exe     string `expr:"exe"`
	User    string `expr:"user"`
	UID     int64  `expr:"uid"`
	State   string `expr:"state"`
	Nice    int    `expr:"nice"`
	Threads int    `expr:"threads"`
	FDs     int    `expr:"fds"`
	Command string `expr:"command"`

	Cgroup    string `expr:"cgroup"`
	Unit      string `expr:"unit"`
	Container string `expr:"container"`
	VM        string `expr:"vm"`
	Kthread   bool   `expr:"kthread"`

	CPU  float64        `expr:"cpu"`
	RSS  uint64         `expr:"rss"`
	VSZ  uint64         `expr:"vsz"`
	IO   ioEnvironment  `expr:"io"`
	GPU  gpuEnvironment `expr:"gpu"`
	NPU  npuEnvironment `expr:"npu"`
	Port []int          `expr:"port"`
}

func newProcessEnvironment(p *procmodel.Process) processEnvironment {
	return processEnvironment{
		PID: p.PID, PPID: p.PPID, PGID: p.PGID,
		Name: p.Name, Cmdline: p.Cmdline, Exe: p.Exe,
		User: p.User, UID: p.UID, State: p.State,
		Nice: p.Nice, Threads: p.Threads, FDs: p.FDs, Command: procmodel.Command(p),
		Cgroup: p.Cgroup, Unit: p.Unit, Container: p.Container,
		VM: p.VM, Kthread: p.Kthread,
		CPU: p.CPU, RSS: p.RSS, VSZ: p.VSZ,
		IO:  ioEnvironment{Read: p.IORd, Write: p.IOWr},
		GPU: gpuEnvironment{Util: p.GPUPc, Mem: p.GPUMB},
		NPU: npuEnvironment{Util: p.NPUPc}, Port: p.Ports,
	}
}
