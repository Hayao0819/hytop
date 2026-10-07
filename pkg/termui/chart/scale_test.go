package chart_test

import (
	"math"
	"testing"

	"github.com/Hayao0819/hytop/pkg/termui/chart"
)

func TestNiceMaxUsesReadableStepsAndHeadroom(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		floor  float64
		values []float64
		want   float64
	}{
		{name: "idle floor", floor: 1024, values: []float64{0, 0}, want: 2000},
		{name: "small traffic", floor: 0, values: []float64{900}, want: 1000},
		{name: "headroom crosses step", floor: 0, values: []float64{1900}, want: 5000},
		{name: "ignores gaps", floor: 0, values: []float64{math.NaN(), 7000}, want: 10000},
		{name: "ignores infinity", floor: math.Inf(1), values: []float64{math.Inf(-1), 7}, want: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := chart.NiceMax(test.floor, test.values); got != test.want {
				t.Fatalf("NiceMax() = %v, want %v", got, test.want)
			}
		})
	}
}
