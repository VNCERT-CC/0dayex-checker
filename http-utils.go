package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

var httpClientTimeout = 7 * time.Second
var dialTimeout = 7 * time.Second
var httpClient = &fasthttp.Client{
	TLSConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
	MaxIdemponentCallAttempts: 5, // retry if empty resp
	ReadTimeout:               httpClientTimeout,
	MaxConnsPerHost:           233,
	MaxIdleConnDuration:       15 * time.Minute,
	ReadBufferSize:            1024 * 8,
	Dial: func(addr string) (net.Conn, error) {
		// no suitable address found => ipv6 can not dial to ipv4,..
		hostname, port, err := net.SplitHostPort(addr)
		if err != nil {
			if err1, ok := err.(*net.AddrError); ok && strings.Index(err1.Err, "missing port") != -1 {
				hostname, port, err = net.SplitHostPort(strings.TrimRight(addr, ":") + ":80")
			}
			if err != nil {
				return nil, err
			}
		}
		if port == "" || port == ":" {
			port = "80"
		}
		return fasthttp.DialDualStackTimeout("["+hostname+"]:"+port, dialTimeout)
	},
}

var errEncodingNotSupported = errors.New("response content encoding not supported")

func getResponseBody(resp *fasthttp.Response) ([]byte, error) {
	var contentEncoding = resp.Header.Peek("Content-Encoding")
	if len(contentEncoding) < 1 {
		return resp.Body(), nil
	}
	if bytes.Equal(contentEncoding, []byte("gzip")) {
		return resp.BodyGunzip()
	}
	if bytes.Equal(contentEncoding, []byte("deflate")) {
		return resp.BodyInflate()
	}
	return nil, errEncodingNotSupported
}

func acquireRequest(url string) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	normalizeRequest(req)
	return req
}

func normalizeRequest(req *fasthttp.Request) {
	req.Header.Set(`User-Agent`, `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:106.0) Gecko/20100101 Firefox/106.0`)
	req.Header.Set(`Accept`, `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8`)
	req.Header.Set(`Accept-Language`, `en-US,en;q=0.5`)
	req.Header.Set(`Accept-Encoding`, `gzip, deflate`)
	req.Header.Set(`Connection`, `close`)
	req.Header.Set(`Upgrade-Insecure-Requests`, `1`)
	req.Header.Set(`Sec-Fetch-Dest`, `document`)
	req.Header.Set(`Sec-Fetch-Mode`, `navigate`)
	req.Header.Set(`Sec-Fetch-Site`, `cross-site`)
	req.Header.Set(`Pragma`, `no-cache`)
	req.Header.Set(`Cache-Control`, `no-cache`)
	req.Header.Set(`TE`, `trailers`)
}

func doRequestFollowRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirectsCount int, f func(*fasthttp.Response)) (err error) {
	redirectsCount := 0

	for {
		if err = httpClient.DoTimeout(req, resp, httpClientTimeout); err != nil {
			log.Println(err)
			break
		}
		if f != nil {
			f(resp)
		}
		if maxRedirectsCount == 1 {
			break
		}
		statusCode := resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(statusCode) {
			break
		}

		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = fasthttp.ErrTooManyRedirects
			break
		}
		location := resp.Header.Peek(`location`)
		if len(location) == 0 {
			err = fasthttp.ErrMissingLocation
			break
		}
		req.URI().UpdateBytes(location)
		resp.Reset()
	}

	return err
}
