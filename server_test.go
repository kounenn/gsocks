package gsocks

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"golang.org/x/net/proxy"
	"log"
)

func tcpConnPair(pAddr string) (serverConn, clientConn net.Conn) {
	listener, _ := net.Listen("tcp", ":0")
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		pClient, err := proxy.SOCKS5("tcp", pAddr, nil, &net.Dialer{})
		if err != nil {
			log.Fatal(err.Error())
		}
		clientConn, err = pClient.Dial("tcp", listener.Addr().String())
		if err != nil {
			log.Fatal(err.Error())
		}
	}()
	serverConn, _ = listener.Accept()
	<-done
	return
}

func checkIO(w io.Writer, r io.Reader, t *testing.T) {
	wBuf := make([]byte, 65535)
	rBuf := make([]byte, 65535)
	for i := 0; i < 10; i++ {
		n := randByte(wBuf, 1)
		w.Write(wBuf[:n])
		io.ReadFull(r, rBuf[:n])
		if !bytes.Equal(wBuf[:n], rBuf[:n]) {
			t.Logf("wBuf %v\n", wBuf[:n])
			t.Logf("rBuf %v\n", rBuf[:n])
			t.Errorf("%s fail", t.Name())
			return
		}
	}
}

func TestSocks5_ListenAndServe(t *testing.T) {
	pServer := New(":0", true, 60, 60)

	if e := pServer.listen(); e != nil {
		t.Error(e.Error())
	}

	wg1 := sync.WaitGroup{}
	wg2 := sync.WaitGroup{}

	wg1.Add(1)
	wg2.Add(1)

	go func() {
		pServer.serve()
		wg1.Done()
	}()

	serverConn, clientConn := tcpConnPair(pServer.listener.Addr().String())
	defer serverConn.Close()

	go func() {
		defer clientConn.Close()
		checkIO(clientConn, clientConn, t)
		wg2.Done()
	}()

	_, err := io.Copy(serverConn, serverConn)
	if err != nil && err != io.EOF {
		t.Log(err.Error())
		t.Errorf("%s fail", t.Name())
	}

	wg2.Wait()
	pServer.Close()
	wg1.Wait()

	if !t.Failed() {
		t.Logf("%s pass\n", t.Name())
	}
}
