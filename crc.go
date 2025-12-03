package rtu

import "github.com/sigurn/crc16"

var crcTable = crc16.MakeTable(crc16.CRC16_MODBUS)

func SetChecksum(b []byte) {
	cs := crc16.Checksum(b[:len(b)-2], crcTable)
	b[len(b)-2] = byte(cs)
	b[len(b)-1] = byte(cs >> 8)
}

func checksum(b []byte) bool {
	cs := crc16.Checksum(b[:len(b)-2], crcTable)
	return b[len(b)-2] == byte(cs) && b[len(b)-1] == byte(cs>>8)
}
