package main

import (
    "errors"
    "fmt"
    "net"
    "time"
//    "syscall"
//"golang.org/x/net/ipv4"
)

type IP_Reader struct {
    incomingPackets chan []byte
    nr              *Network_Reader
    protocol        uint8
    ip              string
    fragBuf         map[string](chan []byte)
}

func (nr *Network_Reader) NewIP_Reader(ip string, protocol uint8) (*IP_Reader, error) {
    c, err := nr.bind(ip, protocol)
    if err != nil {
        return nil, err
    }

    return &IP_Reader{
        incomingPackets: c,
        nr:              nr,
        protocol:        protocol,
        ip:              ip,
        fragBuf:         make(map[string](chan []byte)),
    }, nil
}

func slicePacket(b []byte) (hrd, payload []byte) {
    hdrLen := int(b[0]&0x0f) * 4
    //fmt.Println("HdrLen: ", hdrLen)
    return b[:hdrLen], b[hdrLen:]
}

const FRAGMENT_TIMEOUT = 15

func (ipr *IP_Reader) ReadFrom() (ip string, b, payload []byte, e error) {
    //fmt.Println("STARTING READ")
    b = <-ipr.incomingPackets
    //fmt.Println("RAW READ COMPLETED")
    fmt.Println("Read Length: ", len(b))
    //fmt.Println("Full Read Data: ", b)

    hdr, p := slicePacket(b)

    // extract source IP and protocol
    ip = net.IPv4(hdr[12], hdr[13], hdr[14], hdr[15]).String()
    proto := uint8(hdr[9])
    if !((ipr.ip == ip || ipr.ip == "*") && ipr.protocol == proto) {
        fmt.Println("Not interested in packet: dropping.")
        return ipr.ReadFrom()
    }

    // TODO: verify the checksum outside, not inside

    packetOffset := uint16(hdr[6]&0x1f)<<8 + uint16(hdr[7])
    //fmt.Println("PACK OFF", packetOffset, "HEADER FLAGS", (hdr[6] >> 5))
    if ((hdr[6]>>5)&0x01 == 0) && (packetOffset == 0) { // if not fragment
        // verify checksum
        if verifyChecksum(hdr) != 0 {
            //fmt.Println("Header checksum verification failed. Packet dropped.")
            //fmt.Println("Wrong header: ", hdr)
            //fmt.Println("Payload (dropped): ", p)
            // TODO: return a different packet after dropping instead of returning an error
            return "", nil, nil, errors.New("Header checksum incorrect, packet dropped")
        }

        //fmt.Println("Payload Length: ", len(p))
        //fmt.Println("Full payload: ", p)
        //fmt.Println("PACKET COMPLETELY READ")
        return ip, b, p, nil
    } else {
        bufID := string([]byte{hdr[12], hdr[13], hdr[14], hdr[15], // the source IP
            hdr[16], hdr[17], hdr[18], hdr[19], // the destination IP
            hdr[9],         // protocol
            hdr[4], hdr[5], // identification
        })

        if c, ok := ipr.fragBuf[bufID]; ok {
            // the fragment has already started
            go func() { c <- b }()
        } else {
            // create the fragment buffer
            ipr.fragBuf[bufID] = make(chan []byte)

            // create the packet assembler in a goroutine to allow the program to continue
            go func(in <-chan []byte, finished chan<- []byte) {
                payload := make([]byte)
                extraFrags := make(map[uint64]([]byte))
                t := time.Now()
                recvLast := false
                for time.Since(t).Seconds() <= FRAGMENT_TIMEOUT {
                    select {
                    case frag := <-in:
                        hdr, p := slicePacket(frag)
                        //offset := 8 * (uint64(hdr[6]&0x1f)<<8 + uint64(hdr[7]))
                    //fmt.Println("RECEIVED FRAG")
                    //fmt.Println("Offset:", offset)
                    //fmt.Println(len(payload))

                    // add to map
                        extraFrags[8*(uint64(p[6])<<3>>11+uint64(p[7]))] = p
                        if (hdr[6]>>5)&0x01 == 0 {
                            recvLast = true
                        }
                    // add to payload
                        for storedFrag, found := extraFrags[uint64(len(payload))]; found; {
                            delete(extraFrags, uint64(len(payload)))
                            payload = append(payload, storedFrag...)
                        }
                        if recvLast && len(extraFrags) == 0 {
                            fullPacketHdr := hdr
                            totalLen := uint16(fullPacketHdr[0]&0x0F)*4 + uint16(len(payload))
                            fullPacketHdr[2] = byte(totalLen >> 8)
                            fullPacketHdr[3] = byte(totalLen)
                            fullPacketHdr[6] = 0
                            fullPacketHdr[7] = 0
                            //fullPacketHdr[10] = 0
                            //fullPacketHdr[11] = 0
                            check := calculateChecksum(fullPacketHdr[:20])
                            fullPacketHdr[10] = byte(check >> 8)
                            fullPacketHdr[11] = byte(check)

                            // send the packet back into processing
                            go func() {
                                finished <- append(fullPacketHdr, payload...)
                                //fmt.Println("FINISHED")
                            }()
                            fmt.Println("Just wrote back in")
                            return
                        }
                    default:
                    // make the timeout actually have a chance of being hit
                    }
                }

                // drop the packet upon timeout
                fmt.Println(errors.New("Fragments took too long, packet dropped"))
                return
            }(ipr.fragBuf[bufID], ipr.incomingPackets)

            // send in the first fragment
            ipr.fragBuf[bufID] <- p

            // TODO: Clear/Remove the fragment buffer after some time
        }

        // after dealing with the fragment, try reading again
        //fmt.Println("RECURSE")
        return ipr.ReadFrom()
    }
}

func (ipr *IP_Reader) Close() error {
    return ipr.nr.unbind(ipr.ip, ipr.protocol)
}

/* h := &ipv4.Header{
	Version:  ipv4.Version,      // protocol version
	Len:      20,                // header length
	TOS:      0,                 // type-of-service (0 is everything normal)
	TotalLen: len(x) + 20,       // packet total length (octets)
	ID:       0,                 // identification
	Flags:    ipv4.DontFragment, // flags
	FragOff:  0,                 // fragment offset
	TTL:      8,                 // time-to-live (maximum lifespan in seconds)
	Protocol: 17,                // next protocol (17 is UDP)
	Checksum: 0,                 // checksum (apparently autocomputed)
	//Src:    net.IPv4(127, 0, 0, 1), // source address, apparently done automatically
	Dst: net.ParseIP(c.manager.ipAddress), // destination address
	//Options                         // options, extension headers
}
*/
