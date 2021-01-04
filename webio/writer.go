package webio

// The original work was derived from go-chi's middleware, source:
// https://github.com/go-chi/chi/tree/master/middleware/wrap_writer.go

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

const defaultResponseTime = "X-Response-Time"

// CustomWriter is a proxy around an http.ResponseWriter that allows you to
// hook into various parts of the response process.
// It also adds response time header directly since it can track the response
// time no matter the response is generated.
type CustomWriter interface {
	http.ResponseWriter
	// Code returns the status code of the response
	Code() int
	// Duration is the time taken from when the writer is created to the first
	// call to WriteHeader
	Duration() time.Duration
	// TeeWriter sets the writer the user will use to receive the response body
	// while calls to Write are being made. Do note that it should not be
	// accepting
	TeeWriter(tee io.Writer)
}

type custom struct {
	w           http.ResponseWriter
	start       time.Time
	code        int
	duration    time.Duration
	wroteHeader bool
	headerName  string
	teeWriter   io.Writer
}

func newWriter(w http.ResponseWriter, protoMajor int, headerName string) http.ResponseWriter {
	_, fl := w.(http.Flusher)

	if headerName == "" {
		headerName = defaultResponseTime
	}

	tw := custom{
		w:          w,
		start:      time.Now(),
		headerName: headerName,
	}

	if protoMajor == 2 {
		_, ps := w.(http.Pusher)
		if fl && ps {
			return &http2Writer{tw}
		}
	} else {
		_, hj := w.(http.Hijacker)
		_, rf := w.(io.ReaderFrom)
		if fl && hj && rf {
			return &httpWriter{tw}
		}
	}
	if fl {
		return &flushWriter{tw}
	}

	return &tw
}

func (t *custom) WriteHeader(code int) {
	if !t.wroteHeader {
		t.wroteHeader = true
		t.duration = time.Since(t.start)
		t.code = code

		// write the response time header
		dur := int(t.duration.Milliseconds())
		t.Header().Add(t.headerName, strconv.Itoa(dur)+"ms")

		t.w.WriteHeader(code)
	}
}

func (t *custom) Write(buf []byte) (int, error) {
	if !t.wroteHeader {
		t.WriteHeader(http.StatusOK)
	}

	n, err := t.w.Write(buf)
	if err != nil {
		return n, err
	}

	if t.teeWriter == nil {
		return n, err
	}

	_, err = t.teeWriter.Write(buf[:n])

	return n, err
}

func (t *custom) Header() http.Header {
	return t.w.Header()
}

func (t *custom) Duration() time.Duration {
	return t.duration
}

func (t *custom) Code() int {
	return t.code
}

func (t *custom) TeeWriter(w io.Writer) {
	t.teeWriter = w
}

type flushWriter struct {
	custom
}

func (f *flushWriter) Flush() {
	f.wroteHeader = true
	fl := f.custom.w.(http.Flusher)
	fl.Flush()
}

type httpWriter struct {
	custom
}

func (h1 *httpWriter) Flush() {
	h1.wroteHeader = true
	fl := h1.custom.w.(http.Flusher)
	fl.Flush()
}

func (h1 *httpWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := h1.custom.w.(http.Hijacker)
	return hj.Hijack()
}

func (h1 *httpWriter) ReadFrom(r io.Reader) (int64, error) {
	rf := h1.custom.w.(io.ReaderFrom)
	if !h1.wroteHeader {
		h1.WriteHeader(http.StatusOK)
	}
	return rf.ReadFrom(r)
}

type http2Writer struct {
	custom
}

func (h2 *http2Writer) Push(target string, opts *http.PushOptions) error {
	return h2.custom.w.(http.Pusher).Push(target, opts)
}

func (h2 *http2Writer) Flush() {
	h2.wroteHeader = true
	fl := h2.custom.w.(http.Flusher)
	fl.Flush()
}

// static tests
var _ http.Flusher = &httpWriter{}
var _ http.Flusher = &http2Writer{}

var _ http.Pusher = &http2Writer{}
var _ http.Hijacker = &httpWriter{}
var _ io.ReaderFrom = &httpWriter{}
