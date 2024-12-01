package multiplexer

import "machine"

// TCA9548A multiplexer
type Multiplexer struct {
	i2c     *machine.I2C
	addr    uint16
	Channel uint8
}

func NewMultiplexer(i2c *machine.I2C, addr uint16) *Multiplexer {
	return &Multiplexer{
		i2c:  i2c,
		addr: addr,
	}
}

func (m *Multiplexer) Select(channel uint8) error {
	data := []byte{1 << channel}
	// println("Changing mux channel to ", channel)
	return m.i2c.Tx(m.addr, data, nil)
}
