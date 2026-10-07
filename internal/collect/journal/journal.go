// Package journal reads systemd journal entries through journalctl.
package journal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/safe"
	"github.com/Hayao0819/hytop/internal/domain/unitmodel"
	"github.com/Hayao0819/hytop/internal/errors"
)

// Reader follows one unit. Closing its context stops the child process.
type Reader struct {
	mu      sync.Mutex
	entries []unitmodel.LogEntry
	err     error
	limit   int
}

func NewReader(limit int) *Reader {
	if limit <= 0 {
		limit = 200
	}

	return &Reader{limit: limit}
}

// Follow streams journalctl entries for unit until ctx is canceled.
func (r *Reader) Follow(ctx context.Context, unit string) {
	args := []string{"--output=json", "--no-pager", "--lines", strconv.Itoa(r.limit), "--follow"}
	if unit != "" {
		args = append(args, "--unit", unit)
	}

	commandCtx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(commandCtx, "journalctl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stop()
		r.fail(errors.Wrap(err, "opening journalctl"))

		return
	}

	if err := cmd.Start(); err != nil {
		stop()
		r.fail(errors.Wrap(err, "starting journalctl"))

		return
	}

	go r.consume(ctx, stdout, stop, func() error {
		defer stop()
		if err := cmd.Wait(); err != nil {
			if message := strings.TrimSpace(stderr.String()); message != "" {
				return errors.New(message)
			}
			return err
		}

		return nil
	})
}

func (r *Reader) consume(ctx context.Context, source io.Reader, stop func(), wait func() error) {
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		if entry, ok := parse(scanner.Bytes()); ok {
			r.add(entry)
		}
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		stop()
	}
	waitErr := wait()
	if ctx.Err() != nil {
		return
	}
	if scanErr != nil {
		r.fail(errors.Wrap(scanErr, "reading journalctl"))
	} else if waitErr != nil {
		r.fail(errors.Wrap(waitErr, "running journalctl"))
	}
}

func (r *Reader) add(entry unitmodel.LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)

	if len(r.entries) > r.limit {
		r.entries = r.entries[len(r.entries)-r.limit:]
	}
}

func (r *Reader) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.err = err
}

// Entries are the newest lines last.
func (r *Reader) Entries() []unitmodel.LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.entries)
}

func (r *Reader) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.err
}

// raw is the subset of journalctl's JSON a pane needs. Its fields can be a
// string or an array of them, so they are decoded loosely.
type raw struct {
	Timestamp string          `json:"__REALTIME_TIMESTAMP"`
	Priority  string          `json:"PRIORITY"`
	Unit      string          `json:"_SYSTEMD_UNIT"`
	Message   json.RawMessage `json:"MESSAGE"`
}

func parse(line []byte) (unitmodel.LogEntry, bool) {
	var r raw

	if err := json.Unmarshal(line, &r); err != nil {
		return unitmodel.LogEntry{}, false
	}

	entry := unitmodel.LogEntry{Unit: safe.Text(r.Unit), Priority: 6, Message: safe.Text(message(r.Message))}

	if micros, err := strconv.ParseInt(r.Timestamp, 10, 64); err == nil {
		entry.Time = time.UnixMicro(micros)
	}

	if priority, err := strconv.Atoi(r.Priority); err == nil {
		entry.Priority = priority
	}

	return entry, entry.Message != ""
}

// message accepts both forms journalctl emits: a string, or the byte array it
// falls back to when the payload is not valid UTF-8.
func message(payload json.RawMessage) string {
	var text string
	if err := json.Unmarshal(payload, &text); err == nil {
		return text
	}

	var bytes []byte
	if err := json.Unmarshal(payload, &bytes); err == nil {
		return strings.TrimRight(string(bytes), "\n")
	}

	return ""
}
