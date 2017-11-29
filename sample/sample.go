package main

import (
	"gsocks"
)

func main() {
	server := gsocks.New("127.0.0.1:1080", true, 60, 60)
	server.ListenAndServe()
}