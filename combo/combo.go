package combo

import (
	"desktop-audio-ctrl/protocol"
	"desktop-audio-ctrl/rotary"
	screenlib "desktop-audio-ctrl/screen"
	"fmt"
	"image/color"
	"machine"
	"math"
	"math/rand/v2"
	"time"

	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

var (
	drawColor = color.RGBA{255, 255, 255, 255}
)

type Combo struct {
	screen    *screenlib.Screen
	encoder   *rotary.Encoder
	state     uint8
	name      string
	id        uint8
	lastCount int32
	lastTime  time.Time
	exactStep float64
}

func NewCombo(i2c *machine.I2C, screenChannel uint8, encoderAddress uint16, name string, id uint8) *Combo {
	c := Combo{
		screen:  screenlib.NewScreen(screenChannel),
		encoder: rotary.NewEncoder(i2c, encoderAddress),
		name:    name,
		state:   uint8(rand.IntN(101)),
		id:      id,
	}
	return &c
}

func (c *Combo) SetState(state uint8) bool {
	if state == c.state {
		return false
	}
	c.state = state
	return true
}

const (
	STEP_INCREASE = .8
	MAX_STEP      = 8
)

func (c *Combo) Update() (*protocol.Event, bool) {
	state, err := c.encoder.GetState()
	if err != nil {
		panic(err)
	}
	switch state {
	case rotary.BtnClick:
		c.state = 0
		return protocol.NewEvent(protocol.EVENT_TYPE_CLICK, c.id, c.state), true
	case rotary.BtnDoubleClick:
		return protocol.NewEvent(protocol.EVENT_TYPE_DOUBLE_CLICK, c.id, c.state), true
	default:
		break
	}

	currentCount, err := c.encoder.GetCount()
	if err != nil {
		panic(err)
	}

	delta := currentCount - c.lastCount
	if delta == 0 {
		return nil, false
	}

	var eventType protocol.EventType

	currentTime := time.Now()
	deltaTime := currentTime.Sub(c.lastTime)
	c.lastTime = currentTime
	c.lastCount = currentCount

	deltaTimeMs := float64(deltaTime.Milliseconds())

	step := 1

	if deltaTimeMs < 60 {
		newStep := float64(c.exactStep) + STEP_INCREASE
		step = int(math.Round(newStep))
		if step > MAX_STEP {
			step = MAX_STEP
		}
		if step < 1 {
			step = 1
		}
		c.exactStep = newStep
	} else {
		c.exactStep = 1
	}

	newState := int(c.state) + int(delta)*step
	if newState < 0 {
		c.state = 0
	} else {
		c.state = uint8(newState)
		if c.state < 0 {
			c.state = 0
		}
		if c.state > 100 {
			c.state = 100
		}
	}

	if delta > 0 {
		eventType = protocol.EVENT_TYPE_CW
	} else {
		eventType = protocol.EVENT_TYPE_CCW
	}

	return protocol.NewEvent(eventType, c.id, c.state), true
}

const TEXT_HEIGHT = 9

func (c *Combo) Draw() {
	c.screen.Activate()
	screenlib.Display.ClearBuffer()

	centerText(c.name, &freemono.Regular9pt7b, TEXT_HEIGHT+8)
	bar(c.state)

	screenlib.Display.Display()
}

func centerText(text string, font *tinyfont.Font, x int) {
	_, outBox := tinyfont.LineWidth(font, text)
	y := 64 - ((64 - outBox) / 2)
	tinyfont.WriteLineRotated(screenlib.Display, font, int16(x), int16(y), text, drawColor, tinyfont.ROTATION_270)
}

const (
	quadrantTopLeft = iota + 1
	quadrantTopRight
	quadrantBottomLeft
	quadrantBottomRight
)

func bar(volume uint8) {
	var leftX int16 = 23
	var rightX int16 = 124
	// var topY int16 = 17
	var bottomY int16 = 64 - 10
	var topY int16 = bottomY - 30
	// var bottomY int16 = 47
	var radius int16 = 10

	for y := topY + radius; y <= bottomY-radius; y++ {
		screenlib.Display.SetPixel(leftX, y, drawColor)
		screenlib.Display.SetPixel(rightX, y, drawColor)
	}

	for x := leftX + radius; x <= rightX-radius; x++ {
		screenlib.Display.SetPixel(x, topY, drawColor)
		screenlib.Display.SetPixel(x, bottomY, drawColor)
	}

	drawCorner(leftX+radius, topY+radius, radius, quadrantTopLeft)
	drawCorner(rightX-radius, topY+radius, radius, quadrantTopRight)
	drawCorner(leftX+radius, bottomY-radius, radius, quadrantBottomLeft)
	drawCorner(rightX-radius, bottomY-radius, radius, quadrantBottomRight)

	if volume > 0 {
		startX := rightX - 1 - int16(volume-1)
		if startX < leftX+1 {
			startX = leftX + 1
		}
		endX := rightX - 1
		for x := startX; x <= endX; x++ {
			var yStart, yEnd int16
			if x >= leftX+radius && x <= rightX-radius {
				yStart = topY + 1
				yEnd = bottomY - 1
			} else {
				var dx int16
				if x < leftX+radius {
					dx = (leftX + radius) - x
				} else {
					dx = x - (rightX - radius)
				}
				dy := int16(math.Ceil(math.Sqrt(float64(radius*radius - dx*dx))))
				yStart = (topY + radius) - dy + 1
				yEnd = (bottomY - radius) + dy - 1
			}
			for y := yStart; y <= yEnd; y++ {
				screenlib.Display.SetPixel(x, y, drawColor)
			}
		}
	}

	var text string
	if volume == 100 {
		text = "!!"
	} else {
		text = fmt.Sprintf("%02d", volume)
	}
	var space int16 = 64 - topY
	var textStartY int16 = topY - space/2 + TEXT_HEIGHT*2 - 2
	var textPosX int16 = rightX - int16(volume) + TEXT_HEIGHT/2
	var textTop int16 = textPosX - TEXT_HEIGHT
	var internalOffset int16 = 3

	if textTop < leftX+internalOffset {
		textPosX = leftX + TEXT_HEIGHT + internalOffset
	}
	if textPosX > rightX-internalOffset {
		textPosX = rightX - internalOffset
	}

	tinyfont.WriteLineRotated(screenlib.Display, &freemono.Regular9pt7b, textPosX, textStartY, text, drawColor, tinyfont.ROTATION_270)
}

func drawCorner(centerX, centerY, radius int16, quadrant int) {
	for dx := int16(0); dx <= radius; dx++ {
		dy := int16(math.Round(math.Sqrt(float64(radius*radius - dx*dx))))
		switch quadrant {
		case quadrantTopRight:
			screenlib.Display.SetPixel(centerX+dx, centerY-dy, drawColor)
			screenlib.Display.SetPixel(centerX+dy, centerY-dx, drawColor)
		case quadrantTopLeft:
			screenlib.Display.SetPixel(centerX-dx, centerY-dy, drawColor)
			screenlib.Display.SetPixel(centerX-dy, centerY-dx, drawColor)
		case quadrantBottomLeft:
			screenlib.Display.SetPixel(centerX-dx, centerY+dy, drawColor)
			screenlib.Display.SetPixel(centerX-dy, centerY+dx, drawColor)
		case quadrantBottomRight:
			screenlib.Display.SetPixel(centerX+dx, centerY+dy, drawColor)
			screenlib.Display.SetPixel(centerX+dy, centerY+dx, drawColor)
		}
	}
}

func (c *Combo) ClearScreen() {
	c.screen.Activate()
	screenlib.Display.ClearBuffer()
	screenlib.Display.Display()
}
