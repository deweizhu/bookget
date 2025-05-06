package gohttp

import "context"

// NewClient new request object
func NewClient(c context.Context, opts ...Options) *Request {
	req := &Request{ctx: c}
	if len(opts) > 0 {
		req.opts = opts[0]
	} else {
		req.opts = Options{}
	}
	return req
}

// Get send get request
func Get(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.Get(uri, opts...)
}

// Post send post request
func Post(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.Post(uri, opts...)
}

// Put send put request
func Put(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.Post(uri, opts...)
}

// Patch send patch request
func Patch(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.Patch(uri, opts...)
}

// Delete send delete request
func Delete(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.Delete(uri, opts...)
}

// Download file
func FastGet(c context.Context, uri string, opts ...Options) (*Response, error) {
	r := NewClient(c)
	return r.FastGet(uri, opts...)
}
