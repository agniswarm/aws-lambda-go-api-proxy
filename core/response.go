// Package core provides utility methods that help convert proxy events
// into an http.Request and http.ResponseWriter
package core

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
)

const (
	defaultStatusCode    = -1
	contentTypeHeaderKey = "Content-Type"
)

// ProxyResponseWriter implements http.ResponseWriter and adds the method
// necessary to return an events.APIGatewayProxyResponse object
type ProxyResponseWriter struct {
	headers   http.Header
	body      bytes.Buffer
	status    int
	observers []chan<- bool
}

// NewProxyResponseWriter returns a new ProxyResponseWriter object.
// The object is initialized with an empty map of headers and a
// status code of -1
func NewProxyResponseWriter() *ProxyResponseWriter {
	return &ProxyResponseWriter{
		headers:   make(http.Header),
		status:    defaultStatusCode,
		observers: make([]chan<- bool, 0),
	}

}

func (r *ProxyResponseWriter) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)

	r.observers = append(r.observers, ch)

	return ch
}

func (r *ProxyResponseWriter) notifyClosed() {
	for _, v := range r.observers {
		v <- true
	}
}

// Header implementation from the http.ResponseWriter interface.
func (r *ProxyResponseWriter) Header() http.Header {
	return r.headers
}

// Write sets the response body in the object. If no status code
// was set before with the WriteHeader method it sets the status
// for the response to 200 OK.
func (r *ProxyResponseWriter) Write(body []byte) (int, error) {
	if r.status == defaultStatusCode {
		r.status = http.StatusOK
	}

	// if the content type header is not set when we write the body we try to
	// detect one and set it by default. If the content type cannot be detected
	// it is automatically set to "application/octet-stream" by the
	// DetectContentType method
	if r.Header().Get(contentTypeHeaderKey) == "" {
		r.Header().Add(contentTypeHeaderKey, http.DetectContentType(body))
	}

	return (&r.body).Write(body)
}

// WriteHeader sets a status code for the response. This method is used
// for error responses.
func (r *ProxyResponseWriter) WriteHeader(status int) {
	r.status = status
}

// GetProxyResponse converts the data passed to the response writer into
// an events.APIGatewayProxyResponse object.
// Returns a populated proxy response object. If the response is invalid, for example
// has no headers or an invalid status code returns an error.
func (r *ProxyResponseWriter) GetProxyResponse() (events.APIGatewayProxyResponse, error) {
	r.notifyClosed()

	if r.status == defaultStatusCode {
		return events.APIGatewayProxyResponse{}, errors.New("Status code not set on response")
	}

	var output string
	isBase64 := false

	bb := (&r.body).Bytes()

	if utf8.Valid(bb) {
		output = string(bb)
	} else {
		output = base64.StdEncoding.EncodeToString(bb)
		isBase64 = true
	}

	return events.APIGatewayProxyResponse{
		StatusCode:        r.status,
		MultiValueHeaders: http.Header(r.headers),
		Body:              output,
		IsBase64Encoded:   false,
	}, nil
}
