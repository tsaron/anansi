package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/rs/zerolog"
)

// Timeout is a middleware that cancels ctx after a given timeout and return
// a 504 Gateway Timeout error to the client.
// P.S this was copied directly from go-chi, only removed writing to the response.
// Also note that this middleware can only be used once in the entire stack. Using
// it again has not effect on requests(i.e. the first use is the preferred).
//
// It's required that you select the ctx.Done() channel to check for the signal
// if the context has reached its deadline and return, otherwise the timeout
// signal will be just ignored.
//
// ie. a route/handler may look like:
//
//  r.Get("/long", func(w http.ResponseWriter, r *http.Request) {
// 	 ctx := r.Context()
// 	 processTime := time.Duration(rand.Intn(4)+1) * time.Second
//
// 	 select {
// 	 case <-ctx.Done():
// 	 	return
//
// 	 case <-time.After(processTime):
// 	 	 // The above channel simulates some hard work.
// 	 }
//
// 	 w.Write([]byte("done"))
//  })
func Timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer func() { cancel() }()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// AttachLogger attaches a new zerolog.Logger to each new HTTP request.
// Stolen from https://github.com/rs/zerolog/blob/master/hlog/hlog.go
func AttachLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a copy of the logger (including internal context slice)
			// to prevent data race when using UpdateContext.
			l := log.With().Logger()
			r = r.WithContext(l.WithContext(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

type Response interface {
	// Code returns the response code of the response
	Code() int
	// Body returns the body of the request as bytes
	Body() []byte
}

// Log updates a future log entry with the request parameters such as request ID and headers.
// Truncates logged request/response size to maxBodySize bytes. Set to less than zero to disable
// truncation, otherwise it defaults to 8kb
func Log(maxBodySize int) func(http.Handler) http.Handler {
	if maxBodySize == 0 {
		maxBodySize = 8 * 1024
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := zerolog.Ctx(r.Context())

			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				log.UpdateContext(func(ctx zerolog.Context) zerolog.Context {
					return ctx.Str("id", reqID)
				})
			}

			log.UpdateContext(func(ctx zerolog.Context) zerolog.Context {
				return ctx.
					Str("method", r.Method).
					Str("remote_address", r.RemoteAddr).
					Str("url", r.URL.String()).
					Interface("request_headers", toLower(r.Header))
			})

			requestBody, err := ReadBody(r)
			if err != nil {
				panic(err)
			}

			if len(requestBody) != 0 {
				log.UpdateContext(func(ctx zerolog.Context) zerolog.Context {
					buffer := logBody(log, maxBodySize, requestBody)
					return ctx.RawJSON("request", buffer.Bytes())
				})
			}

			defer func() {
				ww, ok := w.(Response)
				if !ok {
					return
				}
				log.UpdateContext(func(ctx zerolog.Context) zerolog.Context {
					buffer := logBody(log, maxBodySize, ww.Body())
					return ctx.RawJSON("response", buffer.Bytes())
				})
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func toLower(headers http.Header) map[string]interface{} {
	lowerCaseHeaders := make(map[string]interface{})

	for k, v := range headers {
		lowerKey := strings.ToLower(k)
		if len(v) == 0 {
			lowerCaseHeaders[lowerKey] = ""
		} else if len(v) == 1 {
			lowerCaseHeaders[lowerKey] = v[0]
		} else {
			lowerCaseHeaders[lowerKey] = v
		}
	}

	return lowerCaseHeaders
}

func logBody(log *zerolog.Logger, maxSize int, body []byte) *bytes.Buffer {
	buffer := new(bytes.Buffer)

	if err := json.Compact(buffer, body); err != nil {
		panic(err)
	}

	// only truncate large requests
	if maxSize > 0 && buffer.Len() > maxSize {
		buffer.Truncate(maxSize - 3) // leave space for the elipsis(3 bytes)
		buffer.WriteString("...")
	}

	return buffer
}
