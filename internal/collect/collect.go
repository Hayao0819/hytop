// Package collect gathers metric samples and textual facts.
package collect

import (
	"context"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/procmodel"
)

type State int

const (
	Ready State = iota
	NeedsPrivilege
	NoKernelSupport
	NoHardware
	Failed
)

// Availability describes whether a collector can run and how to resolve an
// unavailable state.
type Availability struct {
	State  State
	Reason string
	Remedy string
}

func (a Availability) OK() bool { return a.State == Ready }

// KernelAvailability turns a probe error into the common collector status.
func KernelAvailability(err error, remedy string) Availability {
	if err == nil {
		return Availability{State: Ready}
	}

	return Availability{State: NoKernelSupport, Reason: err.Error(), Remedy: remedy}
}

type Collector interface {
	Name() string
	Check() Availability
	// Collect must stop promptly when its context is canceled.
	Collect(context.Context, time.Time) ([]metric.Sample, error)
}

// Facts contains textual, low-frequency collector values.
type Facts map[string]string

// FactCollector reports facts as well as samples.
type FactCollector interface {
	Collector

	// Facts must stop promptly when its context is canceled.
	Facts(context.Context) (Facts, error)
}

// StaticFactCollector marks facts that need no refresh after a successful read.
type StaticFactCollector interface {
	FactCollector
	StaticFacts()
}

// ProcessCollector also builds the process table.
type ProcessCollector interface {
	Collector

	Processes(context.Context) ([]procmodel.Process, error)
}

// Sink is where collected values go.
type Sink interface {
	WriteSamples(samples []metric.Sample)
	WriteProcesses(procs []procmodel.Process)
	ReplaceFacts(source string, facts map[string]string)
}
