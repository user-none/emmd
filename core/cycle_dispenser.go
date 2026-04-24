package core

// cycleDispenser emits integer cycle counts per scanline whose sum over
// (fps * scanlines) calls equals exactly clockHz. The remainder is carried
// across scanlines and frames in accum, so totals stay exact indefinitely.
type cycleDispenser struct {
	clockHz int
	divisor int
	accum   int
}

func newCycleDispenser(clockHz, fps, scanlines int) cycleDispenser {
	return cycleDispenser{clockHz: clockHz, divisor: fps * scanlines}
}

// Next returns the cycle count for the next scanline.
func (c *cycleDispenser) Next() int {
	c.accum += c.clockHz
	n := c.accum / c.divisor
	c.accum -= n * c.divisor
	return n
}
