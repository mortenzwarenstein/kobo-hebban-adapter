package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Proxy struct {
	target *url.URL
	rp     *httputil.ReverseProxy
}

func New(target string) *Proxy {
	u, err := url.Parse(target)
	if err != nil {
		panic("invalid proxy target: " + err.Error())
	}

	rp := httputil.NewSingleHostReverseProxy(u)

	original := rp.Director
	rp.Director = func(req *http.Request) {
		original(req)
		req.Host = u.Host
	}

	return &Proxy{target: u, rp: rp}
}

func (p *Proxy) Handler() http.HandlerFunc {
	return p.rp.ServeHTTP
}

func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request) {
	p.rp.ServeHTTP(w, r)
}
