package gohttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Request object
type Request struct {
	ctx  context.Context
	opts Options
	cli  *http.Client
	req  *http.Request
	body io.Reader
}

// Get send get request
func (r *Request) Get(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("GET", uri, opts...)
	return req.do()
}

// Post send post request
func (r *Request) Post(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("POST", uri, opts...)
	return req.do()
}

// Put send put request
func (r *Request) Put(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("PUT", uri, opts...)
	return req.do()
}

// Patch send patch request
func (r *Request) Patch(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("PATCH", uri, opts...)
	return req.do()
}

// Delete send delete request
func (r *Request) Delete(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("DELETE", uri, opts...)
	return req.do()
}

// Options send options request
func (r *Request) Options(uri string, opts ...Options) (*Response, error) {
	req, _ := r.Request("OPTIONS", uri, opts...)
	return req.do()
}

// Request send request
func (r *Request) Request(method, uri string, opts ...Options) (*Request, error) {
	if len(opts) > 0 {
		r.opts = opts[0]
	}
	if r.opts.Headers == nil {
		r.opts.Headers = make(map[string]interface{})
	}

	switch method {
	case http.MethodGet, http.MethodDelete:
		req, err := http.NewRequest(method, uri, nil)
		if err != nil {
			return nil, err
		}
		req.WithContext(r.ctx)
		r.req = req
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodOptions:
		// parse body
		r.parseBody()

		req, err := http.NewRequest(method, uri, r.body)
		if err != nil {
			return nil, err
		}
		req.WithContext(r.ctx)
		r.req = req
	default:
		return nil, errors.New("invalid request method")
	}

	// parseOptions
	r.parseOptions()

	// parseClient
	r.parseClient()

	// parse query
	r.parseQuery()

	// parse headers
	r.parseHeaders()

	// parse cookies
	r.parseCookieFile()
	r.parseCookies()

	if r.opts.Debug {
		// print request object
		dump, err := httputil.DumpRequest(r.req, true)
		if err == nil {
			log.Printf("\n%s\n\n", dump)
		}
	}
	return r, nil
}

func (r *Request) do() (*Response, error) {
	var _resp = new(http.Response)
	var err error
	for i := 0; i < r.opts.Retry; i++ {
		_resp, err = r.cli.Do(r.req)
		if err == nil && _resp != nil {
			break
		}
	}
	if _resp == nil || _resp.Body == nil {
		return nil, err
	}
	defer _resp.Body.Close()
	resp := &Response{
		resp: _resp,
		req:  r.req,
		err:  err,
	}
	if err != nil || _resp.StatusCode != http.StatusOK {
		if r.opts.Debug {
			// print response err
			fmt.Println(err)
		}
		return resp, err
	}

	if r.opts.DestFile != "" {
		dl := &Download{
			startedAt: time.Now(),
			ctx:       context.Background(),
			mutex:     new(sync.RWMutex),
			info: &Info{
				Size:      uint64(_resp.ContentLength),
				Rangeable: false,
			},
			Dest: r.opts.DestFile,
		}
		// Wait group.
		var wg sync.WaitGroup
		wg.Add(1)
		go dlProgressBar(&wg, dl)
		_, err := resp.dlFile(dl)
		wg.Wait()
		resp.err = err
	} else {
		body, err := io.ReadAll(_resp.Body)
		resp.body = body
		resp.err = err
	}

	if r.opts.Debug {
		// print response data
		body, _ := resp.GetBody()
		fmt.Println(string(body))
	}
	return resp, nil
}

func (r *Request) parseOptions() {
	r.opts.timeout = time.Duration(r.opts.Timeout*1000) * time.Millisecond

	if r.opts.Retry == 0 {
		r.opts.Retry = 3
	}
}

func (r *Request) parseClient() {
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}

	if r.opts.Proxy != "" {
		proxy, err := url.Parse(r.opts.Proxy)
		if err == nil {
			tr.Proxy = http.ProxyURL(proxy)
		}
	}
	r.cli = &http.Client{
		Timeout:   r.opts.timeout,
		Transport: tr,
	}
	if r.opts.CookieJar != nil {
		r.cli.Jar = r.opts.CookieJar
	}
}

func (r *Request) parseQuery() {
	switch r.opts.Query.(type) {
	case string:
		str := r.opts.Query.(string)
		r.req.URL.RawQuery = str
	case map[string]string:
		q := r.req.URL.Query()
		for k, v := range r.opts.Query.(map[string]string) {
			q.Set(k, v)
		}
		r.req.URL.RawQuery = q.Encode()
	case map[string]interface{}:
		q := r.req.URL.Query()
		for k, v := range r.opts.Query.(map[string]interface{}) {
			if vv, ok := v.(string); ok {
				q.Set(k, vv)
				continue
			}
			if vv, ok := v.([]string); ok {
				for _, vvv := range vv {
					q.Add(k, vvv)
				}
			}
		}
		r.req.URL.RawQuery = q.Encode()
	}
}

func (r *Request) parseCookies() {
	switch r.opts.Cookies.(type) {
	case string:
		cookies := r.opts.Cookies.(string)
		r.req.Header.Add("Cookie", cookies)
	case map[string]string:
		cookies := r.opts.Cookies.(map[string]string)
		for k, v := range cookies {
			r.req.AddCookie(&http.Cookie{
				Name:  k,
				Value: v,
			})
		}
	case []*http.Cookie:
		cookies := r.opts.Cookies.([]*http.Cookie)
		for _, cookie := range cookies {
			r.req.AddCookie(cookie)
		}
	}
}

func (r *Request) parseCookieFile() {
	if r.opts.CookieFile == "" {
		return
	}
	fp, err := os.Open(r.opts.CookieFile)
	if err != nil {
		return
	}
	defer fp.Close()

	bsHeader, err := io.ReadAll(fp)
	if err != nil {
		return
	}
	mHeader := strings.Split(string(bsHeader), "\n")
	for _, line := range mHeader {
		if strings.HasPrefix(line, "#") {
			continue
		}
		text := regexp.MustCompile(`\\"`).ReplaceAllString(line, "\"")
		row := strings.Split(text, "\t")
		if len(row) < 8 {
			continue
		}
		k := strings.ReplaceAll(row[5], "\"", "")
		v := strings.ReplaceAll(row[6], "\"", "")
		//expires := strings.ReplaceAll(row[4], "#HttpOnly_", "")
		r.req.AddCookie(&http.Cookie{
			Name:  k,
			Value: v,
		})
	}
}

func (r *Request) parseHeaders() {
	if r.opts.Headers != nil {
		for k, v := range r.opts.Headers {
			if vv, ok := v.(string); ok {
				r.req.Header.Set(k, vv)
				continue
			}
			if vv, ok := v.([]string); ok {
				for _, vvv := range vv {
					r.req.Header.Add(k, vvv)
				}
			}
		}
	}
}

func (r *Request) parseBody() {
	// application/x-www-form-urlencoded
	if r.opts.FormParams != nil {
		if _, ok := r.opts.Headers["Content-Type"]; !ok {
			r.opts.Headers["Content-Type"] = "application/x-www-form-urlencoded"
		}

		values := url.Values{}
		for k, v := range r.opts.FormParams {
			if vv, ok := v.(string); ok {
				values.Set(k, vv)
			}
			if vv, ok := v.([]string); ok {
				for _, vvv := range vv {
					values.Add(k, vvv)
				}
			}
		}
		r.body = strings.NewReader(values.Encode())

		return
	}

	// application/json
	if r.opts.JSON != nil {
		if _, ok := r.opts.Headers["Content-Type"]; !ok {
			r.opts.Headers["Content-Type"] = "application/json"
		}

		b, err := json.Marshal(r.opts.JSON)
		if err == nil {
			r.body = bytes.NewReader(b)

			return
		}
	}
	// []byte
	if r.opts.Body != nil {
		r.body = bytes.NewReader(r.opts.Body)
	}
	return
}
