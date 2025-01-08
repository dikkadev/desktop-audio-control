package screen

import (
	"desktop-audio-ctrl/multiplexer"
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/sh1106"
)

const ADDR = 0x3C

var (
	Display *sh1106.Device
	bus     *machine.I2C
	mux     *multiplexer.Multiplexer

	onColor  = color.RGBA{255, 255, 255, 255}
	offColor = color.RGBA{0, 0, 0, 255}
)

func Initialize(bus_ *machine.I2C, mux_ *multiplexer.Multiplexer) {
	bus = bus_
	println("Bus var set")
	disp := sh1106.NewI2C(bus)
	Display = &disp
	Display.Configure(sh1106.Config{
		Width:    128,
		Height:   64,
		VccState: sh1106.SWITCHCAPVCC,
		Address:  ADDR,
	})
	println("Display initialized")
	mux = mux_
	println("Multiplexer var set")

	for i := uint8(0); i < 5; i++ {
		mux.Select(i)
		Display.Configure(sh1106.Config{
			Width:    128,
			Height:   64,
			VccState: sh1106.SWITCHCAPVCC,
			Address:  ADDR,
		})
		Display.ClearBuffer()
	}
	println("Screens cleared")
}

type Screen struct {
	MuxChannel uint8
}

func NewScreen(muxChannel uint8) *Screen {
	return &Screen{
		MuxChannel: muxChannel,
	}
}

func (s *Screen) Activate() {
	if mux == nil {
		panic("Multiplexer not initialized")
	}
	err := mux.Select(s.MuxChannel)
	if err != nil {
		panic(err)
	}
	time.Sleep(5 * time.Millisecond)
}

func (s *Screen) Clear() {
	s.Activate()
	Display.ClearBuffer()
	Display.Display()
}

func (s *Screen) DrawImage(img []byte) {
	s.Activate()
	Display.ClearBuffer()
	Display.SetBuffer(img)
	Display.Display()
}
