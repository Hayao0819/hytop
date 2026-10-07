// Package diskmodel defines filesystem, drive, and directory-scan data.
package diskmodel

import "time"

// Mount is one mounted filesystem.
type Mount struct {
	Path   string
	Device string
	FSType string
	Opts   string

	Total     uint64
	Free      uint64
	Available uint64

	Inodes     uint64
	InodesFree uint64
}

func (m Mount) Used() uint64 { return m.Total - m.Free }

// Usage returns the percentage of user-available capacity in use.
func (m Mount) Usage() float64 {
	usable := m.Used() + m.Available
	if usable == 0 {
		return 0
	}

	return 100 * float64(m.Used()) / float64(usable)
}

func (m Mount) InodeUsage() float64 {
	if m.Inodes == 0 {
		return 0
	}

	return 100 * float64(m.Inodes-m.InodesFree) / float64(m.Inodes)
}

// Health is a drive's SMART health state.
type Health int

const (
	Unknown Health = iota
	Passed
	Failing
)

func (h Health) String() string {
	switch h {
	case Passed:
		return "passed"
	case Failing:
		return "FAILING"
	default:
		return "unknown"
	}
}

// Device combines sysfs metadata and optional SMART data for one drive.
type Device struct {
	Name   string
	Model  string
	Serial string
	Vendor string
	Bus    string

	Size     uint64
	Rotating bool
	Optical  bool
	Sched    string

	// Reason explains unavailable SMART fields.
	Reason string

	Health      Health
	Temperature float64
	PowerOnTime time.Duration
	PowerCycles uint64
	Firmware    string

	// Wear is consumed endurance in percent, or -1 when unavailable.
	Wear float64

	Written uint64
	Read    uint64

	Attributes []Attribute
}

// Kind returns the display category for the drive.
func (d Device) Kind() string {
	switch {
	case d.Optical:
		return "Optical"
	case d.Rotating:
		return "HDD"
	default:
		return "SSD"
	}
}

// Attribute is one decoded SMART attribute.
type Attribute struct {
	ID        int
	Name      string
	Value     int
	Worst     int
	Threshold int
	Raw       string
	Failing   bool
}

// Entry is one directory child with its recursive usage.
type Entry struct {
	Name  string
	Path  string
	Size  uint64
	Items int
	Dir   bool

	// Mount reports a nested filesystem excluded from this entry's total.
	Mount bool
}

// Scan is an incremental directory scan.
type Scan struct {
	Root    string
	Entries []Entry
	Total   uint64
	Items   int
	Done    bool
	Err     string

	// Reused counts directories answered from cached direct-file totals.
	Reused int
}
