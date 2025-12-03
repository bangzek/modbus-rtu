package rtu

import (
	"io"
	"time"

	"github.com/bangzek/clock"
)

const (
	TIMEOUT = time.Second
)

var (
	ctime = clock.New()
)

type PortOpener interface {
	Open(bool) (io.ReadWriteCloser, time.Duration, error)
}

type Controller struct {
	Port    PortOpener
	Timeout time.Duration

	port   io.ReadWriteCloser
	wait   time.Duration
	repeat bool
}

func (c *Controller) Close() {
	if c.port != nil {
		c.port.Close()
		c.port = nil
	}
}

func (c *Controller) Send(cmd Cmd) error {
	if c.Timeout <= 0 {
		c.Timeout = TIMEOUT
	}
	if c.port == nil {
		var err error
		c.port, c.wait, err = c.Port.Open(c.repeat)
		if err != nil {
			c.repeat = true
			return err
		}
		c.repeat = false
	}

	tx := cmd.TxBytes()
	debugLog("tx: % X", tx)
	debugLog("TX: %s", cmd.Tx())
	if n, err := c.port.Write(tx); err != nil {
		c.Close()
		return err
	} else if n != len(tx) {
		c.Close()
		return io.ErrShortWrite
	}

	time.Sleep(c.wait)

	rx := cmd.RxBytes()
	if cap(*rx) == 0 {
		return nil
	}

	for deadline := ctime.Now().Add(c.Timeout); ; {
		if n, ok, err := c.read(rx, cmd.IsValidRx); err != nil {
			c.Close()
			return err
		} else if n > 0 {
			debugLog("rx: % X", *rx)
			if !ok {
				c.Close()
				return BadRxErr(*rx)
			}
			debugLog("RX: %s", cmd.Rx())
			break
		}

		if ctime.Now().After(deadline) {
			c.Close()
			return ErrTimeout
		}
	}
	return cmd.Err()
}

func (c *Controller) read(b *[]byte, isValid func() bool) (int, bool, error) {
	*b = (*b)[:cap(*b)]
	for n := 0; n < len(*b); {
		nn, err := c.port.Read((*b)[n:])
		n += nn
		*b = (*b)[:n]
		if err != nil {
			return n, false, err
		} else if nn == 0 {
			return n, false, nil
		} else if isValid() {
			return n, true, nil
		}
		*b = (*b)[:cap(*b)]
	}
	return len(*b), false, nil
}
