package rtu

import "github.com/bangzek/clock"

func SetClock(mock *clock.Mock) {
	ctime = mock
}
