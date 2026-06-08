package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"

	"aigateway/internal/store"
)

// responseCapturer wraps http.ResponseWriter to capture status and body.
type responseCapturer struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rc *responseCapturer) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapturer) Write(b []byte) (int, error) {
	if rc.status == 0 {
		rc.status = http.StatusOK
	}
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// Logging returns middleware that logs proxied requests and responses to the store.
func Logging(s *store.Store, routePrefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture request body
		var reqBody string
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("ERROR reading request body: %v", err)
			} else {
				reqBody = string(bodyBytes)
				// Replace body so the downstream handler can still read it
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// Wrap response writer to capture response
		capturer := &responseCapturer{
			ResponseWriter: w,
			status:         0,
		}

		// Call the next handler (the reverse proxy)
		next.ServeHTTP(capturer, r)

		// Calculate duration
		duration := time.Since(start).Milliseconds()

		// Build log entry
		logEntry := &store.CallLog{
			Route:          routePrefix,
			Method:         r.Method,
			RequestURL:     r.URL.String(),
			RequestBody:    truncate(reqBody, 65535),
			ResponseStatus: capturer.status,
			ResponseBody:   truncate(capturer.body.String(), 65535),
			DurationMs:     duration,
		}

		// Persist asynchronously so the client isn't waiting on DB I/O
		go func() {
			if err := s.InsertLog(logEntry); err != nil {
				log.Printf("ERROR inserting call log: %v", err)
			}
		}()
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
