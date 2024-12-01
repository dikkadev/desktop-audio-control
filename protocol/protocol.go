package protocol

import "fmt"

type EventType uint8

const (
	// device -> host
	EVENT_TYPE_CW EventType = iota + 1
	EVENT_TYPE_CCW
	EVENT_TYPE_CLICK
	EVENT_TYPE_DOUBLE_CLICK

	// host -> device
	EVENT_TYPE_SET
)

const (
	SIGNATURE uint8 = 0x69
)

type Event struct {
	Type  EventType
	Combo uint8
	State uint8
}

func Marshal(e Event) []byte {
	return []byte{SIGNATURE, SIGNATURE, uint8(e.Type), e.Combo, e.State}
}

func Unmarshal(data []byte) (Event, bool) {
	if len(data) != 5 {
		return Event{}, false
	}
	if data[0] != SIGNATURE || data[1] != SIGNATURE {
		return Event{}, false
	}
	fmt.Println("type byte:", data[2])
	fmt.Println("combo byte:", data[3])
	fmt.Println("state byte:", data[4])
	return Event{Type: EventType(data[2]), Combo: data[3], State: data[4]}, true
}

func NewEvent(t EventType, c, s uint8) *Event {
	return &Event{Type: t, Combo: c, State: s}
}

func (e *Event) String() string {
	var state string
	if e.State < 10 {
		state = "  " + string(e.State+48)
	} else if e.State < 100 {
		state = " " + string(e.State/10+48) + string(e.State%10+48)
	} else if e.State == 100 {
		state = "100"
	} else {
		state = "ERR"
	}

	combo := string(e.Combo + 48)

	switch e.Type {
	case EVENT_TYPE_CW:
		return "CW    " + combo + " " + state
	case EVENT_TYPE_CCW:
		return "CCW   " + combo + " " + state
	case EVENT_TYPE_CLICK:
		return "Clck  " + combo + " " + state
	case EVENT_TYPE_DOUBLE_CLICK:
		return "DblClck" + combo + " " + state
	case EVENT_TYPE_SET:
		return "Set   " + combo + " " + state
	default:
		return "Unknown" + combo + " " + state
	}
}

func IsEventAtStart(data []byte) bool {
	if data[0] != SIGNATURE || data[1] != SIGNATURE {
		return false
	}
	return true
}
