package main

import (
	"fmt"
	"log"
	"os"
	"time"

	rtu "github.com/bangzek/modbus-rtu"
)

func main() {
	rtu.InfoLogFunc = log.Printf
	rtu.DebugLogFunc = log.Printf

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s DEV\n"+
			" e.g.: %s /dev/ttyM1\n",
			os.Args[0],
			os.Args[0])
		os.Exit(1)
	}

	con := &rtu.Controller{
		Port: &rtu.SerialPort{
			Dev: os.Args[1],
		},
	}

	demoWellPro(con)
	//demoRFID(con)
	//demoTemp(con)
}

// This is for 2 Well Pro WP****ADAM DI-DO
func demoWellPro(con *rtu.Controller) {
	on := []bool{true, true, true, true, true, true, true, true}
	off := []bool{false, false, false, false, false, false, false, false}
	on1 := rtu.NewWriteCoilsCmd(1, 0, on)
	on2 := rtu.NewWriteCoilsCmd(2, 0, on)
	off1 := rtu.NewWriteCoilsCmd(1, 0, off)
	off2 := rtu.NewWriteCoilsCmd(2, 0, off)
	read := rtu.NewReadDInputsCmd(1, 0, 2)

	tick := time.NewTicker(time.Second / 5)

	if err := con.Send(on1); err != nil {
		log.Fatalf("ERR: %s\n", err)
	}
	if err := con.Send(on2); err != nil {
		log.Fatalf("ERR: %s\n", err)
	}
	time.Sleep(time.Second)
	if err := con.Send(off1); err != nil {
		log.Fatalf("ERR: %s\n", err)
	}
	if err := con.Send(off2); err != nil {
		log.Fatalf("ERR: %s\n", err)
	}

	for {
		<-tick.C
		if err := con.Send(read); err != nil {
			log.Fatalf("ERR: %s\n", err)
		}
		if read.Input(0) {
			if err := con.Send(on1); err != nil {
				log.Fatalf("ERR: %s\n", err)
			}
		} else {
			if err := con.Send(off1); err != nil {
				log.Fatalf("ERR: %s\n", err)
			}
		}
		if read.Input(1) {
			if err := con.Send(on2); err != nil {
				log.Fatalf("ERR: %s\n", err)
			}
		} else {
			if err := con.Send(off2); err != nil {
				log.Fatalf("ERR: %s\n", err)
			}
		}
		fmt.Println()
	}
}

// This is for AR14x1M/R ISO 15693 RFID reader
func demoRFID(con *rtu.Controller) {
	rh := rtu.NewReadHRegsCmd(2, 0x2000, 4)

	tick := time.NewTicker(time.Second / 2)
	for {
		<-tick.C
		if err := con.Send(rh); err != nil {
			log.Printf("ERR: %s\n", err)
			fmt.Println()
			continue
		}
		log.Printf("ID: % X\n", rh.Bytes())
		fmt.Println()
	}
}

// This is for SHT20 Temperature and Humidity Sensor
func demoTemp(con *rtu.Controller) {
	ri := rtu.NewReadIRegsCmd(1, 1, 2)

	tick := time.NewTicker(time.Second)
	for {
		<-tick.C
		if err := con.Send(ri); err != nil {
			log.Printf("ERR: %s\n", err)
			fmt.Println()
			continue
		}
		log.Printf("Temp %g Humid %g\n",
			float64(ri.Reg(0))/10,
			float64(ri.Reg(1))/10)
		fmt.Println()
	}
}
