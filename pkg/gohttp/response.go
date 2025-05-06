package gohttp

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// Response response object
type Response struct {
	resp *http.Response
	req  *http.Request
	body []byte
	err  error
}

// ResponseBody response body
type ResponseBody []byte

// String fmt outout
func (r ResponseBody) String() string {
	return string(r)
}

// Read get slice of response body
func (r ResponseBody) Read(length int) []byte {
	if length > len(r) {
		length = len(r)
	}

	return r[:length]
}

// GetContents format response body as string
func (r ResponseBody) GetContents() string {
	return string(r)
}

// GetRequest get request object
func (r *Response) GetRequest() *http.Request {
	return r.req
}

// GetBody parse response body
func (r *Response) GetBody() (ResponseBody, error) {
	return ResponseBody(r.body), r.err
}

// GetParsedBody parse response body
func (r *Response) GetJsonDecodeBody(body interface{}) (err error) {
	return json.Unmarshal(r.body, body)
}

// GetStatusCode get response status code
func (r *Response) GetStatusCode() int {
	return r.resp.StatusCode
}

// GetReasonPhrase get response reason phrase
func (r *Response) GetReasonPhrase() string {
	status := r.resp.Status
	arr := strings.Split(status, " ")

	return arr[1]
}

// IsTimeout get if request is timeout
func (r *Response) IsTimeout() bool {
	if r.err == nil {
		return false
	}
	netErr, ok := r.err.(net.Error)
	if !ok {
		return false
	}
	if netErr.Timeout() {
		return true
	}

	return false
}

// GetHeaders get response headers
func (r *Response) GetHeaders() map[string][]string {
	return r.resp.Header
}

// GetHeader get response header
func (r *Response) GetHeader(name string) []string {
	headers := r.GetHeaders()
	for k, v := range headers {
		if strings.ToLower(name) == strings.ToLower(k) {
			return v
		}
	}

	return nil
}

// GetHeaderLine get a single response header
func (r *Response) GetHeaderLine(name string) string {
	header := r.GetHeader(name)
	if len(header) > 0 {
		return header[0]
	}

	return ""
}

// HasHeader get if header exsits in response headers
func (r *Response) HasHeader(name string) bool {
	headers := r.GetHeaders()
	for k := range headers {
		if strings.ToLower(name) == strings.ToLower(k) {
			return true
		}
	}

	return false
}

// Cookies get if header exsits in response headers
func (r *Response) GetCookies() []*http.Cookie {
	return r.resp.Cookies()
}
