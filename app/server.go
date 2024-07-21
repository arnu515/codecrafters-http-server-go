package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

type HTTP1_1Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

func NewResponse(status string, headers map[string]string, body []byte) string {
	headersStr := ""
	if headers != nil {
		for k, v := range headers {
			headersStr += fmt.Sprintf("%s: %s\r\n", strings.ToLower(k), v)
		}
	}
	headersStr += fmt.Sprintf("content-length: %d\r\n", len(body))
	return fmt.Sprintf("HTTP/1.1 %s\r\n%s\r\n%s", status, headersStr, string(body))
}

func ErrToRes(err error, status string) string {
	return NewResponse(status, map[string]string{"Content-Type": "text/plain"}, []byte(err.Error()))
}

func parseFirstLine(line string) (string, string, error) {
	a, isHttp1_1 := strings.CutSuffix(line, "HTTP/1.1")
	if !isHttp1_1 {
		return "", "", errors.New("Only HTTP/1.1 is supported")
	}
	b := strings.Split(a, " ")
	return b[0], b[1], nil
}

func parseRequest(req []byte) (*HTTP1_1Request, error) {
	reqLen := len(req)
	line1 := ""
	for i := range reqLen {
		if req[i] == '\r' && i+1 <= len(req) && req[i+1] == '\n' {
			break
		}
		line1 += string(req[i])
	}
	method, path, err := parseFirstLine(line1)
	if err != nil {
		return nil, err
	}
	return &HTTP1_1Request{
		Method: method,
		Path:   path,
	}, nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	b := make([]byte, 1024)
	_, err := conn.Read(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read from TCP connection %s: %s\n", conn.RemoteAddr().String(), err)
		return
	}

	req, err := parseRequest(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse HTTP request from TCP connection %s: %s\n", conn.RemoteAddr().String(), err)
		conn.Write([]byte(ErrToRes(err, "422 Unprocessable Entity")))
		return
	}

	if req.Path == "/" {
		conn.Write([]byte(NewResponse("200 OK", nil, []byte{})))
	} else {
		conn.Write([]byte(NewResponse("404 Not Found", nil, []byte{})))
	}
}

func main() {
	server, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	fmt.Println("Listening on port 4221")

	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not accept TCP connection: "+err.Error())
			continue
		}

		go handleConnection(conn)
	}
}
