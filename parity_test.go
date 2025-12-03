package rtu_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/bangzek/modbus-rtu"
)

var _ = Describe("Parity", func() {
	DescribeTable("valid",
		func(p Parity, s string) {
			Expect(p.IsValid()).To(BeTrue())
			Expect(p.String()).To(Equal(s))

			j := `"` + s + `"`
			Expect(json.Marshal(p)).To(Equal([]byte(j)))

			var n Parity
			Expect(json.Unmarshal([]byte(j), &n)).To(Succeed())
			Expect(n).To(Equal(p))
		},
		Entry(nil, NoParity, "NONE"),
		Entry(nil, OddParity, "ODD"),
		Entry(nil, EvenParity, "EVEN"),
	)

	Context("invalid", func() {
		p := EvenParity + 1

		It("is invalid", func() {
			Expect(p.IsValid()).To(BeFalse())
		})
		It("has ERR string", func() {
			Expect(p.String()).To(Equal("ERR:3"))
		})
		It("can't be marshal-ed", func() {
			_, err := p.MarshalText()
			Expect(err).To(MatchError("Invalid Parity: 3"))
		})
		It("can't be marshal-ed", func() {
			_, err := p.MarshalText()
			Expect(err).To(MatchError("Invalid Parity: 3"))
		})
		It("can't be unmarshal-ed", func() {
			var n Parity
			Expect(n.UnmarshalText([]byte("ERR:3"))).
				To(MatchError(`Invalid Parity from "ERR:3"`))
		})
	})
})
