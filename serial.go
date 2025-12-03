package rtu

import (
	"io"
	"time"

	"github.com/albenik/go-serial/v2"
)

const (
	SERIAL_TIMEOUT = 30 * time.Millisecond
	SERIAL_WAIT    = 30 * time.Millisecond
	BAUDRATE       = 9600
)

type OpenErr struct {
	Dev string
	Err error
}

func (e OpenErr) Error() string {
	return e.Err.Error() + " while opening " + e.Dev
}

func (e OpenErr) Unwrap() error {
	return e.Err
}

type SerialPort struct {
	Dev      string
	Timeout  time.Duration
	Wait     time.Duration
	Baudrate int
	Parity   Parity
}

func (p *SerialPort) Open(
	repeat bool,
) (io.ReadWriteCloser, time.Duration, error) {
	if p.Dev == "" {
		panic("empty SerialPort.Dev")
	}
	if p.Timeout <= 0 {
		p.Timeout = SERIAL_TIMEOUT
	}
	if p.Wait <= 0 {
		p.Wait = SERIAL_WAIT
	}
	if p.Baudrate <= 0 {
		p.Baudrate = BAUDRATE
	}

	if repeat {
		debugLog("Opening %s", p.Dev)
	} else {
		log("Opening %s", p.Dev)
	}
	port, err := serial.Open(p.Dev,
		serial.WithBaudrate(p.Baudrate),
		serial.WithParity(serial.Parity(p.Parity)),
		serial.WithReadTimeout(int(p.Timeout.Milliseconds())),
		serial.WithWriteTimeout(int(p.Timeout.Milliseconds())))
	if err != nil {
		return nil, p.Wait, OpenErr{p.Dev, err}
	}
	log("%s opened", p.Dev)
	return port, p.Wait, nil
}
