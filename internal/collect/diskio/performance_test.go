package diskio

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Hayao0819/hytop/internal/domain/metric"
	"github.com/Hayao0819/hytop/internal/domain/series"
)

func TestPhysicalDiskRatesUseTheProviderClock(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	first := []performanceCounters{
		{Name: "0 C:", DiskReadBytesPerSec: 1000, DiskWriteBytesPerSec: 2000, PercentIdleTime: 2e7, PercentIdleTime_Base: 20e7, Timestamp_Sys100NS: 10e7},
		{Name: "1 D: E:", DiskReadBytesPerSec: 3000, DiskWriteBytesPerSec: 4000, PercentIdleTime: 3e7, PercentIdleTime_Base: 20e7, Timestamp_Sys100NS: 10e7},
		{Name: "_Total", DiskReadBytesPerSec: 4000, DiskWriteBytesPerSec: 6000, Timestamp_Sys100NS: 10e7},
	}
	if samples, _ := performanceSamples(nil, first, now); len(samples) != 0 {
		t.Fatalf("first samples = %v", samples)
	}
	_, previous := performanceSamples(nil, first, now)
	second := []performanceCounters{
		{Name: "0 C: F:", DiskReadBytesPerSec: 5000, DiskWriteBytesPerSec: 10000, PercentIdleTime: 3e7, PercentIdleTime_Base: 24e7, Timestamp_Sys100NS: 12e7},
		{Name: "1 D: E:", DiskReadBytesPerSec: 5000, DiskWriteBytesPerSec: 4000, PercentIdleTime: 7e7, PercentIdleTime_Base: 24e7, Timestamp_Sys100NS: 12e7},
		{Name: "_Total", DiskReadBytesPerSec: 10000, DiskWriteBytesPerSec: 14000, Timestamp_Sys100NS: 12e7},
	}
	stamp := now.Add(30 * time.Second)
	samples, _ := performanceSamples(previous, second, stamp)
	want := map[series.Key]float64{
		"diskio.disk0.read": 2000, "diskio.disk0.write": 4000, "diskio.disk0.util": 75,
		"diskio.disk1.read": 1000, "diskio.disk1.write": 0, "diskio.disk1.util": 0,
		"diskio.total.read": 3000, "diskio.total.write": 4000,
	}
	assertPerformanceSamples(t, samples, want, stamp)
}

func TestPhysicalDiskCounterResetsAndBounds(t *testing.T) {
	t.Parallel()

	previous := performanceCounters{
		Name: "0 C:", DiskReadBytesPerSec: 100, DiskWriteBytesPerSec: 200,
		PercentIdleTime: 1e7, PercentIdleTime_Base: 10e7, Timestamp_Sys100NS: 10e7,
	}
	current := previous
	current.Timestamp_Sys100NS += 1e7
	current.PercentIdleTime_Base += 1e7
	current.DiskReadBytesPerSec += 50
	current.DiskWriteBytesPerSec += 100
	tests := []struct {
		name   string
		change func(*performanceCounters)
		want   map[series.Key]float64
	}{
		{"busy", func(*performanceCounters) {}, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100, "diskio.disk0.util": 100,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"idle", func(c *performanceCounters) { c.PercentIdleTime += 1e7 }, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100, "diskio.disk0.util": 0,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"idle overshoot", func(c *performanceCounters) { c.PercentIdleTime += 2e7 }, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100, "diskio.disk0.util": 0,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"idle reset", func(c *performanceCounters) { c.PercentIdleTime = 0 }, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"idle clock repeated", func(c *performanceCounters) { c.PercentIdleTime_Base = previous.PercentIdleTime_Base }, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"idle clock reset", func(c *performanceCounters) { c.PercentIdleTime_Base = 0 }, map[series.Key]float64{
			"diskio.disk0.read": 50, "diskio.disk0.write": 100,
			"diskio.total.read": 50, "diskio.total.write": 100,
		}},
		{"read reset", func(c *performanceCounters) { c.DiskReadBytesPerSec = 0 }, nil},
		{"write reset", func(c *performanceCounters) { c.DiskWriteBytesPerSec = 0 }, nil},
		{"repeated timestamp", func(c *performanceCounters) { c.Timestamp_Sys100NS = previous.Timestamp_Sys100NS }, nil},
		{"clock regression", func(c *performanceCounters) { c.Timestamp_Sys100NS = previous.Timestamp_Sys100NS - 1 }, nil},
		{"missing timestamp", func(c *performanceCounters) { c.Timestamp_Sys100NS = 0 }, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := current
			test.change(&next)
			now := time.Unix(100, 0)
			samples, _ := performanceSamples(map[string]performanceCounters{"disk0": previous}, []performanceCounters{next}, now)
			assertPerformanceSamples(t, samples, test.want, now)
		})
	}
}

func TestPhysicalDiskKeysRemainStableAndDevicesExpire(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	first := []performanceCounters{
		{Name: "00 C:", PercentIdleTime_Base: 1e7, Timestamp_Sys100NS: 1e7},
		{Name: "0 C:", PercentIdleTime_Base: 1e7, Timestamp_Sys100NS: 1e7},
		{Name: "2", PercentIdleTime_Base: 1e7, Timestamp_Sys100NS: 1e7},
		{Name: "_Total", Timestamp_Sys100NS: 1e7},
		{Name: "invalid", Timestamp_Sys100NS: 1e7},
		{Name: "-1", Timestamp_Sys100NS: 1e7},
	}
	_, previous := performanceSamples(nil, first, now)
	if len(previous) != 2 {
		t.Fatalf("devices = %v", previous)
	}
	second := []performanceCounters{
		{Name: "0 C: D:", DiskReadBytesPerSec: 100, PercentIdleTime_Base: 2e7, Timestamp_Sys100NS: 2e7},
		{Name: "00 C:", DiskReadBytesPerSec: 100, PercentIdleTime_Base: 2e7, Timestamp_Sys100NS: 2e7},
		{Name: "1 E:", DiskReadBytesPerSec: 200, PercentIdleTime_Base: 2e7, Timestamp_Sys100NS: 2e7},
	}
	samples, previous := performanceSamples(previous, second, now)
	assertPerformanceSamples(t, samples, map[series.Key]float64{
		"diskio.disk0.read": 100, "diskio.disk0.write": 0, "diskio.disk0.util": 100,
		"diskio.total.read": 100, "diskio.total.write": 0,
	}, now)
	if _, exists := previous["disk2"]; exists {
		t.Fatal("removed disk retained its baseline")
	}
	_, previous = performanceSamples(previous, nil, now)
	samples, _ = performanceSamples(previous, second, now)
	if len(samples) != 0 {
		t.Fatalf("reappeared disks reused old counters: %v", samples)
	}
}

func TestRepeatedPhysicalDiskTimestampKeepsTheOriginalBaseline(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	_, previous := performanceSamples(nil, []performanceCounters{
		{Name: "0 C:", DiskReadBytesPerSec: 100, PercentIdleTime_Base: 1e7, Timestamp_Sys100NS: 1e7},
	}, now)
	samples, previous := performanceSamples(previous, []performanceCounters{
		{Name: "0 C:", DiskReadBytesPerSec: 150, PercentIdleTime_Base: 1e7, Timestamp_Sys100NS: 1e7},
	}, now)
	if len(samples) != 0 {
		t.Fatalf("repeated timestamp produced rates: %v", samples)
	}
	samples, _ = performanceSamples(previous, []performanceCounters{
		{Name: "0 C:", DiskReadBytesPerSec: 200, PercentIdleTime_Base: 2e7, Timestamp_Sys100NS: 2e7},
	}, now)
	assertPerformanceSamples(t, samples, map[series.Key]float64{
		"diskio.disk0.read": 100, "diskio.disk0.write": 0, "diskio.disk0.util": 100,
		"diskio.total.read": 100, "diskio.total.write": 0,
	}, now)
}

func TestPhysicalDiskQueryErrorsPreserveTheBaseline(t *testing.T) {
	t.Parallel()

	failure := errors.New("access denied")
	calls := 0
	collector := newPerformanceCollector(func() ([]performanceCounters, error) {
		calls++
		if calls == 2 {
			return nil, failure
		}
		return []performanceCounters{{Name: "0 C:", PercentIdleTime_Base: uint64(calls) * 1e7, Timestamp_Sys100NS: uint64(calls) * 1e7}}, nil
	})
	now := time.Unix(100, 0)
	if _, err := collector.Collect(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	baseline := collector.previous["disk0"]
	if samples, err := collector.Collect(t.Context(), now); !errors.Is(err, failure) || len(samples) != 0 {
		t.Fatalf("failed query = %v, %v", samples, err)
	}
	if collector.previous["disk0"] != baseline {
		t.Fatal("failed query replaced the baseline")
	}
	samples, err := collector.Collect(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	assertPerformanceSamples(t, samples, map[series.Key]float64{
		"diskio.disk0.read": 0, "diskio.disk0.write": 0, "diskio.disk0.util": 100,
		"diskio.total.read": 0, "diskio.total.write": 0,
	}, now)
}

func TestPhysicalDiskQueriesStayBoundedAfterCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	collector := newPerformanceCollector(func() ([]performanceCounters, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return []performanceCounters{{Name: "0 C:", Timestamp_Sys100NS: uint64(calls.Load()) * 1e7}}, nil
	})
	now := time.Unix(100, 0)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := collector.Collect(ctx, now)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("query did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled query = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Collect did not return after cancellation")
	}
	for range 3 {
		ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		_, err := collector.Collect(ctx, now)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("pending query = %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("started %d queries while WMI was blocked", calls.Load())
	}
	if len(collector.previous) != 0 {
		t.Fatal("canceled query published a baseline")
	}
}

func TestPhysicalDiskCollectionCanBeCanceledWhileAnotherCallWaits(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	collector := newPerformanceCollector(func() ([]performanceCounters, error) {
		close(started)
		<-release
		return nil, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := collector.Collect(ctx, time.Now())
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("query did not start")
	}
	limited, stop := context.WithTimeout(t.Context(), time.Millisecond)
	defer stop()
	if _, err := collector.Collect(limited, time.Now()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("concurrent collection = %v", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("first collection did not stop")
	}
}

func TestCanceledPhysicalDiskQueryIsDiscardedBeforeRecovery(t *testing.T) {
	t.Parallel()

	calls := 0
	collector := newPerformanceCollector(func() ([]performanceCounters, error) {
		calls++
		return []performanceCounters{{Name: "0 C:", Timestamp_Sys100NS: uint64(calls) * 1e7}}, nil
	})
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan performanceResult, 1)
	done <- performanceResult{counters: []performanceCounters{{Name: "9 Z:", Timestamp_Sys100NS: 1e7}}}
	collector.pending = &performanceQuery{ctx: canceled, done: done}
	if _, err := collector.Collect(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(collector.previous) != 1 || collector.previous["disk0"].Name != "0 C:" {
		t.Fatalf("recovered baseline = %v, queries = %d", collector.previous, calls)
	}
	if _, err := collector.Collect(canceled, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled collection = %v", err)
	}
	if calls != 1 {
		t.Fatalf("canceled collection started another query: %d", calls)
	}
}

func assertPerformanceSamples(t *testing.T, samples []metric.Sample, want map[series.Key]float64, stamp time.Time) {
	t.Helper()
	got := make(map[series.Key]float64)
	for _, sample := range samples {
		if _, duplicate := got[sample.Key]; duplicate {
			t.Errorf("duplicate sample %s", sample.Key)
		}
		if sample.Time != stamp || math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) {
			t.Errorf("invalid sample: %+v", sample)
		}
		got[sample.Key] = sample.Value
	}
	if len(got) != 0 || len(want) != 0 {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("samples = %v, want %v", got, want)
		}
	}
}
