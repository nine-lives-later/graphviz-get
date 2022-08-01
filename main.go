package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	debug           bool
	graphvizVersion string
)

func writeHttpResponse(res *fasthttp.Response, status int, contentType string, body []byte) {
	res.Header.Set("X-Graphviz-Version", graphvizVersion)

	res.Header.Set("Content-Type", contentType)
	res.Header.Set("Content-Length", strconv.Itoa(len(body)))

	res.Header.Set("X-Content-Type-Options", "nosniff")

	// add cors headers
	res.Header.Set("Access-Control-Allow-Origin", "*")
	res.Header.Set("Access-Control-Allow-Methods", "GET")
	res.Header.Set("Access-Control-Allow-Headers", "Content-Type")

	// write the body
	res.SetBodyRaw(body)
	res.SetStatusCode(status)
}

var base64Pattern = regexp.MustCompile(`^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)?$`)

func handleRequest(ctx *fasthttp.RequestCtx) {
	log := log.WithFields(logrus.Fields{
		"clientIP": ctx.RemoteAddr().String(),
		"path":     string(ctx.Path()),
		"method":   string(ctx.Method()),
		"id":       ctx.ID(),
	})

	if debug {
		log.Debug("New request...")
	}

	// do some basic routing
	queryPath := string(ctx.Path())
	format := queryPath[1:] // strip first slash

	var contentType string

	switch queryPath {
	case "/favicon.ico":
		fallthrough
	case "/robots.txt":
		writeHttpResponse(&ctx.Response, http.StatusNotFound, "text/plain", []byte("Error: file not found"))
		return
	case "/":
		writeHttpResponse(&ctx.Response, http.StatusOK, "text/plain", []byte("Graphviz GET - https://github.com/nine-lives-later/graphviz-get"))
		return
	case "/svg":
		contentType = "image/svg+xml; charset=utf-8"
	case "/png":
		contentType = "image/png"
	case "/webp":
		contentType = "image/webp"
	case "/pdf":
		contentType = "application/pdf"
	case "/plain":
		contentType = "text/plain"
	default:
		log.Errorf("Unknown format specified: '%v'", queryPath)
		writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Unknown format specified: '%v'", format)))
		return
	}

	// parse the query string
	dotgraph := ctx.QueryArgs().String()
	if dotgraph == "" {
		log.Errorf("No query specified (the part after the questionmark)")
		writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte("Error: No query specified (the part after the questionmark)"))
		return
	}

	dotgraph = strings.Replace(dotgraph, "=%3D", "==", 1) // fix fasthttp forcing url encoding on base64 '==' ending
	dotgraph = strings.ReplaceAll(dotgraph, "%2F", "/")   // fix fasthttp forcing url encoding on '/'

	// decode, if encoded
	if strings.Contains(dotgraph, "%20") {
		var err error
		dotgraph, err = url.QueryUnescape(dotgraph)
		if err != nil {
			log.Errorf("Failed to decode query: %v", err)
			writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to decode query: %v", err)))
			return
		}
	}

	// decode base64, if used
	if base64Pattern.MatchString(dotgraph) {
		bin, err := base64.StdEncoding.DecodeString(dotgraph)
		if err != nil {
			log.Errorf("Failed to decode base64: %v", err)
			writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to decode base64: %v", err)))
			return
		}

		// check for gzip signature
		if len(bin) > 3 && bin[0] == 0x1f && bin[1] == 0x8b && bin[2] == 0x08 {
			gzr, err := gzip.NewReader(bytes.NewReader(bin))
			if err != nil {
				log.Errorf("Failed to inflate gzip: %v", err)
				writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to inflate gzip: %v", err)))
				return
			}

			bin, err = io.ReadAll(gzr)
			if err != nil {
				log.Errorf("Failed to inflate gzip: %v", err)
				writeHttpResponse(&ctx.Response, http.StatusBadRequest, "text/plain", []byte(fmt.Sprintf("Error: Failed to inflate gzip: %v", err)))
				return
			}
		}

		dotgraph = string(bin)
	}

	if debug {
		log.Debugf("------>\n%v\n<-------", dotgraph)
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
		log.Errorf("Running dot failed: %v", err)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Error: %v", err))
		sb.WriteString("\n\n\n\n")
		sb.Write(errorBuf.Bytes())
		sb.WriteString("\n\n\n\n")
		sb.Write(outputBuf.Bytes())
		sb.WriteString("\n\n\n\n")
		sb.WriteString(dotgraph)
		sb.WriteString("\n\n\n\n")

		writeHttpResponse(&ctx.Response, http.StatusInternalServerError, "text/plain", []byte(sb.String()))
		return
	}

	// write the reply
	writeHttpResponse(&ctx.Response, http.StatusOK, contentType, outputBuf.Bytes())
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
			log.Fatalf("Error running 'dot -V': %v", err)
			os.Exit(1)
		}

		graphvizVersion = strings.TrimSpace(outputBuf.String())

		log.Infof("Graphviz version: %v", graphvizVersion)
	}

	// setup the http server
	handler := handleRequest
	handler = fasthttp.CompressHandler(handler) // enable gzip compression

	// open the socket
	log.Info("Listening on :8080 ...")
	if debug {
		log.StandardLogger().SetLevel(log.DebugLevel)

		log.Debug("Debug mode is enabled")
	}

	server := fasthttp.Server{
		Logger:             log.WithField("service", "fasthttp"),
		GetOnly:            true,
		MaxRequestBodySize: 1024,                               // no request body is used in this scenario
		ReadBufferSize:     fasthttp.DefaultMaxRequestBodySize, // all information is in the header
		WriteBufferSize:    fasthttp.DefaultMaxRequestBodySize,
		Handler:            handler,
	}
	err := server.ListenAndServe("0.0.0.0:8080")
	if err != nil {
		log.Fatalf("Error listening: %v", err)
		os.Exit(1)
	}
}
