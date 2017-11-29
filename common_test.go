package gsocks

import (
	"testing"
	"bytes"
)

func TestSocks5_AddrCODEC(t *testing.T) {
	testMap := make(map[string]uint8)
	testMap["127.0.0.1:123"] = addrIp4
	testMap["[2001:db8::68]:123"] = addrIp6
	testMap["www.baidu.com:433"] = addrDomain
	for addr, atyp := range testMap {
		p, at, _ := encodeAddr(addr)
		if atyp != at {
			t.Errorf("type of dstList %s encode failed, except: %d get: %d", addr, atyp, at)
		}
		a, e := decodeAddr(p, at)
		if at == addrDomain {
			a = a[1:]
		}
		if e != nil {
			t.Error(e.Error())
			t.Errorf("Baddr %v decode failed\n", p)
		}
		if addr != a {
			t.Errorf("codec failed, except: %v, get: %v", addr, a)
		}
	}

	if !t.Failed() {
		t.Logf("%s pass\n", t.Name())
	}
}

func TestSocks5_CODEC(t *testing.T) {
	addr := "127.0.0.1:65534"
	buf := make([]byte, 24)
	n := randByte(buf, 1)
	buf = buf[:n]
	buf2, err := encodePacket(buf, addr)
	if err != nil {
		t.Fatalf(err.Error())
	}
	buf2, addr2, err := decodePacket(buf2)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if !bytes.Equal(buf, buf2) {
		t.Fatalf("buffer %v CODEC failed\nget %v\n", buf, buf2)
	}
	if addr != addr2 {
		t.Fatalf("address CODEC failed\n"+
			"excpet:%v\nget:%v\n", addr, addr2)
	}

	t.Logf("%s pass\n", t.Name())
}
