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

var PLAIN = map[string]string{"content-type": "text/plain"}

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
	return NewResponse(status, PLAIN, []byte(err.Error()))
}

func parseFirstLine(line string) (string, string, error) {
	a, isHttp1_1 := strings.CutSuffix(line, "HTTP/1.1")
	if !isHttp1_1 {
		return "", "", errors.New("Only HTTP/1.1 is supported")
	}
	b := strings.Split(a, " ")
	return b[0], b[1], nil
}

func parseHeaders(headersRaw string) map[string]string {
	headers := make(map[string]string)
	for _, str := range strings.Split(headersRaw, "\r\n") {
		k, v, ok := strings.Cut(str, ":")
		if ok {
			headers[strings.ToLower(strings.Trim(k, " "))] = strings.ToLower(strings.Trim(v, " "))
		}
	}
	return headers
}

func parseRequest(req []byte) (*HTTP1_1Request, error) {
	// split[0] = first line and headers ; split[1] = body
	split := strings.SplitN(string(req), "\r\n\r\n", 2)
	// split2[0] = first line ; split2[1] = headers
	split2 := strings.SplitN(split[0], "\r\n", 2)
	method, path, err := parseFirstLine(split2[0])
	if err != nil {
		return nil, err
	}
	headers := parseHeaders(split2[1])
	return &HTTP1_1Request{
		Method:  method,
		Path:    path,
		Headers: headers,
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
	} else if req.Path == "/user-agent" {
		conn.Write([]byte(NewResponse("200 OK", PLAIN, []byte(req.Headers["user-agent"]))))
	} else {
		str, ok := strings.CutPrefix(req.Path, "/echo/")
		if ok {
			conn.Write([]byte(NewResponse("200 OK", PLAIN, []byte(str))))
		} else {
			conn.Write([]byte(NewResponse("404 Not Found", nil, []byte{})))
		}
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
