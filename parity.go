package rtu

import (
	"fmt"

	"github.com/albenik/go-serial/v2"
)

type Parity serial.Parity

const (
	NoParity   = Parity(serial.NoParity)
	OddParity  = Parity(serial.OddParity)
	EvenParity = Parity(serial.EvenParity)
)

func (p Parity) IsValid() bool {
	switch p {
	case NoParity, OddParity, EvenParity:
		return true
	default:
		return false
	}
}

func (p Parity) String() string {
	switch p {
	case NoParity:
		return "NONE"
	case OddParity:
		return "ODD"
	case EvenParity:
		return "EVEN"
	default:
		return fmt.Sprintf("ERR:%d", p)
	}
}

func (p Parity) MarshalText() ([]byte, error) {
	if p.IsValid() {
		return []byte(p.String()), nil
	} else {
		return nil, fmt.Errorf("Invalid Parity: %d", p)
	}
}

func (p *Parity) UnmarshalText(b []byte) error {
	switch string(b) {
	case "NONE":
		*p = NoParity
	case "ODD":
		*p = OddParity
	case "EVEN":
		*p = EvenParity
	default:
		return fmt.Errorf("Invalid Parity from %q", b)
	}
	return nil
}
