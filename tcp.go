package gsocks

import (
	"net"
	"io"
	"time"
)

type channel struct {
	*net.TCPConn
	timeout int
}

func newChannel(c *net.TCPConn, timeout int) (t *channel) {
	t = new(channel)
	t.TCPConn = c
	t.timeout = timeout
	return
}

func (conn *channel) Read(p []byte) (n int, err error) {
	conn.TCPConn.SetReadDeadline(time.Now().
		Add(time.Duration(conn.timeout) * time.Second))
	n, err = conn.TCPConn.Read(p)
	return
}

func (conn *channel) Write(p []byte) (n int, err error) {
	conn.TCPConn.SetWriteDeadline(time.Now().
		Add(time.Duration(conn.timeout) * time.Second))
	n, err = conn.TCPConn.Write(p)
	return
}

func setChannel(src, dst *channel, ch chan<- error) {
	defer dst.CloseWrite()
	n, err := io.Copy(dst, src)
	if err != nil {
		ch <- err
		return
	}
	logDebug.Printf("%s send %d bytes to %s\n", src.RemoteAddr(), n, dst.RemoteAddr())
	ch <- nil
}
