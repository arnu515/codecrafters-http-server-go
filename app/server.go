package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"strings"
)

var directory string

func init() {
	flag.StringVar(&directory, "directory", "", "Directory where files are located")
	flag.Parse()
}

type Req struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

type Res struct {
	Status  uint
	CType   string
	Headers map[string]string
	Body    []byte
}

func (r *Res) StatusText() string {
	switch r.Status {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 422:
		return "Unprocessable Entity"
	case 500:
		return "Internal Server Error"
	default:
		return ""
	}
}

func (r *Res) String(enc bool) string {
	headersStr := ""
	if r.Headers != nil {
		// remove content-{encoding,length,type} from headers
		delete(r.Headers, "content-encoding")
		delete(r.Headers, "content-length")
		delete(r.Headers, "content-type")

		for k, v := range r.Headers {
			headersStr += fmt.Sprintf("%s: %s\r\n", strings.ToLower(k), v)
		}
	}
	headersStr += fmt.Sprintf("content-length: %d\r\n", len(r.Body))
	if r.CType != "" {
		headersStr += fmt.Sprintf("content-type: %s\r\n", r.CType)
	}
	if enc {
		headersStr += "content-encoding: gzip\r\n"
	}
	return fmt.Sprintf("HTTP/1.1 %d %s\r\n%s\r\n%s", r.Status, r.StatusText(), headersStr, string(r.Body))
}

func ErrRes(err error, status uint) *Res {
	return &Res{
		Status: status,
		CType:  "text/plain",
		Body:   []byte(err.Error()),
	}
}

func h(contentType string, enc bool) map[string]string {
	headers := map[string]string{
		"content-type": contentType,
	}
	if enc {
		headers["content-encoding"] = "gzip"
	}
	return headers
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

func parseRequest(req []byte) (*Req, error) {
	// split[0] = first line and headers ; split[1] = body
	split := strings.SplitN(string(req), "\r\n\r\n", 2)
	// split2[0] = first line ; split2[1] = headers
	split2 := strings.SplitN(split[0], "\r\n", 2)
	method, path, err := parseFirstLine(split2[0])
	if err != nil {
		return nil, err
	}
	headers := parseHeaders(split2[1])
	return &Req{
		Method:  method,
		Path:    path,
		Headers: headers,
		Body:    []byte(split[1]),
	}, nil
}

func handleSendFile(p string) *Res {
	stat, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Res{Status: 404}
		} else {
			return ErrRes(err, 500)
		}
	}
	if stat.IsDir() {
		return &Res{Status: 404}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ErrRes(err, 500)
	}
	return &Res{
		Status: 200,
		CType:  "application/octet-stream",
		Body:   []byte(data),
	}
}

func handleCreateFile(p string, content []byte) *Res {
	err := os.WriteFile(p, content, 0600)
	if err != nil {
		return ErrRes(err, 500)
	} else {
		return &Res{Status: 201}
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("Received TCP Connection from %s\n", conn.RemoteAddr())

	b := make([]byte, 1024)
	_, err := conn.Read(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read from TCP connection %s: %s\n", conn.RemoteAddr().String(), err)
		return
	}

	for i, v := range b {
		if v == 0 {
			b = b[:i]
			break
		}
	}

	req, err := parseRequest(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse HTTP request from TCP connection %s: %s\n", conn.RemoteAddr().String(), err)
		conn.Write([]byte((ErrRes(err, 422)).String(false)))
	}
	enc := strings.Contains(req.Headers["accept-encoding"], "gzip")

	if req.Path == "/" {
		conn.Write([]byte((&Res{Status: 200}).String(enc)))
	} else if req.Path == "/user-agent" {
		conn.Write([]byte((&Res{
			Status: 200,
			CType:  "text/plain",
			Body:   []byte(req.Headers["user-agent"]),
		}).String(enc)))
	} else {
		echoStr, echoOk := strings.CutPrefix(req.Path, "/echo/")
		filesStr, filesOk := strings.CutPrefix(req.Path, "/files/")
		if echoOk {
			conn.Write([]byte((&Res{
				Status: 200,
				CType:  "text/plain",
				Body:   []byte(echoStr),
			}).String(enc)))
		} else if filesOk && directory[0] == '/' {
			p := path.Join(directory, filesStr)
			if req.Method == "GET" {
				conn.Write([]byte(handleSendFile(p).String(enc)))
			} else if req.Method == "POST" {
				conn.Write([]byte(handleCreateFile(p, req.Body).String(enc)))
			} else {
				conn.Write([]byte((&Res{Status: 405}).String(enc)))
			}
		} else {
			conn.Write([]byte((&Res{Status: 404}).String(enc)))
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
