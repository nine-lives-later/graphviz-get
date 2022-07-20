package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var debug bool
var httpFirstLinePattern = regexp.MustCompile(`^GET /(.+?)\?(.+) HTTP/1\..+`)

func newHttpResponse(status int, contentType string, body []byte) []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("HTTP/1.1 %v\r\n", status))
	buf.WriteString("Server: get-graphviz/1.0\r\n")

	buf.WriteString("Content-Type: ")
	buf.WriteString(contentType)
	buf.WriteString("\r\n")

	buf.WriteString("Content-Length: ")
	buf.WriteString(strconv.Itoa(len(body)))
	buf.WriteString("\r\n")

	buf.WriteString("Connection: close\r\n")
	buf.WriteString("Access-Control-Allow-Origin: *\r\n")
	buf.WriteString("Access-Control-Allow-Methods: GET\r\n")
	buf.WriteString("Access-Control-Allow-Headers: Content-Type\r\n")
	buf.WriteString("X-Content-Type-Options: nosniff\r\n")

	buf.WriteString("\r\n")
	buf.Write(body)

	return buf.Bytes()
}

var base64Pattern = regexp.MustCompile(`^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)?$`)

func handleRequest(conn net.Conn) {
	defer conn.Close()

	if debug {
		fmt.Printf("New connection from %v ...\n", conn.RemoteAddr())
	}

	scanner := bufio.NewScanner(conn)

	// read HTTP GET
	if !scanner.Scan() {
		fmt.Println("Failed to read first line from request")
		return
	}

	firstLine := scanner.Text()

	if debug {
		fmt.Printf("=======> %v\n", firstLine)
	}

	// ignore any web browser standard calls
	if strings.Contains(firstLine, "GET /favicon.ico") || strings.Contains(firstLine, "GET /robots.txt") {
		conn.Write(newHttpResponse(http.StatusNotFound, "text/plain", []byte("Error: file not found")))
		return
	}

	// parse the query string
	m := httpFirstLinePattern.FindStringSubmatch(firstLine)

	if len(m) <= 0 {
		fmt.Println("Error: Failed match pattern on full path:", firstLine)
		conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte("Error: Failed match pattern on full path")))
		return
	}

	// validate the format
	format := m[1]
	var contentType string

	switch format {
	case "svg":
		contentType = "image/svg+xml; charset=utf-8"
	case "png":
		contentType = "image/png"
	case "webp":
		contentType = "image/webp"
	case "pdf":
		contentType = "application/pdf"
	case "plain":
		contentType = "text/plain"
	default:
		fmt.Printf("Error: Unknown format specified: '%v'\n", format)
		conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Unknown format specified: '%v'", format))))
		return
	}

	// get the graph
	dotgraph := m[2]
	if dotgraph == "" {
		fmt.Println("Error: No query specified (the part after the questionmark)")
		conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte("Error: No query specified (the part after the questionmark)")))
		return
	}

	// decode, if encoded
	if strings.Contains(dotgraph, "%20") {
		var err error
		dotgraph, err = url.QueryUnescape(dotgraph)
		if err != nil {
			fmt.Println("Error: Failed to decode query:", err.Error())
			conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to decode query: %v", err))))
			return
		}
	}

	// decode base64, if used
	if base64Pattern.MatchString(dotgraph) {
		bin, err := base64.StdEncoding.DecodeString(dotgraph)
		if err != nil {
			fmt.Println("Error: Failed to decode base64:", err.Error())
			conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to decode base64: %v", err))))
			return
		}

		// check for gzip signature
		if len(bin) > 3 && bin[0] == 0x1f && bin[1] == 0x8b && bin[2] == 0x08 {
			gzr, err := gzip.NewReader(bytes.NewReader(bin))
			if err != nil {
				fmt.Println("Error: Failed to inflate gzip:", err.Error())
				conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to inflate gzip: %v", err))))
				return
			}

			bin, err = io.ReadAll(gzr)
			if err != nil {
				fmt.Println("Error: Failed to inflate gzip:", err.Error())
				conn.Write(newHttpResponse(http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to inflate gzip: %v", err))))
				return
			}
		}

		dotgraph = string(bin)
	}

	if debug {
		fmt.Printf("------>\n%v\n<-------\n", dotgraph)
	}

	// render the graph
	var outputBuf bytes.Buffer
	var errorBuf bytes.Buffer

	dot := exec.Command("dot", "-T"+format)
	dot.Stdin = bytes.NewBuffer([]byte(dotgraph))
	dot.Stdout = &outputBuf
	dot.Stderr = &errorBuf

	err := dot.Run()
	if err != nil {
		fmt.Println("Error:", err.Error())

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Error: %v", err))
		sb.WriteString("\n\n\n\n")
		sb.Write(errorBuf.Bytes())
		sb.WriteString("\n\n\n\n")
		sb.Write(outputBuf.Bytes())
		sb.WriteString("\n\n\n\n")
		sb.WriteString(dotgraph)
		sb.WriteString("\n\n\n\n")

		conn.Write(newHttpResponse(http.StatusInternalServerError, "text/plain", []byte(sb.String())))
		return
	}

	// write the reply
	buf := newHttpResponse(http.StatusOK, contentType, outputBuf.Bytes())

	if debug {
		fmt.Printf("----->>\n%v\n<<------\n\n", string(buf))
	}

	_, err = conn.Write(buf)
	if err != nil {
		fmt.Println("Error writing response: ", err.Error())
	}
}

func main() {
	debug = os.Getenv("DEBUG") == "1"

	// check if dot is actually working
	{
		var outputBuf bytes.Buffer

		dot := exec.Command("dot", "-V")
		dot.Stdout = &outputBuf
		dot.Stderr = &outputBuf

		err := dot.Run()
		if err != nil {
			fmt.Println("Error running 'dot -V':", err.Error())
			os.Exit(1)
		}

		fmt.Printf("Graphviz version: %v\n", strings.TrimSpace(outputBuf.String()))
	}

	// open the socket
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()

	fmt.Println("Listening on :8080 ...")
	if debug {
		fmt.Println("Debug mode is enabled")
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			continue
		}

		go handleRequest(conn)
	}
}
