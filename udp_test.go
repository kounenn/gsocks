package gsocks

import (
	"testing"
	"net"
	"sync"
	"bytes"
	"fmt"
)

func TestSocks5_udpRelay(t *testing.T) {
	relayAddr, _ := net.ResolveUDPAddr("udp", "localhost:12345")
	serverAddr, _ := net.ResolveUDPAddr("udp", "localhost:12346")

	c, _ := net.ListenUDP("udp", relayAddr)
	client, err := net.DialUDP("udp", nil, relayAddr)
	if err != nil {
		failedNow(t, err)
	}
	relay := newRelay(c, 60)
	udpServer, _ := net.ListenUDP("udp", serverAddr)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		relay.setRelay()
	}()

	msg := []byte("hello world")

	go func() {
		defer func() {
			udpServer.Close()
			wg.Done()
		}()
		buf := make([]byte, 65535)
		n, a, err := udpServer.ReadFromUDP(buf)
		if err != nil {
			failedNow(t, err)
		}
		buf = buf[:n]
		if !bytes.Equal(buf, msg) {
			failedNow(t, fmt.Sprintf("Server recv malformation msg %s,expcet %s", buf, msg))
		}
		if _, err := udpServer.WriteToUDP(buf, a); err != nil {
			failedNow(t, err)
		}
	}()

	go func() {
		defer func() {
			client.Close()
			wg.Done()
		}()
		buf, _ := encodePacket(msg, udpServer.LocalAddr().String())
		if _, err := client.Write(buf); err != nil {
			failedNow(t, err)
		}
		buf = make([]byte, 65535)
		n, err := client.Read(buf)
		if err != nil {
			failedNow(t, err)
		}
		buf, addr, _ := decodePacket(buf[:n])
		if addr != udpServer.LocalAddr().String() {
			failedNow(t, fmt.Sprintf("Client recv error addr %s,except %s", addr, udpServer.LocalAddr()))
		}
		if !bytes.Equal(buf, msg) {
			failedNow(t, fmt.Sprintf("Client recv malformation msg %s,expcet %s", buf, msg))
		}
	}()

	wg.Wait()
	succeed(t)

	return
}
