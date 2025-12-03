package rtu_test

import (
	"errors"
	"fmt"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bangzek/clock"
	. "github.com/bangzek/modbus-rtu"
)

var _ = Describe("Controller", func() {
	const dsn = clock.DefaultScriptNow
	Context("single send", func() {
		It("runs just fine", func() {
			cmd := NewReadCoilsCmd(3, 2, 1)
			rwc := &MockRwc{
				Writes: []WriteScript{
					{8, nil},
				},
				Reads: []ReadScript{
					{[]byte{3, 1, 1, 0b1, 0x91, 0xf0}, nil},
				},
			}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd)).To(Succeed())
			con.Close()
			Expect(port.Calls).To(Equal([]bool{false}))
			Expect(rwc.Calls).To(Equal([]string{
				"WRITE [03 01 00 02 00 01 5D E8]",
				"READ",
				"CLOSE",
			}))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 01 00 02 00 01 5D E8",
				"D:TX: 3<-RC  2:1",
				"D:rx: 03 01 01 01 91 F0",
				"D:RX: 3->RC  1[1]",
			}))
		})
	})

	Context("two send", func() {
		It("runs just fine", func() {
			cmd1 := NewReadDInputsCmd(3, 2, 1)
			cmd2 := NewWriteCoilCmd(0, 258, true)
			rwc := &MockRwc{
				Writes: []WriteScript{
					{8, nil},
					{8, nil},
				},
				Reads: []ReadScript{
					{nil, nil},
					{[]byte{3, 2}, nil},
					{[]byte{1, 0b1, 0x61, 0xf0}, nil},
				},
			}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd1)).To(Succeed())
			Expect(con.Send(cmd2)).To(Succeed())
			Expect(port.Calls).To(Equal([]bool{false}))
			Expect(rwc.Calls).To(Equal([]string{
				"WRITE [03 02 00 02 00 01 19 E8]",
				"READ",
				"READ",
				"READ",
				"WRITE [00 05 01 02 FF 00 2D D7]",
			}))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 02 00 02 00 01 19 E8",
				"D:TX: 3<-RDI 2:1",
				"D:rx: 03 02 01 01 61 F0",
				"D:RX: 3->RDI 1[1]",
				"D:tx: 00 05 01 02 FF 00 2D D7",
				"D:TX: 0<-W1C 258 true",
			}))
		})
	})

	Context("error on open", func() {
		It("returns that err", func() {
			cmd1 := NewReadDInputsCmd(3, 2, 1)
			err1 := errors.New("one")
			cmd2 := NewWriteCoilCmd(0, 258, true)
			err2 := errors.New("two")
			port := &MockPort{
				Opens: []OpenScript{
					{nil, SERIAL_WAIT, err1},
					{nil, SERIAL_WAIT, err2},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd1)).To(MatchError(err1))
			Expect(con.Send(cmd2)).To(MatchError(err2))
			Expect(port.Calls).To(Equal([]bool{false, true}))
			Expect(log.Msgs).To(BeEmpty())
		})
	})

	Context("error on tx", func() {
		It("returns that err", func() {
			cmd1 := NewReadDInputsCmd(3, 2, 1)
			err1 := errors.New("one")
			cmd2 := NewWriteCoilCmd(0, 258, true)
			rwc1 := &MockRwc{Writes: []WriteScript{{8, err1}}}
			rwc2 := &MockRwc{Writes: []WriteScript{{5, nil}}}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc1, SERIAL_WAIT, nil},
					{rwc2, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd1)).To(MatchError(err1))
			Expect(con.Send(cmd2)).To(MatchError(io.ErrShortWrite))
			Expect(port.Calls).To(Equal([]bool{false, false}))
			Expect(rwc1.Calls).To(Equal([]string{
				"WRITE [03 02 00 02 00 01 19 E8]",
				"CLOSE",
			}))
			Expect(rwc2.Calls).To(Equal([]string{
				"WRITE [00 05 01 02 FF 00 2D D7]",
				"CLOSE",
			}))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 02 00 02 00 01 19 E8",
				"D:TX: 3<-RDI 2:1",
				"D:tx: 00 05 01 02 FF 00 2D D7",
				"D:TX: 0<-W1C 258 true",
			}))
		})
	})

	Context("error on rx", func() {
		It("returns that err", func() {
			cmd := NewReadCoilsCmd(3, 2, 1)
			err := errors.New("something")
			rwc := &MockRwc{
				Writes: []WriteScript{
					{8, nil},
				},
				Reads: []ReadScript{
					{[]byte{3, 1, 1, 0b1, 0x91, 0xf0}, err},
				},
			}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd)).To(MatchError(err))
			Expect(port.Calls).To(Equal([]bool{false}))
			Expect(rwc.Calls).To(Equal([]string{
				"WRITE [03 01 00 02 00 01 5D E8]",
				"READ",
				"CLOSE",
			}))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 01 00 02 00 01 5D E8",
				"D:TX: 3<-RC  2:1",
			}))
		})
	})

	Context("bad rx", func() {
		It("returns BadRxErr", func() {
			rx := []byte{3, 1, 1, 0b1, 0x91, 0xf1}
			cmd := NewReadCoilsCmd(3, 2, 1)
			rwc := &MockRwc{
				Writes: []WriteScript{
					{8, nil},
				},
				Reads: []ReadScript{
					{rx, nil},
				},
			}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd)).To(MatchError("invalid response: [03 01 01 01 91 F1]"))
			Expect(port.Calls).To(Equal([]bool{false}))
			Expect(rwc.Calls).To(Equal([]string{
				"WRITE [03 01 00 02 00 01 5D E8]",
				"READ",
				"CLOSE",
			}))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 01 00 02 00 01 5D E8",
				"D:TX: 3<-RC  2:1",
				"D:rx: 03 01 01 01 91 F1",
			}))
		})
	})

	Context("timeout", func() {
		It("returns ErrTimeout", func() {
			t := time.Date(2024, time.March, 2, 10, 11, 12, 0, time.UTC)
			mc := new(clock.Mock)
			mc.NowScripts = []time.Duration{
				0, 0, TIMEOUT,
			}
			SetClock(mc)
			mc.Start(t)
			cmd := NewReadCoilsCmd(3, 2, 1)
			rwc := &MockRwc{
				Writes: []WriteScript{
					{8, nil},
				},
				Reads: []ReadScript{
					{nil, nil},
				},
			}
			port := &MockPort{
				Opens: []OpenScript{
					{rwc, SERIAL_WAIT, nil},
				},
			}
			con := &Controller{
				Port: port,
			}
			log := NewLog()
			Expect(con.Send(cmd)).To(MatchError(ErrTimeout))
			Expect(port.Calls).To(Equal([]bool{false}))
			Expect(rwc.Calls).To(Equal([]string{
				"WRITE [03 01 00 02 00 01 5D E8]",
				"READ",
				"READ",
				"CLOSE",
			}))
			mc.Stop()
			Expect(mc.Calls()).To(HaveExactElements(
				"now",
				"now",
				"now",
			))
			Expect(mc.Times()).To(HaveExactElements(
				t.Add(dsn),
				t.Add(2*dsn),
				t.Add(2*dsn+TIMEOUT),
			))
			Expect(log.Msgs).To(Equal([]string{
				"D:tx: 03 01 00 02 00 01 5D E8",
				"D:TX: 3<-RC  2:1",
			}))
		})
	})
})

type MockPort struct {
	Opens []OpenScript

	Calls []bool
	i     int
}

type OpenScript struct {
	Rwc  io.ReadWriteCloser
	Wait time.Duration
	Err  error
}

func (m *MockPort) Open(
	repeat bool,
) (rwc io.ReadWriteCloser, wait time.Duration, err error) {
	if m.i < len(m.Opens) {
		rwc = m.Opens[m.i].Rwc
		wait = m.Opens[m.i].Wait
		err = m.Opens[m.i].Err
	}
	m.i++
	m.Calls = append(m.Calls, repeat)
	return
}

type MockRwc struct {
	Writes []WriteScript
	Reads  []ReadScript

	Calls []string

	iWrite int
	iRead  int
}

type WriteScript struct {
	N   int
	Err error
}

type ReadScript struct {
	Bytes []byte
	Err   error
}

func (m *MockRwc) Write(b []byte) (n int, err error) {
	if m.iWrite < len(m.Writes) {
		n = m.Writes[m.iWrite].N
		err = m.Writes[m.iWrite].Err
	}
	m.Calls = append(m.Calls, fmt.Sprintf("WRITE [% X]", b))
	m.iWrite++
	return
}

func (m *MockRwc) Read(b []byte) (n int, err error) {
	if m.iRead < len(m.Reads) {
		s := m.Reads[m.iRead]
		if len(b) < len(s.Bytes) {
			panic(fmt.Sprintf("Invalid MockRwc.ReadScript[%d].Bytes %d>%d",
				m.iRead, len(s.Bytes), len(b)))
		}
		if len(s.Bytes) > 0 {
			copy(b, s.Bytes)
			n = len(s.Bytes)
		}
		err = s.Err
	}
	m.Calls = append(m.Calls, "READ")
	m.iRead++
	return
}

func (m *MockRwc) Close() error {
	m.Calls = append(m.Calls, "CLOSE")
	return nil
}
