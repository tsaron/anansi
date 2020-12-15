package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tsaron/anansi/responses"
)

// Success sends a JSend success message with status code 200. It logs the response
// if a zerolog.Logger is attached to the request.
func Success(r *http.Request, w http.ResponseWriter, v interface{}) {
	log := zerolog.Ctx(r.Context())
	raw := getJSON(log, v)

	responses.Write(w, http.StatusOK, raw)

	log.Info().
		Int("status", http.StatusOK).
		Int("length", len(raw)).
		Interface("response_headers", toLower(w.Header())).
		Msg("")
}

// Error sends a JSend error message. It logs the response if a zerolog.Logger is attached to the request.
func Error(r *http.Request, w http.ResponseWriter, err Err) {
	log := zerolog.Ctx(r.Context())
	raw := getJSON(log, err)

	responses.Write(w, err.Code, raw)

	log.Err(err).
		Int("status", err.Code).
		Int("length", len(raw)).
		Interface("response_headers", toLower(w.Header())).
		Msg("")
}

func getJSON(log *zerolog.Logger, v interface{}) []byte {
	raw, _ := json.Marshal(v)

	// log API responses
	if v != nil {
		log.UpdateContext(func(ctx zerolog.Context) zerolog.Context {
			buffer := new(bytes.Buffer)

			if err := json.Compact(buffer, raw); err != nil {
				panic(err)
			}

			return ctx.RawJSON("response", buffer.Bytes())
		})
	}

	return raw
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
