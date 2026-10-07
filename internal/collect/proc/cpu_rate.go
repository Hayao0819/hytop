package proc

type cpuMark struct {
	started uint64
	total   float64
}

func cpuRate(previous, current, elapsed float64, sameProcess bool) float64 {
	if !sameProcess || elapsed <= 0 || current < previous {
		return 0
	}

	return 100 * (current - previous) / elapsed
}
