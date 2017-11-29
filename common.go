package gsocks

import (
	"net"
	"io"
	"strconv"
	"encoding/binary"
	"fmt"
	"strings"
	"errors"
)

const socks5Version = 5

const (
	authNone     = 0
	authPassword = 2
)

const (
	cmdConnect  = 1
	cmdUDPAssoc = 3
)

const (
	addrIp4    = 1
	addrDomain = 3
	addrIp6    = 4
)

const (
	errFail                    = iota
	errForbidden
	errNetworkUnreachable
	errHostUnreachable
	errConnectRefused
	errTTLExpired
	errCommandNotSupported
	errAddressTypeNotSupported
)

type state string

const (
	stateHello      = "Hello"
	stateAuth       = "Auth"
	stateRequest    = "Request"
	stateTCPChannel = "TCPChannel"
	stateUDPRelay   = "UDPRelay"
	stateError      = "Error"
	stateTerminal   = "Terminal"
)

var socks5ErrString = []string{
	"",
	"general failure",
	"connection forbidden",
	"network unreachable",
	"host unreachable",
	"connection refused",
	"ttl expired",
	"command not supported",
	"address type not supported",
}

type Client struct {
	*channel
	state  state
	addr   string
	server *Socks5Server
	error  *socks5Error
}

type socks5Error struct {
	error
	code uint8
}

func newClient(s *Socks5Server, t *channel) (c *Client) {
	c = new(Client)
	c.channel = t
	c.server = s
	c.error = &socks5Error{}
	c.setState(stateHello)
	return
}

func (client *Client) hello() {
	buf := make([]byte, 2)

	if _, err := io.ReadFull(client, buf); err != nil {
		logDebug.Println(err.Error())
		client.setTerminal("Read ver and num of methods failed")
		return
	}
	// VER
	if buf[0] != socks5Version {
		client.setTerminal(fmt.Sprintf("Unsupported socks version %s", buf[0]))
		return
	}

	// NMETHODs
	l := buf[1]
	if l == 0 {
		client.setTerminal("Methods length is 0")
		return
	}

	// METHODS
	bl := make([]byte, l)
	if _, err := io.ReadFull(client, bl); err != nil {
		logDebug.Println(err.Error())
		client.setTerminal("Read methods failed")
		return
	}

	// todo: auth method

	// METHOD
	buf[1] = authNone

	if _, err := client.Write(buf); err != nil {
		client.setTerminal("Write methods to client failed")
		return
	}

	client.setState(stateRequest)

}

func (client *Client) auth() {

}

func (client *Client) request() {
	// request = VER + CMD + RSV + ATYP + DST.ADDR + DST.PORT
	buf := make([]byte, 4)
	if _, err := io.ReadFull(client, buf); err != nil {
		logDebug.Println(err.Error())
		client.setTerminal("Read request failed")
		return
	}

	// VER
	if buf[0] != socks5Version {
		client.setError(errFail)
		return
	}

	// CMD
	switch buf[1] {
	case cmdConnect:
		client.setState(stateTCPChannel)
	case cmdUDPAssoc:
		if client.server.udp {
			client.setState(stateUDPRelay)
		} else {
			client.setError(errCommandNotSupported)
		}
	default:
		client.setError(errCommandNotSupported)
	}

	// ATYP
	t := buf[3]
	l := 0
	switch t {
	case addrIp4:
		l = net.IPv4len
	case addrIp6:
		l = net.IPv6len
	case addrDomain:
		if _, err := io.ReadFull(client, buf[:1]); err != nil {
			logDebug.Println(err.Error())
			client.setTerminal("Read dstList dstList length failed")
			return
		}
		l = int(buf[0])
	default:
		client.setError(errAddressTypeNotSupported)
		return
	}

	l += 2
	if l > len(buf) {
		buf = append(buf, make([]byte, l-len(buf))...)
	}
	if _, err := io.ReadFull(client, buf); err != nil {
		logDebug.Println(err.Error())
		client.setTerminal("Read dstList dstList and port failed")
		return
	}

	// DST.ADDR, DST.PORT
	client.addr, _ = decodeAddr(buf, t)
	logDebug.Printf("Request address is %s\n", client.addr)
}

func (client *Client) response() {
	// response = VER + REP + RSV + ATYP + ADDR + PORT

	buf := make([]byte, 4+4+2)
	buf[0] = socks5Version

	switch client.state {
	case stateTCPChannel:
		buf[3] = addrIp4
	case stateUDPRelay:
		bAddr, t, _ := encodeAddr(client.server.addr)
		buf[3] = t
		buf = append(buf[:4], bAddr...)
	case stateError:
		buf[3] = addrIp4
		buf[1] = client.error.code
		client.setState(stateTerminal)
	default:
		logDebug.Printf("Invalid client state %s", client.state)
	}
	if _, err := client.Write(buf); err != nil {
		logDebug.Println(err.Error())
		client.setTerminal(fmt.Sprintf("Write response to %s failed", client.RemoteAddr()))
	}
	logDebug.Printf("Send response to %s\n", client.RemoteAddr())
}

func (client *Client) tcpChannel() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", client.addr)
	if err != nil {
		logDebug.Println(err)
		client.setError(errHostUnreachable)
		return
	}

	var r *net.TCPConn
	for i := 0; i < 5; i++ { // if temporary error,retry 5 times
		r, err = net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			logDebug.Println(err.Error())
			if e, ok := err.(net.Error); ok {
				if e.Temporary() {
					continue
				}
				client.setError(errNetworkUnreachable)
				return
			}
			client.setError(errFail)
			return
		}
		break
	}

	remote := newChannel(r, client.server.timeout)

	defer remote.Close()

	client.response()
	if client.state != stateTCPChannel {
		return
	}

	ch := make(chan error, 2)
	go setChannel(client.channel, remote, ch)
	go setChannel(remote, client.channel, ch)

	for i := 0; i < 2; i++ {
		if e := <-ch; e != nil {
			logWarn.Println(e.Error())
		}
	}

	client.setState(stateTerminal)
	logInfo.Printf("TCP channel between %s and %s was completed", client.RemoteAddr(), remote.RemoteAddr())
}

func (client *Client) udpRelay() {
	client.response()
	if client.state != stateUDPRelay {
		return
	}
	//for {
	//	if _, err :=client.TCPConn.Read(make([]byte,1));err!=nil{
	//		break
	//	}
	//}
	client.setState(stateTerminal)
}

func (client *Client) setTerminal(msg string) {
	logWarn.Println(msg)
	client.setState(stateTerminal)
}

func (client *Client) setError(code uint8) {
	if code == 0 {
		client.error = nil
	} else {
		client.error = &socks5Error{errors.New(socks5ErrString[code]), code}

	}
	client.setState(stateError)
}

func (client *Client) setState(s state) {
	client.state = s
	logDebug.Printf("Client[%s] state change to %v", client.RemoteAddr(), s)
}

func encodeAddr(addr string) (bAddr []byte, t uint8, err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}
	ba := net.ParseIP(host)
	t = 0
	if ba == nil {
		bAddr = []byte(host)
		l := len(bAddr)
		bl := make([]byte, 1)
		bl[0] = uint8(l)
		bAddr = append(bl, bAddr...)
		t = addrDomain
	} else {
		if strings.Contains(host, ".") {
			t = addrIp4
			bAddr = ba.To4()
		} else {
			t = addrIp6
			bAddr = ba.To16()
		}
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return
	}
	bPort := make([]byte, 2)
	binary.BigEndian.PutUint16(bPort, uint16(p))
	bAddr = append(bAddr, bPort...)

	return
}

func decodeAddr(p []byte, t uint8) (addr string, err error) {
	ba, bp := p[:len(p)-2], p[len(p)-2:]

	switch t {
	case addrIp4, addrIp6:
		addr = net.IP(ba).String()
	case addrDomain:
		addr = string(ba)
	default:
		err = fmt.Errorf("invaild address type")
		return
	}
	addr = net.JoinHostPort(
		addr, strconv.FormatUint(
			uint64(binary.BigEndian.Uint16(bp)), 10))
	return
}
