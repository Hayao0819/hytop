package chart

import "image"

type columnSpan struct {
	top, bottom int // Raster rows, with bottom excluded; bottom == 0 marks a gap.
}

type linePath []columnSpan

func projectLine(values []float64, resolution image.Point, low, high float64) linePath {
	path := make(linePath, resolution.X)
	var previous image.Point
	connected := false
	for x, value := range sample(values, resolution.X) {
		if !finite(value) {
			connected = false
			continue
		}
		point := image.Pt(x, resolution.Y-max(fill(value, low, high, resolution.Y), 1))
		if connected {
			path.connect(previous, point)
		} else {
			path.add(point)
		}
		previous, connected = point, true
	}
	return path
}

func (path linePath) add(point image.Point) {
	column := &path[point.X]
	if column.bottom == 0 {
		column.top, column.bottom = point.Y, point.Y+1
	} else {
		column.top = min(column.top, point.Y)
		column.bottom = max(column.bottom, point.Y+1)
	}
}

// Endpoints are consecutive columns in the plot raster.
func (path linePath) connect(from, to image.Point) {
	dx, dy := to.X-from.X, to.Y-from.Y
	stepY := 1
	if dy < 0 {
		dy, stepY = -dy, -1
	}
	err := dx - dy
	for {
		path.add(from)
		if from == to {
			return
		}
		twice := 2 * err
		if twice >= -dy {
			err -= dy
			from.X++
		}
		if twice <= dx {
			err += dx
			from.Y += stepY
		}
	}
}
