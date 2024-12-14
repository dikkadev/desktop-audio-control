package main

import (
	"desktop-audio-ctrl/combo"
	"desktop-audio-ctrl/multiplexer"
	"desktop-audio-ctrl/protocol"
	screenlib "desktop-audio-ctrl/screen"
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/ws2812"
)

const (
	muxAddr = 0x70

	DELIMINATOR = 0xF0
)

var (
	combos = make([]*combo.Combo, 5, 5)
	names  = []string{"Game", "Chat", "Media", "Aux", "Speak"}
)

func main() {
	time.Sleep(time.Second * 2)

	blinkInternal()

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
	const eventLength = 6
	buffer := make([]byte, 0, eventLength)

	blinkInternal()

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
				event, ok := protocol.Unmarshal(buffer[:eventLength-1])
				if ok {
					handleEvent(event)
					// serial.Write(protocol.NewEvent(protocol.EVENT_TYPE_ACK, event.Combo, event.State).
					serial.Write(protocol.Marshal(protocol.Event{Type: protocol.EVENT_TYPE_ACK, Combo: event.Combo, State: event.State}))
				} else {
					// println("Invalid event received")
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
				packet := protocol.Marshal(*event)
				packet = append(packet, DELIMINATOR)
				_, err := serial.Write(packet)
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

// blinks the ws2812 LED @ gp16 3 times
func blinkInternal() {
	onTime := 100 * time.Millisecond
	offTime := 100 * time.Millisecond
	led := ws2812.NewWS2812(machine.WS2812)
	for i := 0; i < 3; i++ {
		led.WriteColors([]color.RGBA{color.RGBA{255, 0, 0, 255}})
		time.Sleep(onTime)
		led.WriteColors([]color.RGBA{color.RGBA{0, 0, 0, 0}})
		time.Sleep(offTime)
	}
}

// handleEvent processes incoming events
func handleEvent(e protocol.Event) {
	// println("Received event:", e.String())
	// blinkInternal()
	switch e.Type {
	case protocol.EVENT_TYPE_ACK:
		println("Received ACK event for Combo:", e.Combo, "with State:", e.State)
	case protocol.EVENT_TYPE_SET:
		if e.Combo < uint8(len(combos)) {
			// println("Received SET event for Combo:", e.Combo, "with State:", e.State)
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
