#  G-socks

Implement [socks5](https://www.ietf.org/rfc/rfc1928.txt) protocol server side



### feature

* TCP connect(0x01)  command
* UDP association(0x03) command
* Max number of clients

### example

```go
package main

import (
	"gsocks"
)

func main() {
	server := gsocks.New("127.0.0.1:1080", true, 60, 60) //server_addr, udp，timeout, client_n
	server.ListenAndServe()
}
```





