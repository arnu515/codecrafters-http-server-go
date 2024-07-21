package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	server, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not accept TCP connection: "+err.Error())
			continue
		}

		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		conn.Close()
	}
}
