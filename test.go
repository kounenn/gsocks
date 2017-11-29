package gsocks

import (
	"testing"
	"math/rand"
)

func failedNow(t *testing.T, i interface{}) {
	var errStr string
	switch t := i.(type) {
	case error:
		errStr = t.Error()
	case string:
		errStr = t
	default:
		return
	}
	t.Log(errStr)
	t.FailNow()
}

func failed(t *testing.T, i interface{}) {
	var errStr string
	switch t := i.(type) {
	case error:
		errStr = t.Error()
	case string:
		errStr = t
	}
	t.Log(errStr)
	t.Failed()
}

func succeed(t *testing.T) {
	if !t.Failed() {
		t.Logf("%s pass\n", t.Name())
	}
}

func randByte(buf []byte, min int) (n int) {
	n = rand.Intn(len(buf)-min+1) + min
	if _, err := rand.Read(buf); err != nil {
		panic(err.Error())
	}
	return
}
