package socks

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	socks5Version = 0x05

	authNoneRequired = 0x00
	authNoAcceptable = 0xFF

	CmdConnect = 0x01
	CmdBind = 0x02
	CmdAssociate = 0x03

	ReplySuccess = 0x00
	ReplyGeneralFailure = 0x01
	ReplyCommandNotSupported = 0x07

	addrTypeIPv4 = 0x01
	addrTypeFQDN = 0x03
	addrTypeIPv6 = 0x04
)

type SocksError struct {
	Code uint8
}

type Request struct {
	Version  uint8
	Command  uint8
	Addr	 *Address
}

type Reply struct {
	Version  uint8
	Reply	 uint8
	Addr     *Address
}

type Address struct {
	Raw	[]byte
	Addr    string
	Port    uint16
}

func Handshake(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadAtLeast(conn, header, len(header)); err != nil {
		return err
	}

	if header[0] != socks5Version {
		return fmt.Errorf("Unsupported SOCKS version: %v", header[0])
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	reply := []byte{socks5Version, authNoAcceptable}
	for _, method := range methods {
		if method == authNoneRequired {
			reply[1] = authNoneRequired
		}
	}

	_, err := conn.Write(reply)
	if err != nil {
		return err
	}

	return nil
}

func (addr *Address) String() string {
	return net.JoinHostPort(addr.Addr, fmt.Sprint(addr.Port))
}

func (addr *Address) HostPort() (string, string) {
	return addr.Addr, string(addr.Port)
}

func (addr *Address) Read(r net.Conn) error {
	addr.Raw = make([]byte, 1)
	if _, err := r.Read(addr.Raw); err != nil {
		return err
	}

	switch uint8(addr.Raw[0]) {
	case addrTypeIPv4:
		a := make([]byte, 4)
		if _, err := io.ReadAtLeast(r, a, len(a)); err != nil {
			return err
		}
		addr.Addr = net.IP(a).String()
		addr.Raw = append(addr.Raw, a...)
	case addrTypeFQDN:
		l := make([]byte, 1)
		if _, err := r.Read(l); err != nil {
			return err
		}
		addrLen := uint8(l[0])
		addr.Raw = append(addr.Raw, l[0])

		a := make([]byte, addrLen)
		if _, err := io.ReadAtLeast(r, a, len(a)); err != nil {
			return err
		}
		addr.Addr = string(a)
		addr.Raw = append(addr.Raw, a...)
	case addrTypeIPv6:
		a := make([]byte, 16)
		if _, err := io.ReadAtLeast(r, a, len(a)); err != nil {
			return err
		}
		addr.Addr = net.IP(a).String()
		addr.Raw = append(addr.Raw, a...)
	default:
		return fmt.Errorf("Unsupported address type: %v", addr.Raw[0])
	}

	port := make([]byte, 2)
	if _, err := io.ReadAtLeast(r, port, len(port)); err != nil {
		return err
	}
	addr.Port = binary.BigEndian.Uint16(port)
	addr.Raw = append(addr.Raw, port...)

	return nil
}

func UnmarshalRequest(conn net.Conn) (*Request, error) {
	header := make([]byte, 3)
	if _, err := io.ReadAtLeast(conn, header, len(header)); err != nil {
		return nil, err
	}

	if header[0] != socks5Version {
		return nil, fmt.Errorf("Unsupported SOCKS version: %v", header[0])
	}

	request := &Request{
		Version: header[0],
		Command: header[1],
		Addr: &Address{},
	}

	err := request.Addr.Read(conn)
	if err != nil {
		return nil, err
	}

	return request, nil
}

func SendReply(conn net.Conn, response uint8) error {
	msg := make([]byte, 4)
	msg[0] = socks5Version
	msg[1] = response
	msg[2] = 0
	msg[3] = addrTypeIPv4
	msg = append(msg, 0x00, 0x00, 0x00, 0x00)
	msg = append(msg, 0x00, 0x00)

	_, err := conn.Write(msg)
	if err != nil {
		return err
	}

	return err
}
