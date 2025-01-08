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

	INACTIVITY_TIMEOUT = 15 * time.Second
)

var (
	combos = make([]*combo.Combo, 5, 5)
	names  = []string{"Game", "Chat", "Media", "Aux", "Speak"}

	lastActivity = time.Now()
	screenOn     = true
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

	for i := 0; i < 5; i++ {
		mux.Select(uint8(i))
		if i2c.Tx(0x3C, []byte{0x00}, nil) != nil {
			panic("No device found at 0x3C in channel " + string(i+48))
		}
	}

	screenlib.Initialize(i2c, mux)
	time.Sleep(time.Millisecond * 100)

	println("Creating combos")
	for i := 0; i < 5; i++ {
		combos[i] = combo.NewCombo(i2c, uint8(i), uint16(0x30+i), names[i], uint8(i))
		combos[i].Draw()
	}

	serial := machine.Serial

	const eventLength = 6
	buffer := make([]byte, 0, eventLength)

	blinkInternal()

	for {
		updated := false

		for serial.Buffered() > 0 {
			b, err := serial.ReadByte()
			if err != nil {
				println("Error reading serial:", err)
				break
			}
			buffer = append(buffer, b)

			if len(buffer) >= eventLength {
				event, ok := protocol.Unmarshal(buffer[:eventLength-1])
				if ok {
					handleEvent(event)
					serial.Write(protocol.Marshal(protocol.Event{Type: protocol.EVENT_TYPE_ACK, Combo: event.Combo, State: event.State}))
				} else {
					// println("Invalid event received")
				}
				buffer = buffer[eventLength:]
			}
		}

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
				lastActivity = time.Now()
			}
		}

		if screenOn && time.Since(lastActivity) > INACTIVITY_TIMEOUT {
			turnScreensOff()
		}

		if !screenOn && updated {
			turnScreensOn()
		}

		if !updated {
			time.Sleep(time.Millisecond * 3)
		}

	}
}

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

func handleEvent(e protocol.Event) {
	switch e.Type {
	case protocol.EVENT_TYPE_ACK:
		println("Received ACK event for Combo:", e.Combo, "with State:", e.State)
	case protocol.EVENT_TYPE_SET:
		if e.Combo < uint8(len(combos)) {
			changed := combos[e.Combo].SetState(e.State)
			if changed {
				combos[e.Combo].Draw()
				lastActivity = time.Now()
			}
		} else {
			println("Invalid Combo ID in SET event:", e.Combo)
		}
	default:
		println("Received non-SET event:", e.String())
	}
}

func turnScreensOff() {
	for _, c := range combos {
		c.ClearScreen()
	}
	screenOn = false
	println("Screens turned off due to inactivity")
}

func turnScreensOn() {
	for _, c := range combos {
		c.Draw()
	}
	screenOn = true
	println("Screens turned on due to activity")
}
