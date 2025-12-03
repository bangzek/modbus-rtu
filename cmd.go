package rtu

import (
	"bytes"
	"fmt"
	"strconv"
	"unsafe"
)

type Cmd interface {
	TxBytes() []byte
	DevAddr() byte
	SetDevAddr(byte)
	Addr() uint16
	SetAddr(uint16)
	Tx() string

	RxBytes() *[]byte
	IsValidRx() bool
	Rx() string
	Err() error

	String() string
}

type cmd struct {
	tx []byte
	rx []byte
}

func (c *cmd) TxBytes() []byte {
	return c.tx
}

func (c *cmd) DevAddr() byte {
	return c.tx[0]
}

func (c *cmd) SetDevAddr(x byte) {
	c.tx[0] = x
	SetChecksum(c.tx)
}

func (c *cmd) Addr() uint16 {
	return (uint16(c.tx[2]) << 8) | uint16(c.tx[3])
}

func (c *cmd) SetAddr(x uint16) {
	c.tx[2] = byte(x >> 8)
	c.tx[3] = byte(x)
	SetChecksum(c.tx)
}

func (c *cmd) RxBytes() *[]byte {
	return &c.rx
}

func (c *cmd) Err() error {
	if len(c.rx) == 5 {
		return ModbusErr(c.rx[2])
	} else {
		return nil
	}
}

func (c *cmd) isValidErr() bool {
	return len(c.rx) == 5 && checksum(c.rx) &&
		c.rx[0] == c.tx[0] && c.rx[1] == c.tx[1]|0x80
}

//----------------------------------------------------------------------

type ReadCoilsCmd struct {
	cmd
}

func NewReadCoilsCmd(devAddr byte, addr uint16, count uint16) *ReadCoilsCmd {
	if devAddr == 0 {
		panic("could not broadcast ReadCoilsCmd")
	}
	if count == 0 {
		panic("zero count")
	}
	if count > 2000 {
		panic(fmt.Sprintf("count too many: %d", count))
	}
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 1
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	tx[4] = byte(count >> 8)
	tx[5] = byte(count)
	SetChecksum(tx)

	l := count / 8
	if count%8 > 0 {
		l++
	}

	return &ReadCoilsCmd{cmd{
		tx: tx,
		rx: make([]byte, 0, l+5),
	}}
}

func (c *ReadCoilsCmd) Count() int {
	return (int(c.tx[4]) << 8) | int(c.tx[5])
}

func (c *ReadCoilsCmd) Coil(i int) bool {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	b := byte(1 << (i % 8))
	return c.rx[3+i/8]&b == b
}

func (c *ReadCoilsCmd) Bytes() []byte {
	return c.rx[3 : len(c.rx)-2]
}

func (c *ReadCoilsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) >= 6 && checksum(c.rx) &&
			c.rx[0] == c.tx[0] &&
			c.rx[1] == c.tx[1] &&
			c.rx[2] == c.byteLen() &&
			len(c.rx) == int(c.rx[2])+5)
}

func (c *ReadCoilsCmd) byteLen() byte {
	n := c.Count()
	l := byte(n / 8)
	if n%8 > 0 {
		l++
	}
	return l
}

func (c *ReadCoilsCmd) String() string {
	if c.IsValidRx() {
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 8
		if err := c.Err(); err != nil {
			l += daLen(c.rx[0]) + 26
		} else {
			l += daLen(c.rx[0]) + cLen(c.Count()) + 8 +
				c.Count()*2 + c.Count()/5
			if c.Count() > 10 {
				l += 2
			}
		}
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = c.aRx(b)
		return unsafe.String(&b[0], len(b))
	} else {
		h := hexs(c.rx)
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
			10 + h.Len()
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = append(b, '[')
		b = h.Append(b)
		b = append(b, ']')
		return unsafe.String(&b[0], len(b))
	}
}

func (c *ReadCoilsCmd) Tx() string {
	//  <- 2
	// RC  3
	// ' ' 1
	//   : 1
	// -----+
	//     7
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 7
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadCoilsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-RC  "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(c.Count()), 10)
	return b
}

func (c *ReadCoilsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// RC   3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// RC  3
		// ' ' 1
		//  [] 2
		// -----+
		//     8
		l += 8 + cLen(c.Count()) + c.Count()*2 + c.Count()/5
		if c.Count() > 10 {
			l += 2
		}
	}
	noteAlloc(l)
	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadCoilsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->RC  "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		n := c.Count()
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, '[')
		for i := 0; i < n; i++ {
			if i == 0 && n > 10 {
				b = append(b, '\n')
				b = append(b, ' ')
			}
			if i > 0 {
				if i%10 == 0 {
					b = append(b, '\n')
					b = append(b, ' ')
				} else {
					b = append(b, ' ')
					if i%5 == 0 {
						b = append(b, ' ')
					}
				}
			}
			if c.Coil(i) {
				b = append(b, '1')
			} else {
				b = append(b, '0')
			}
		}
		if n > 10 {
			b = append(b, '\n')
		}
		return append(b, ']')
	}
}

//----------------------------------------------------------------------

type ReadDInputsCmd struct {
	cmd
}

func NewReadDInputsCmd(
	devAddr byte, addr uint16, count uint16,
) *ReadDInputsCmd {
	if devAddr == 0 {
		panic("could not broadcast ReadDInputsCmd")
	}
	if count == 0 {
		panic("zero count")
	}
	if count > 2000 {
		panic(fmt.Sprintf("count too many: %d", count))
	}
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 2
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	tx[4] = byte(count >> 8)
	tx[5] = byte(count)
	SetChecksum(tx)

	l := count / 8
	if count%8 > 0 {
		l++
	}

	return &ReadDInputsCmd{cmd{
		tx: tx,
		rx: make([]byte, 0, l+5),
	}}
}

func (c *ReadDInputsCmd) Count() int {
	return (int(c.tx[4]) << 8) | int(c.tx[5])
}

func (c *ReadDInputsCmd) Input(i int) bool {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	b := byte(1 << (i % 8))
	return c.rx[3+i/8]&b == b
}

func (c *ReadDInputsCmd) Bytes() []byte {
	return c.rx[3 : len(c.rx)-2]
}

func (c *ReadDInputsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) >= 6 && checksum(c.rx) &&
			c.rx[0] == c.tx[0] &&
			c.rx[1] == c.tx[1] &&
			c.rx[2] == c.byteLen() &&
			len(c.rx) == int(c.rx[2])+5)
}

func (c *ReadDInputsCmd) byteLen() byte {
	n := c.Count()
	l := byte(n / 8)
	if n%8 > 0 {
		l++
	}
	return l
}

func (c *ReadDInputsCmd) String() string {
	if c.IsValidRx() {
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 8
		if err := c.Err(); err != nil {
			l += daLen(c.rx[0]) + 26
		} else {
			l += daLen(c.rx[0]) + cLen(c.Count()) + 8 +
				c.Count()*2 + c.Count()/5
			if c.Count() > 10 {
				l += 2
			}
		}
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = c.aRx(b)
		return unsafe.String(&b[0], len(b))
	} else {
		h := hexs(c.rx)
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
			10 + h.Len()
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = append(b, '[')
		b = h.Append(b)
		b = append(b, ']')
		return unsafe.String(&b[0], len(b))
	}
}

func (c *ReadDInputsCmd) Tx() string {
	//  <- 2
	// RDI 3
	// ' ' 1
	//   : 1
	// -----+
	//     7
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 7
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadDInputsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-RDI "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(c.Count()), 10)
	return b
}

func (c *ReadDInputsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// RDI  3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// RDI 3
		// ' ' 1
		//  [] 2
		// -----+
		//     8
		l += 8 + cLen(c.Count()) + c.Count()*2 + c.Count()/5
		if c.Count() > 10 {
			l += 2
		}
	}
	noteAlloc(l)
	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadDInputsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->RDI "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		n := c.Count()
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, '[')
		for i := 0; i < n; i++ {
			if i == 0 && n > 10 {
				b = append(b, '\n')
				b = append(b, ' ')
			}
			if i > 0 {
				if i%10 == 0 {
					b = append(b, '\n')
					b = append(b, ' ')
				} else {
					b = append(b, ' ')
					if i%5 == 0 {
						b = append(b, ' ')
					}
				}
			}
			if c.Input(i) {
				b = append(b, '1')
			} else {
				b = append(b, '0')
			}
		}
		if n > 10 {
			b = append(b, '\n')
		}
		return append(b, ']')
	}
}

//----------------------------------------------------------------------

type ReadHRegsCmd struct {
	cmd
}

func NewReadHRegsCmd(devAddr byte, addr uint16, count uint16) *ReadHRegsCmd {
	if devAddr == 0 {
		panic("could not broadcast ReadHRegsCmd")
	}
	if count == 0 {
		panic("zero count")
	}
	if count > 125 {
		panic(fmt.Sprintf("count too many: %d", count))
	}
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 3
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	// tx[4] always 0
	tx[5] = byte(count)
	SetChecksum(tx)

	return &ReadHRegsCmd{cmd{
		tx: tx,
		rx: make([]byte, 0, count*2+5),
	}}
}

func (c *ReadHRegsCmd) Count() int {
	return int(c.tx[5])
}

func (c *ReadHRegsCmd) Reg(i int) uint16 {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	return (uint16(c.rx[3+i*2]) << 8) | uint16(c.rx[3+i*2+1])
}

func (c *ReadHRegsCmd) Bytes() []byte {
	return c.rx[3 : 3+c.Count()*2]
}

func (c *ReadHRegsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) >= 7 && checksum(c.rx) &&
			c.rx[0] == c.tx[0] &&
			c.rx[1] == c.tx[1] &&
			c.rx[2] == c.tx[5]*2 &&
			len(c.rx) == int(c.rx[2])+5)
}

func (c *ReadHRegsCmd) String() string {
	if c.IsValidRx() {
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 8
		if err := c.Err(); err != nil {
			l += daLen(c.rx[0]) + 26
		} else {
			l += daLen(c.rx[0]) + cLen(c.Count()) + 8 + c.Count()*6
			if c.Count() > 5 {
				l += (c.Count() / 5) * 2
			}
			if c.Count() > 10 {
				l += 3
				l -= c.Count() / 10
			}
		}
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = c.aRx(b)
		return unsafe.String(&b[0], len(b))
	} else {
		h := hexs(c.rx)
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
			10 + h.Len()
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = append(b, '[')
		b = h.Append(b)
		b = append(b, ']')
		return unsafe.String(&b[0], len(b))
	}
}

func (c *ReadHRegsCmd) Tx() string {
	//  <- 2
	// RHR 3
	// ' ' 1
	//   : 1
	// -----+
	//     7
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 7
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadHRegsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-RHR "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(c.Count()), 10)
	return b
}

func (c *ReadHRegsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// RHR  3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// RHR 3
		// ' ' 1
		//  [] 2
		// -----+
		//     8
		l += 8 + cLen(c.Count()) + c.Count()*6
		if c.Count() > 5 {
			l += (c.Count() / 5) * 2
		}
		if c.Count() > 10 {
			l += 3
			l -= c.Count() / 10
		}
	}
	noteAlloc(l)

	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadHRegsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->RHR "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		var a [5]byte
		t := a[:0]
		n := c.Count()
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, '[')
		for i := 0; i < n; i++ {
			if i == 0 && n > 10 {
				b = append(b, '\n')
				b = append(b, ' ')
			}
			if i > 0 {
				if i%10 == 0 {
					b = append(b, '\n')
					b = append(b, ' ')
				} else {
					b = append(b, ' ')
					if i%5 == 0 {
						b = append(b, ':')
						b = append(b, ' ')
					}
				}
			}
			t = strconv.AppendInt(t[:0], int64(c.Reg(i)), 10)
			for j := 0; j < cap(t)-len(t); j++ {
				b = append(b, ' ')
			}
			b = append(b, t...)
		}
		if n > 10 {
			b = append(b, '\n')
		}
		return append(b, ']')
	}
}

//----------------------------------------------------------------------

type ReadIRegsCmd struct {
	cmd
}

func NewReadIRegsCmd(devAddr byte, addr uint16, count uint16) *ReadIRegsCmd {
	if devAddr == 0 {
		panic("could not broadcast ReadIRegsCmd")
	}
	if count == 0 {
		panic("zero count")
	}
	if count > 125 {
		panic(fmt.Sprintf("count too many: %d", count))
	}
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 4
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	// tx[4] always 0
	tx[5] = byte(count)
	SetChecksum(tx)

	return &ReadIRegsCmd{cmd{
		tx: tx,
		rx: make([]byte, 0, count*2+5),
	}}
}

func (c *ReadIRegsCmd) Count() int {
	return int(c.tx[5])
}

func (c *ReadIRegsCmd) Reg(i int) uint16 {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	return (uint16(c.rx[3+i*2]) << 8) | uint16(c.rx[3+i*2+1])
}

func (c *ReadIRegsCmd) Bytes() []byte {
	return c.rx[3 : 3+c.Count()*2]
}

func (c *ReadIRegsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) >= 7 && checksum(c.rx) &&
			c.rx[0] == c.tx[0] &&
			c.rx[1] == c.tx[1] &&
			c.rx[2] == c.tx[5]*2 &&
			len(c.rx) == int(c.rx[2])+5)
}

func (c *ReadIRegsCmd) String() string {
	if c.IsValidRx() {
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 8
		if err := c.Err(); err != nil {
			l += daLen(c.rx[0]) + 26
		} else {
			l += daLen(c.rx[0]) + cLen(c.Count()) + 8 + c.Count()*6
			if c.Count() > 5 {
				l += (c.Count() / 5) * 2
			}
			if c.Count() > 10 {
				l += 3
				l -= c.Count() / 10
			}
		}
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = c.aRx(b)
		return unsafe.String(&b[0], len(b))
	} else {
		h := hexs(c.rx)
		l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
			10 + h.Len()
		noteAlloc(l)
		b := make([]byte, 0, l)
		b = c.aTx(b)
		b = append(b, '\n')
		b = append(b, '[')
		b = h.Append(b)
		b = append(b, ']')
		return unsafe.String(&b[0], len(b))
	}
}

func (c *ReadIRegsCmd) Tx() string {
	//  <- 2
	// RIR 3
	// ' ' 1
	//   : 1
	// -----+
	//     7
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) + 7
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadIRegsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-RIR "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(c.Count()), 10)
	return b
}

func (c *ReadIRegsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// RIR  3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// RIR 3
		// ' ' 1
		//  [] 2
		// -----+
		//     8
		l += 8 + cLen(c.Count()) + c.Count()*6
		if c.Count() > 5 {
			l += (c.Count() / 5) * 2
		}
		if c.Count() > 10 {
			l += 3
			l -= c.Count() / 10
		}
	}
	noteAlloc(l)

	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *ReadIRegsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->RIR "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		var a [5]byte
		t := a[:0]
		n := c.Count()
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, '[')
		for i := 0; i < n; i++ {
			if i == 0 && n > 10 {
				b = append(b, '\n')
				b = append(b, ' ')
			}
			if i > 0 {
				if i%10 == 0 {
					b = append(b, '\n')
					b = append(b, ' ')
				} else {
					b = append(b, ' ')
					if i%5 == 0 {
						b = append(b, ':')
						b = append(b, ' ')
					}
				}
			}
			t = strconv.AppendInt(t[:0], int64(c.Reg(i)), 10)
			for j := 0; j < cap(t)-len(t); j++ {
				b = append(b, ' ')
			}
			b = append(b, t...)
		}
		if n > 10 {
			b = append(b, '\n')
		}
		return append(b, ']')
	}
}

//----------------------------------------------------------------------

type WriteCoilCmd struct {
	cmd
}

func NewWriteCoilCmd(devAddr byte, addr uint16, val bool) *WriteCoilCmd {
	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 5
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	if val {
		tx[4] = 0xFF
	}
	// tx[5] always 0
	SetChecksum(tx)

	var rx []byte
	if devAddr > 0 {
		rx = make([]byte, 0, len(tx))
	}

	return &WriteCoilCmd{cmd{
		tx: tx,
		rx: rx,
	}}
}

func (c *WriteCoilCmd) SetDevAddr(x byte) {
	if c.tx[0] == 0 && x != 0 {
		c.rx = make([]byte, 0, len(c.tx))
	} else if c.tx[0] != 0 && x == 0 {
		c.rx = nil
	}

	c.tx[0] = x
	SetChecksum(c.tx)
}

func (c *WriteCoilCmd) Coil() bool {
	return c.tx[4] == 0xFF
}

func (c *WriteCoilCmd) SetCoil(v bool) {
	if v {
		c.tx[4] = 0xFF
	} else {
		c.tx[4] = 0
	}
	SetChecksum(c.tx)
}

func (c *WriteCoilCmd) IsValidRx() bool {
	return c.isValidErr() || (len(c.rx) == 8 && bytes.Equal(c.rx, c.tx))
}

func (c *WriteCoilCmd) String() string {
	if cap(c.rx) > 0 {
		if c.IsValidRx() {
			l := daLen(c.DevAddr()) + aLen(c.Addr()) + 13
			if err := c.Err(); err != nil {
				l += daLen(c.rx[0]) + 26
			} else {
				l += daLen(c.rx[0]) + aLen(c.addr()) + 12
			}
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = c.aRx(b)
			return unsafe.String(&b[0], len(b))
		} else {
			h := hexs(c.rx)
			l := daLen(c.DevAddr()) + aLen(c.Addr()) + 15 + h.Len()
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = append(b, '[')
			b = h.Append(b)
			b = append(b, ']')
			return unsafe.String(&b[0], len(b))
		}
	} else {
		return c.Tx()
	}
}

func (c *WriteCoilCmd) Tx() string {
	//  <- 2
	// W1C 3
	// ' ' 1
	// ' ' 1
	// t/f 5
	// -----+
	//    12
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + 12
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *WriteCoilCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-W1C "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	if c.Coil() {
		return append(b, " true"...)
	} else {
		return append(b, " false"...)
	}
}

func (c *WriteCoilCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// W1C  3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// W1C 3
		// ' ' 1
		// ' ' 1
		// t/f 5
		// -----+
		//    12
		l += aLen(c.addr()) + 12
	}
	noteAlloc(l)
	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *WriteCoilCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->W1C "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		b = strconv.AppendInt(b, int64(c.addr()), 10)
		if c.Coil() {
			return append(b, " true"...)
		} else {
			return append(b, " false"...)
		}
	}
}

func (c *WriteCoilCmd) addr() uint16 {
	return (uint16(c.rx[2]) << 8) | uint16(c.rx[3])
}

//----------------------------------------------------------------------

type WriteRegCmd struct {
	cmd
}

func NewWriteRegCmd(devAddr byte, addr uint16, val uint16) *WriteRegCmd {
	tx := make([]byte, 8)
	tx[0] = devAddr
	tx[1] = 6
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	tx[4] = byte(val >> 8)
	tx[5] = byte(val)
	SetChecksum(tx)

	var rx []byte
	if devAddr > 0 {
		rx = make([]byte, 0, len(tx))
	}

	return &WriteRegCmd{cmd{
		tx: tx,
		rx: rx,
	}}
}

func (c *WriteRegCmd) SetDevAddr(x byte) {
	if c.tx[0] == 0 && x != 0 {
		c.rx = make([]byte, 0, len(c.tx))
	} else if c.tx[0] != 0 && x == 0 {
		c.rx = nil
	}

	c.tx[0] = x
	SetChecksum(c.tx)
}

func (c *WriteRegCmd) Reg() uint16 {
	return (uint16(c.tx[4]) << 8) | uint16(c.tx[5])
}

func (c *WriteRegCmd) SetReg(v uint16) {
	c.tx[4] = byte(v >> 8)
	c.tx[5] = byte(v)
	SetChecksum(c.tx)
}

func (c *WriteRegCmd) IsValidRx() bool {
	return c.isValidErr() || (len(c.rx) == 8 && bytes.Equal(c.rx, c.tx))
}

func (c *WriteRegCmd) String() string {
	if cap(c.rx) > 0 {
		if c.IsValidRx() {
			l := daLen(c.DevAddr()) + aLen(c.Addr()) + aLen(c.Reg()) + 8
			if err := c.Err(); err != nil {
				l += daLen(c.rx[0]) + 26
			} else {
				l += daLen(c.rx[0]) + aLen(c.addr()) + aLen(c.reg()) + 7
			}
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = c.aRx(b)
			return unsafe.String(&b[0], len(b))
		} else {
			h := hexs(c.rx)
			l := daLen(c.DevAddr()) + aLen(c.Addr()) + aLen(c.Reg()) +
				10 + h.Len()
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = append(b, '[')
			b = h.Append(b)
			b = append(b, ']')
			return unsafe.String(&b[0], len(b))
		}
	} else {
		return c.Tx()
	}
}

func (c *WriteRegCmd) Tx() string {
	//  <- 2
	// W1R 3
	// ' ' 1
	// ' ' 1
	// -----+
	//     7
	l := daLen(c.DevAddr()) + aLen(c.Addr()) + aLen(c.Reg()) + 7
	noteAlloc(l)
	b := c.aTx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *WriteRegCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-W1R "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ' ')
	b = strconv.AppendInt(b, int64(c.Reg()), 10)
	return b
}

func (c *WriteRegCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// W1R  3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// W1R 3
		// ' ' 1
		// ' ' 1
		// -----+
		//     7
		l += aLen(c.addr()) + aLen(c.reg()) + 7
	}
	noteAlloc(l)
	b := c.aRx(make([]byte, 0, l))
	return unsafe.String(&b[0], len(b))
}

func (c *WriteRegCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->W1R "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		b = strconv.AppendInt(b, int64(c.addr()), 10)
		b = append(b, ' ')
		return strconv.AppendInt(b, int64(c.reg()), 10)
	}
}

func (c *WriteRegCmd) addr() uint16 {
	return (uint16(c.rx[2]) << 8) | uint16(c.rx[3])
}

func (c *WriteRegCmd) reg() uint16 {
	return (uint16(c.rx[4]) << 8) | uint16(c.rx[5])
}

//----------------------------------------------------------------------

type WriteCoilsCmd struct {
	cmd
}

func NewWriteCoilsCmd(devAddr byte, addr uint16, values []bool) *WriteCoilsCmd {
	if len(values) == 0 {
		panic("empty values")
	}
	if len(values) > 1968 {
		panic(fmt.Sprintf("values too many: %d", len(values)))
	}
	count := uint16(len(values))
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	l := count / 8
	if count%8 > 0 {
		l++
	}

	tx := make([]byte, l+9)
	tx[0] = devAddr
	tx[1] = 15
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	tx[4] = byte(count >> 8)
	tx[5] = byte(count)
	tx[6] = byte(l)
	for i, v := range values {
		if v {
			tx[7+i/8] |= 1 << (i % 8)
		}
	}
	SetChecksum(tx)

	var rx []byte
	if devAddr > 0 {
		rx = make([]byte, 0, 8)
	}

	return &WriteCoilsCmd{cmd{
		tx: tx,
		rx: rx,
	}}
}

func (c *WriteCoilsCmd) SetDevAddr(x byte) {
	if c.tx[0] == 0 && x != 0 {
		c.rx = make([]byte, 0, 8)
	} else if c.tx[0] != 0 && x == 0 {
		c.rx = nil
	}

	c.tx[0] = x
	SetChecksum(c.tx)
}

func (c *WriteCoilsCmd) Count() int {
	return (int(c.tx[4]) << 8) | int(c.tx[5])
}

func (c *WriteCoilsCmd) Coil(i int) bool {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	b := byte(1 << (i % 8))
	return c.tx[7+i/8]&b == b
}

func (c *WriteCoilsCmd) SetCoil(i int, v bool) {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	if v {
		c.tx[7+i/8] |= 1 << (i % 8)
	} else {
		c.tx[7+i/8] &= ^byte(1 << (i % 8))
	}
	SetChecksum(c.tx)
}

func (c *WriteCoilsCmd) SetCoils(coils []bool) {
	if len(coils) != c.Count() {
		panic(fmt.Sprintf("invalid coils len: %d<->%d", len(coils), c.Count()))
	}
	for i, v := range coils {
		if v {
			c.tx[7+i/8] |= 1 << (i % 8)
		} else {
			c.tx[7+i/8] &= ^byte(1 << (i % 8))
		}
	}
	SetChecksum(c.tx)
}

func (c *WriteCoilsCmd) ByteCount() int {
	return int(c.tx[6])
}

func (c *WriteCoilsCmd) Byte(i int) byte {
	if i < 0 || i >= c.ByteCount() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	return c.tx[7+i]
}

// Warning: care must be taken to not set bit outside valid range.
func (c *WriteCoilsCmd) SetByte(i int, b byte) {
	if i < 0 || i >= c.ByteCount() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	c.tx[7+i] = b
	SetChecksum(c.tx)
}

func (c *WriteCoilsCmd) Bytes() []byte {
	return c.tx[7 : 7+c.ByteCount()]
}

// Warning: care must be taken to not set bit outside valid range.
func (c *WriteCoilsCmd) ModifyBytes(f func(b []byte)) {
	f(c.Bytes())
	SetChecksum(c.tx)
}

func (c *WriteCoilsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) == 8 && checksum(c.rx) &&
			bytes.Equal(c.rx[:6], c.tx[:6]))
}

func (c *WriteCoilsCmd) String() string {
	if cap(c.rx) > 0 {
		if c.IsValidRx() {
			l := 10 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
				c.Count()*2 + c.Count()/5
			if c.Count() > 10 {
				l += 2
			}
			if err := c.Err(); err != nil {
				l += daLen(c.rx[0]) + 26
			} else {
				l += daLen(c.rx[0]) + 7 + aLen(c.addr()) + cLen(c.count())
			}
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = c.aRx(b)
			noteAlloc(l)
			return unsafe.String(&b[0], len(b))
		} else {
			h := hexs(c.rx)
			l := 9 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
				c.Count()*2 + c.Count()/5
			if c.Count() > 10 {
				l += 2
			}
			l += 3 + h.Len()
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = append(b, '[')
			b = h.Append(b)
			b = append(b, ']')
			return unsafe.String(&b[0], len(b))
		}
	} else {
		return c.Tx()
	}
}

func (c *WriteCoilsCmd) Tx() string {
	//  <- 2
	// WC  3
	// ' ' 1
	// ':' 1
	//  [] 2
	// -----+
	//     9
	l := 9 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
		c.Count()*2 + c.Count()/5
	if c.Count() > 10 {
		l += 2
	}
	b := c.aTx(make([]byte, 0, l))
	noteAlloc(l)
	return unsafe.String(&b[0], len(b))
}

func (c *WriteCoilsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-WC  "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	n := c.Count()
	b = strconv.AppendInt(b, int64(n), 10)
	b = append(b, '[')
	for i := 0; i < n; i++ {
		if i == 0 && n > 10 {
			b = append(b, '\n')
			b = append(b, ' ')
		}
		if i > 0 {
			if i%10 == 0 {
				b = append(b, '\n')
				b = append(b, ' ')
			} else {
				b = append(b, ' ')
				if i%5 == 0 {
					b = append(b, ' ')
				}
			}
		}
		if c.Coil(i) {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
	}
	if n > 10 {
		b = append(b, '\n')
	}
	return append(b, ']')
}

func (c *WriteCoilsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// WC   3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// WC  3
		// ' ' 1
		// ':' 1
		// -----+
		//     7
		l += 7 + aLen(c.addr()) + cLen(c.count())
	}
	b := c.aRx(make([]byte, 0, l))
	noteAlloc(l)
	return unsafe.String(&b[0], len(b))
}

func (c *WriteCoilsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->WC  "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		b = strconv.AppendInt(b, int64(c.addr()), 10)
		b = append(b, ':')
		return strconv.AppendInt(b, int64(c.count()), 10)
	}
}

func (c *WriteCoilsCmd) addr() uint16 {
	return (uint16(c.rx[2]) << 8) | uint16(c.rx[3])
}

func (c *WriteCoilsCmd) count() int {
	return (int(c.rx[4]) << 8) | int(c.rx[5])
}

//----------------------------------------------------------------------

type WriteRegsCmd struct {
	cmd
}

func NewWriteRegsCmd(devAddr byte, addr uint16, values []uint16) *WriteRegsCmd {
	if len(values) == 0 {
		panic("empty values")
	}
	if len(values) > 123 {
		panic(fmt.Sprintf("values too many: %d", len(values)))
	}
	count := uint16(len(values))
	if addr+count-1 < addr {
		panic(fmt.Sprintf("address overflow: %d, %d", addr, count))
	}

	l := count * 2
	tx := make([]byte, l+9)
	tx[0] = devAddr
	tx[1] = 16
	tx[2] = byte(addr >> 8)
	tx[3] = byte(addr)
	// tx[4] always 0
	tx[5] = byte(count)
	tx[6] = byte(l)
	for i, v := range values {
		tx[7+i*2] = byte(v >> 8)
		tx[8+i*2] = byte(v)
	}
	SetChecksum(tx)

	var rx []byte
	if devAddr > 0 {
		rx = make([]byte, 0, 8)
	}

	return &WriteRegsCmd{cmd{
		tx: tx,
		rx: rx,
	}}
}

func (c *WriteRegsCmd) SetDevAddr(x byte) {
	if c.tx[0] == 0 && x != 0 {
		c.rx = make([]byte, 0, 8)
	} else if c.tx[0] != 0 && x == 0 {
		c.rx = nil
	}

	c.tx[0] = x
	SetChecksum(c.tx)
}

func (c *WriteRegsCmd) Count() int {
	return int(c.tx[5])
}

func (c *WriteRegsCmd) Reg(i int) uint16 {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	return (uint16(c.tx[7+i*2]) << 8) | uint16(c.tx[7+i*2+1])
}

func (c *WriteRegsCmd) SetReg(i int, v uint16) {
	if i < 0 || i >= c.Count() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	c.tx[7+i*2] = byte(v >> 8)
	c.tx[7+i*2+1] = byte(v)
	SetChecksum(c.tx)
}

func (c *WriteRegsCmd) ByteCount() int {
	return int(c.tx[6])
}

func (c *WriteRegsCmd) Byte(i int) byte {
	if i < 0 || i >= c.ByteCount() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	return c.tx[7+i]
}

func (c *WriteRegsCmd) SetByte(i int, b byte) {
	if i < 0 || i >= c.ByteCount() {
		panic(fmt.Sprintf("invalid i: %d", i))
	}
	c.tx[7+i] = b
	SetChecksum(c.tx)
}

func (c *WriteRegsCmd) Bytes() []byte {
	return c.tx[7 : 7+c.ByteCount()]
}

func (c *WriteRegsCmd) ModifyBytes(f func(b []byte)) {
	f(c.Bytes())
	SetChecksum(c.tx)
}

func (c *WriteRegsCmd) IsValidRx() bool {
	return c.isValidErr() ||
		(len(c.rx) == 8 && checksum(c.rx) &&
			bytes.Equal(c.rx[:6], c.tx[:6]))
}

func (c *WriteRegsCmd) String() string {
	if cap(c.rx) > 0 {
		if c.IsValidRx() {
			l := 10 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
				c.Count()*6 + c.Count()/5*2 + 1
			if c.Count() > 10 {
				l += 2
			}
			if err := c.Err(); err != nil {
				l += daLen(c.rx[0]) + 26
			} else {
				l += daLen(c.rx[0]) + 7 + aLen(c.addr()) + cLen(c.count())
			}
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = c.aRx(b)
			noteAlloc(l)
			return unsafe.String(&b[0], len(b))
		} else {
			h := hexs(c.rx)
			l := 9 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
				c.Count()*6 + c.Count()/5*2 + 1
			if c.Count() > 10 {
				l += 2
			}
			l += 3 + h.Len()
			noteAlloc(l)
			b := make([]byte, 0, l)
			b = c.aTx(b)
			b = append(b, '\n')
			b = append(b, '[')
			b = h.Append(b)
			b = append(b, ']')
			return unsafe.String(&b[0], len(b))
		}
	} else {
		return c.Tx()
	}
}

func (c *WriteRegsCmd) Tx() string {
	//  <- 2
	// WR  3
	// ' ' 1
	// ':' 1
	//  [] 2
	// -----+
	//     9
	l := 9 + daLen(c.DevAddr()) + aLen(c.Addr()) + cLen(c.Count()) +
		c.Count()*6 + c.Count()/5*2 + 1
	if c.Count() > 10 {
		l += 2
	}
	b := c.aTx(make([]byte, 0, l))
	noteAlloc(l)
	return unsafe.String(&b[0], len(b))
}

func (c *WriteRegsCmd) aTx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.DevAddr()), 10)
	b = append(b, "<-WR  "...)
	b = strconv.AppendInt(b, int64(c.Addr()), 10)
	b = append(b, ':')
	n := c.Count()
	b = strconv.AppendInt(b, int64(n), 10)
	b = append(b, '[')
	var x [5]byte
	for i := 0; i < n; i++ {
		if i == 0 && n > 10 {
			b = append(b, '\n')
			b = append(b, ' ')
		}
		if i > 0 {
			if i%10 == 0 {
				b = append(b, '\n')
				b = append(b, ' ')
			} else {
				b = append(b, ' ')
				if i%5 == 0 {
					b = append(b, ':')
					b = append(b, ' ')
				}
			}
		}
		t := strconv.AppendInt(x[:0], int64(c.Reg(i)), 10)
		for j := len(t); j < 5; j++ {
			b = append(b, ' ')
		}
		b = append(b, t...)
	}
	if n > 10 {
		b = append(b, '\n')
	}
	return append(b, ']')
}

func (c *WriteRegsCmd) Rx() string {
	l := daLen(c.rx[0])
	if err := c.Err(); err != nil {
		//  ->  2
		// WR   3
		// ' '  1
		// err 20
		// ------+
		//     26
		l += 26
	} else {
		//  -> 2
		// WR  3
		// ' ' 1
		// ':' 1
		// -----+
		//     7
		l += 7 + aLen(c.addr()) + cLen(c.count())
	}
	b := c.aRx(make([]byte, 0, l))
	noteAlloc(l)
	return unsafe.String(&b[0], len(b))
}

func (c *WriteRegsCmd) aRx(b []byte) []byte {
	b = strconv.AppendInt(b, int64(c.rx[0]), 10)
	b = append(b, "->WR  "...)
	if err := c.Err(); err != nil {
		return append(b, err.Error()...)
	} else {
		b = strconv.AppendInt(b, int64(c.addr()), 10)
		b = append(b, ':')
		return strconv.AppendInt(b, int64(c.count()), 10)
	}
}

func (c *WriteRegsCmd) addr() uint16 {
	return (uint16(c.rx[2]) << 8) | uint16(c.rx[3])
}

func (c *WriteRegsCmd) count() int {
	return (int(c.rx[4]) << 8) | int(c.rx[5])
}

//----------------------------------------------------------------------

func daLen(a byte) int {
	if a < 10 {
		return 1
	} else if a < 100 {
		return 2
	} else {
		return 3
	}
}

func aLen(a uint16) int {
	if a < 10 {
		return 1
	} else if a < 100 {
		return 2
	} else if a < 1000 {
		return 3
	} else if a < 10000 {
		return 4
	} else {
		return 5
	}
}

func cLen(c int) int {
	if c < 10 {
		return 1
	} else if c < 100 {
		return 2
	} else if c < 1000 {
		return 3
	} else {
		return 4
	}
}
