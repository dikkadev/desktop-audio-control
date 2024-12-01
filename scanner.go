package main

import (
	"desktop-audio-ctrl/multiplexer"
	"machine"
	"time"
)

func main() {
	time.Sleep(time.Second * 2)

	// i2c scan
	i2c := machine.I2C0
	err := i2c.Configure(machine.I2CConfig{
		SDA: machine.GPIO0,
		SCL: machine.GPIO1,
	})
	if err != nil {
		println("Failed to configure I2C bus")
		return
	}

	muxAddr := uint16(0x70)
	mux := multiplexer.NewMultiplexer(i2c, muxAddr)
	_ = mux

	for {
		println("Scanning I2C bus")
		for channel := uint8(0); channel < 8; channel++ {
			err := mux.Select(channel)
			println("Channel", channel)
			if err != nil {
				panic(err)
			}
			for addr := uint16(0x08); addr < 0x78; addr++ {
				if i2c.Tx(addr, []byte{0x00}, nil) == nil {
					println("Found device at address 0x", itoh(addr))
				} else {
					// println("No device found at address 0x", itoh(addr))
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
}

func itoh(i uint16) string {
	return "0x" + string("0123456789ABCDEF"[i>>4]) + string("0123456789ABCDEF"[i&0x0F])
}
