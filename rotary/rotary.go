package rotary

import (
	"machine"
)

type RotaryState byte

const (
	RotaryIdle     RotaryState = 0x00
	BtnClick       RotaryState = 0x01
	BtnDoubleClick RotaryState = 0x02
	BtnLongPress   RotaryState = 0x03
	BtnLongRelease RotaryState = 0x04
	RotaryCCW      RotaryState = 0x05
	RotaryCW       RotaryState = 0x06
)

func (r RotaryState) String() string {
	switch r {
	case RotaryIdle:
		return "Idle"
	case BtnClick:
		return "Click"
	case BtnDoubleClick:
		return "Double Click"
	case BtnLongPress:
		return "Long Press"
	case BtnLongRelease:
		return "Long Release"
	case RotaryCCW:
		return "Counter Clockwise"
	case RotaryCW:
		return "Clockwise"
	default:
		return "Unknown"
	}
}

type Encoder struct {
	i2c       *machine.I2C
	address   uint16
	counter   int32
	lastState RotaryState
}

func NewEncoder(i2c *machine.I2C, address uint16) *Encoder {
	return &Encoder{
		i2c:     i2c,
		address: address,
	}
}

// GetCount reads the current internal counter value from the encoder.
func (e *Encoder) GetCount() (int32, error) {
	buf := make([]byte, 5)
	err := e.i2c.Tx(e.address, nil, buf)
	if err != nil {
		return 0, err
	}

	count := int32(buf[0]) |
		int32(buf[1])<<8 |
		int32(buf[2])<<16 |
		int32(buf[3])<<24

	return count, nil
}

// GetState reads the current state of the encoder.
func (e *Encoder) GetState() (RotaryState, error) {
	buf := make([]byte, 5)
	err := e.i2c.Tx(e.address, nil, buf)
	if err != nil {
		return RotaryIdle, err
	}

	e.lastState = RotaryState(buf[4])
	return e.lastState, nil
}

// ResetCounter resets the internal encoder's counter to zero.
func (e *Encoder) ResetCounter() error {
	resetFlag := []byte{0xAA}
	return e.i2c.Tx(e.address, resetFlag, nil)
}
