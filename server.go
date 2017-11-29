package gsocks

import (
	"log"
	"net"
	"os"
	"time"
)

type Socks5Server struct {
	addr         string
	udp          bool
	timeout      int
	maxClientNum int
	assocAddr    map[string]time.Time
	done         chan struct{}
	listener     *net.TCPListener
	u            *relay
	clientList   []*Client
}

var logDebug, logInfo, logWarn *log.Logger

func init() {
	log.SetPrefix("proxy:")
	logDebug = log.New(os.Stdout, "DEBUG: ", 0)
	logDebug.SetFlags(log.Lshortfile)
	logInfo = log.New(os.Stdout, "INFO: ", 0)
	logWarn = log.New(os.Stdout, "WARN: ", 0)
	logWarn.SetFlags(log.Lshortfile)
}

func New(addr string, udp bool, timeout int, maxClientNum int) (s *Socks5Server) {
	s = new(Socks5Server)
	s.addr = addr
	s.udp = udp
	s.timeout = timeout
	s.maxClientNum = maxClientNum
	s.done = make(chan struct{})
	s.clientList = make([]*Client, 0, 10)
	s.u = nil
	return
}

func (s *Socks5Server) listen() (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", s.addr)
	s.listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return
	}
	logInfo.Printf("Socks5Server server listen on %s\n", s.listener.Addr())
	return
}

func (s *Socks5Server) serve() {
	maxChan := make(chan struct{}, s.maxClientNum)
	for {
		select {
		case <-s.done:
			return
		default:
			maxChan <- struct{}{}
			c, err := s.listener.AcceptTCP()
			if err != nil {
				logWarn.Println(err.Error())
				<-maxChan
				continue
			}
			logInfo.Printf("Accept connect from %s\n", c.RemoteAddr())

			client := newClient(s, newChannel(c, s.timeout))
			s.clientList = append(s.clientList, client)
			go func(id int) {
				defer func() {
					client.Close()
					s.clientList[id] = nil
				}()

				s.clientFSM(client)
				<-maxChan
			}(len(s.clientList) - 1)
		}
	}
}

func (s *Socks5Server) listenUDP() {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		logDebug.Println(err.Error())
		logWarn.Printf("Resolve UDP address %s failed\n", udpAddr)
		s.udp = false
		return
	}
	c, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		logDebug.Println(err.Error())
		logWarn.Printf("Listen UDP on %s failed\n", udpAddr)
		s.udp = false
		return
	}
	logInfo.Printf("UDP relay listen on %s\n", c.LocalAddr())

	s.u = newRelay(c, s.timeout)
	go s.u.setRelay()
}

func (s *Socks5Server) ListenAndServe() {
	if err := s.listen(); err != nil {
		logWarn.Println(err.Error())
		return
	}
	if s.udp {
		s.listenUDP()
	}
	s.serve()
}

func (s *Socks5Server) Close() {
	close(s.done)
	s.listener.Close()
	if s.u !=nil{
		s.u.Close()
	}
	for _, c := range s.clientList {
		if c != nil {
			c.Close()
		}
	}
	logInfo.Println("Socks5 server stop complete")
}

func (s *Socks5Server) clientFSM(client *Client) {
	for {
		switch client.state {
		case stateHello:
			client.hello()
		case stateAuth:
			client.auth()
		case stateRequest:
			client.request()
		case stateError:
			client.response()
		case stateTCPChannel:
			client.tcpChannel()
		case stateUDPRelay:
			client.udpRelay()
		case stateTerminal:
			return
		default:
			logDebug.Printf("Invalid client state %s", client.state)
			return
		}
	}
}
