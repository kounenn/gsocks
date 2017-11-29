package gsocks

import (
	"net"
	"time"
	"errors"
	"fmt"
	"sync"
)

type relay struct {
	*net.UDPConn
	timeout   int
	cacheMap  map[string]chan []byte
	cacheLock *sync.Mutex
}

func newRelay(c *net.UDPConn, timeout int) (r *relay) {
	r = new(relay)
	r.UDPConn = c
	r.timeout = timeout
	r.cacheMap = make(map[string]chan []byte)
	r.cacheLock = &sync.Mutex{}
	return
}

func (r *relay) setRelay() {
	for {
		var (
			dst string
			err error
		)
		buf := make([]byte, 65535)
		n, a, err := r.UDPConn.ReadFromUDP(buf)
		if err != nil {
			logDebug.Println(err.Error())
			logWarn.Printf("UDP listen on %s done\n", r.LocalAddr())
		}
		src := a.String()
		p := buf[:n]

		logDebug.Printf("Recv %d bytes from %s\n", n, src)

		p, dst, err = decodePacket(p)
		if err != nil {
			logDebug.Printf("Drop %d bytes from %s\n", n, src)
			continue
		}
		key := getCacheKey(src, dst)

		sendCh, ok := r.getCache(key)
		if !ok {
			sendCh = make(chan []byte, 10)
			r.setCache(key, sendCh)
			go func() {
				defer r.delCache(key)
				r.setSubConn(src, dst, sendCh)
				logDebug.Printf("UDP realy on %s and %s done\n", src, dst)
			}()
		}

		sendCh <- p
	}
}

func (r *relay) setSubConn(src, dst string, ch <-chan []byte) {
	done := make(chan struct{},2)

	udpSrc, err := net.ResolveUDPAddr("udp", src)
	if err != nil {
		logDebug.Println(err.Error())
		return
	}
	udpDst, err := net.ResolveUDPAddr("udp", dst)
	if err != nil {
		logDebug.Println(err.Error())
		return
	}
	conn, err := net.DialUDP("udp", nil, udpDst)
	if err != nil {
		logDebug.Println(err.Error())
		return
	}

	defer conn.Close()


	go func() {
		defer func() {
			done<- struct{}{}
		}()

		for {
			select {
			case p, ok := <-ch:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().
					Add(time.Duration(r.timeout) * time.Second))
				n, err := conn.Write(p)
				if err != nil {
					logDebug.Println(err.Error())
					return
				}
				logDebug.Printf("Send %d bytes to %s\n", n, conn.RemoteAddr())
			case <-time.After(time.Duration(r.timeout) * time.Second):
				return
			}
		}
	}()
	go func() {
		defer func() {
			done<- struct{}{}
		}()
		var (
			n   int
			err error
		)
		buf := make([]byte, 65535)
		for {
			conn.SetReadDeadline(time.Now().
				Add(time.Duration(r.timeout) * time.Second))
			n, err = conn.Read(buf)
			if err != nil {
				logDebug.Println(err.Error())
				return
			}
			logDebug.Printf("Recv %d bytes from %s\n", n, dst)
			p := buf[:n]
			p, err = encodePacket(p, dst)
			if err != nil {
				logDebug.Println(err.Error())
				return
			}
			n, err = r.WriteToUDP(p, udpSrc)
			if err != nil {
				logDebug.Println(err.Error())
				return
			}
			logDebug.Printf("Send %d bytes to %s\n", n, src)
		}
	}()

	<-done // wait read or write done
}

func (r *relay) setCache(k string, ch chan []byte) {
	defer r.cacheLock.Unlock()
	r.cacheLock.Lock()
	r.cacheMap[k] = ch
}

func (r *relay) getCache(k string) (ch chan []byte, ok bool) {
	defer r.cacheLock.Unlock()
	r.cacheLock.Lock()
	ch, ok = r.cacheMap[k]
	return
}

func (r *relay) delCache(k string) {
	defer r.cacheLock.Unlock()
	r.cacheLock.Lock()
	close(r.cacheMap[k])
	delete(r.cacheMap, k)
}

func decodePacket(buf []byte) (dBuf []byte, addr string, err error) {
	// FRAG
	if buf[2] != 0 {
		err = errors.New("FRAG is not 0")
		return
	}

	// ATYP
	t := buf[3]

	cur := 4
	l := 0
	switch t {
	case addrIp4:
		l = net.IPv4len
	case addrIp6:
		addr = net.IP(buf[cur: cur+net.IPv6len]).String()
		l = net.IPv6len
	case addrDomain:
		l = int(buf[cur])
		cur++
	default:
		err = fmt.Errorf("invalid atyp %v", buf[cur])
		return
	}

	// DST.ADDR
	// DST.PORT
	addr, err = decodeAddr(buf[cur:cur+l+2], t)
	cur += l + 2

	if err != nil {
		return
	}

	dBuf = buf[cur:]

	return
}

func encodePacket(buf []byte, addr string) (eBuf []byte, err error) {
	bAddr, t, err := encodeAddr(addr)
	if err != nil {
		return
	}

	eBuf = make([]byte, 4)

	// ATYP
	eBuf[3] = t
	eBuf = append(eBuf, bAddr...)
	eBuf = append(eBuf, buf...)
	return
}

func getCacheKey(src, dst string) (key string) {
	key = src + dst
	return
}
