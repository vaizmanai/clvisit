package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
)

type direct struct{}

// Direct is a direct proxy: one that makes network connections directly.
var Direct = direct{}

func (direct) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

// httpsDialer
type httpsDialer struct{}

// HTTPSDialer is a https proxy: one that makes network connections on tls.
var HttpsDialer = httpsDialer{}
var TlsConfig = &tls.Config{}

func (d httpsDialer) Dial(_, addr string) (c net.Conn, err error) {
	return tls.Dial("tcp", addr, TlsConfig)
}

// httpProxy is a HTTP/HTTPS connect proxy.
type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}

	return s, nil
}

func (s *httpProxy) Dial(_, addr string) (net.Conn, error) {
	// Dial and create the https client connection.
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse(fmt.Sprintf("http://%s", addr))
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest(http.MethodConnect, reqURL.String(), nil)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("User-Agent", "revisit communicator")

	err = req.Write(c)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		// TODO close resp body ?
		_ = resp.Body.Close()
		_ = c.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = c.Close()
		return nil, fmt.Errorf("connect server using proxy error, StatusCode [%d]", resp.StatusCode)
	}

	return c, nil
}

func RegisterDialerType() {
	proxy.RegisterDialerType("http", newHTTPProxy)
}

func FromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return proxy.FromURL(u, forward)
}

func FromEnvironment() proxy.Dialer {
	return proxy.FromEnvironment()
}
