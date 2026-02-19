package emu

// timer represents a YM2612 timer (A or B).
type timer struct {
	period  uint16 // Loaded period value
	counter uint16 // Current counter value
}

// stepTimers advances Timer A and Timer B.
// Timer A: 10-bit, counts up by 1 per sample clock, overflows at 1024.
// Timer B: 8-bit, counts up by 1 per 16 sample clocks, overflows at 256.
func (y *YM2612) stepTimers() {
	// Timer A
	if y.timerALoad {
		// CSM key-off on the tick after key-on
		if y.csmKeyOn {
			y.csmKeyOff()
			y.csmKeyOn = false
		}

		y.timerA.counter++
		if y.timerA.counter >= 1024-y.timerA.period {
			y.timerA.counter = 0
			if y.timerAEnable {
				y.timerAOver = true
			}
			// CSM: Timer A overflow triggers key-on for all ch3 operators
			if y.ch3Mode == ch3ModeCSM {
				y.csmKeyOnAll()
				y.csmKeyOn = true
			}
		}
	}

	// Timer B: ticks every 16 sample clocks
	// We track this using a sub-counter embedded in the high bits.
	// Actually, we use a simpler approach: count samples, fire every 16th.
	y.timerBSubCount++
	if y.timerBSubCount >= 16 {
		y.timerBSubCount = 0
		if y.timerBLoad {
			y.timerB.counter++
			if y.timerB.counter >= 256-y.timerB.period {
				y.timerB.counter = 0
				if y.timerBEnable {
					y.timerBOver = true
				}
			}
		}
	}
}

// csmKeyOnAll triggers key-on for all 4 channel 3 operators.
// Does NOT set op.keyOn (that is owned by register $28).
func (y *YM2612) csmKeyOnAll() {
	ch := &y.ch[2]
	for i := 0; i < 4; i++ {
		op := &ch.op[i]
		op.phaseCounter = 0
		op.egState = egAttack
		op.ssgInverted = op.ssgEG&ssgAttack != 0
		if y.effectiveRate(op.ar, op) >= 62 {
			op.egLevel = 0
			op.egState = egDecay
		}
	}
}

// csmKeyOff releases channel 3 operators not held by register $28.
func (y *YM2612) csmKeyOff() {
	ch := &y.ch[2]
	for i := 0; i < 4; i++ {
		op := &ch.op[i]
		if !op.keyOn {
			if op.ssgEG&ssgEnable != 0 && op.ssgInverted {
				op.egLevel = (ssgCenter - op.egLevel) & 0x3FF
				op.ssgInverted = false
			}
			op.egState = egRelease
		}
	}
}
