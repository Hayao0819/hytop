// Package unitmodel defines systemd unit and journal data.
package unitmodel

import "time"

// Unit is one systemd unit snapshot.
type Unit struct {
	Name        string
	Description string
	Load        string
	Active      string
	Sub         string
	Following   string
	Path        string
}

// Failed reports whether loading or activation failed.
func (u Unit) Failed() bool { return u.Active == "failed" || u.Load == "error" }

// LogEntry is one journal entry reduced to display fields.
type LogEntry struct {
	Time     time.Time
	Priority int
	Unit     string
	Message  string
}

// Error reports syslog priorities 0 through 3.
func (e LogEntry) Error() bool { return e.Priority <= 3 }

// Warning reports syslog priority 4.
func (e LogEntry) Warning() bool { return e.Priority == 4 }
