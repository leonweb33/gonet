package udpp

import (
	"ipv4p"
	//"logs"
)

type UDP_Writer struct {
	ipAddress string // destination ip address
	writer    *ipv4p.IP_Writer
	src, dst  uint16 // ports
}

func NewUDP_Writer(src, dest uint16, dstIP string) (*UDP_Writer, error) {
	write, err := ipv4p.NewIP_Writer(dstIP, ipv4p.UDP_PROTO)
	if err != nil {
		return nil, err
	}

	return &UDP_Writer{src: src, dst: dest, ipAddress: dstIP, writer: write}, nil
}

func (c *UDP_Writer) write(x []byte) error {
	UDPHeader := []byte{
		(byte)(c.src >> 8), (byte)(c.src), // Source port in byte slice
		(byte)(c.dst >> 8), (byte)(c.dst), // Destination port in byte slice
		(byte)((8 + len(x)) >> 8), (byte)(8 + len(x)), // Length in bytes of UDP header + data
		0, 0, // Checksum TODO: calculate the checksum correctly
	}

	x = append(UDPHeader, x...)

	return c.writer.WriteTo(x)
}
func (c *UDP_Writer) close() error {
	return nil
}