package main

import (
	"desktop-audio-ctrl/combo"
	"desktop-audio-ctrl/multiplexer"
	"desktop-audio-ctrl/protocol"
	screenlib "desktop-audio-ctrl/screen"
	"machine"
	"time"
)

const (
	muxAddr = 0x70
)

var (
	combos = make([]*combo.Combo, 5, 5)
	names  = []string{"Game", "Chat", "Media", "Aux", "Speak"}
)

func main() {
	time.Sleep(time.Second * 2)

	// Configure I2C
	i2c := machine.I2C0
	err := i2c.Configure(machine.I2CConfig{
		SDA:       machine.GPIO0,
		SCL:       machine.GPIO1,
		Frequency: 400000,
	})
	if err != nil {
		println("Failed to configure I2C bus")
		return
	}

	mux := multiplexer.NewMultiplexer(i2c, muxAddr)

	// Initialize all channels
	for i := 0; i < 5; i++ {
		mux.Select(uint8(i))
		if i2c.Tx(0x3C, []byte{0x00}, nil) != nil {
			panic("No device found at 0x3C in channel " + string(i+48))
		}
	}

	// Initialize the screen
	screenlib.Initialize(i2c, mux)
	time.Sleep(time.Millisecond * 100)

	println("Creating combos")
	for i := 0; i < 5; i++ {
		combos[i] = combo.NewCombo(i2c, uint8(i), uint16(0x30+i), names[i], uint8(i))
		combos[i].Draw()
	}

	// Initialize Serial
	serial := machine.Serial

	// Buffer to store incoming serial data
	const eventLength = 5
	buffer := make([]byte, 0, eventLength)

	for {
		updated := false

		// Handle incoming serial data
		for serial.Buffered() > 0 {
			b, err := serial.ReadByte()
			if err != nil {
				println("Error reading serial:", err)
				break
			}
			buffer = append(buffer, b)

			// Check if buffer has enough data for an event
			if len(buffer) >= eventLength {
				// Attempt to unmarshal the event
				event, ok := protocol.Unmarshal(buffer[:eventLength])
				if ok {
					handleEvent(event)
				} else {
					println("Invalid event received")
				}
				// Remove the processed bytes from the buffer
				buffer = buffer[eventLength:]
			}
		}

		// Handle combo updates
		for i := 0; i < 5; i++ {
			if event, ok := combos[i].Update(); ok {
				combos[i].Draw()
				updated = true
				_, err := serial.Write(protocol.Marshal(*event))
				if err != nil {
					println("ERROR: ", err)
				}
			}
		}

		// Sleep briefly if no updates occurred
		if !updated {
			time.Sleep(time.Millisecond * 3)
		}
	}
}

// handleEvent processes incoming events
func handleEvent(e protocol.Event) {
	switch e.Type {
	case protocol.EVENT_TYPE_SET:
		if e.Combo < uint8(len(combos)) {
			println("Received SET event for Combo:", e.Combo, "with State:", e.State)
			combos[e.Combo].SetState(e.State)
			combos[e.Combo].Draw()
		} else {
			println("Invalid Combo ID in SET event:", e.Combo)
		}
	default:
		// Handle other event types if necessary
		println("Received non-SET event:", e.String())
	}
}
