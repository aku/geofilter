package proxy

import (
  "net"
  "net/http"
  "net/http/httputil"
  "net/url"
)

func getRemoteAddr(r *http.Request) string {
  forwarded := r.Header.Get("X-Forwarded-For")
  if forwarded != "" {
    return forwarded
  }

  realIp := r.Header.Get("X-Real-Ip")
  if realIp != "" {
    return realIp
  }

  return r.RemoteAddr
}

func getIP(addr string) net.IP {
  ip := net.ParseIP(addr)
  if ip == nil {
    if host, _, err := net.SplitHostPort(addr); err == nil {
      return net.ParseIP(host)
    }
  }

  return ip
}

func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
  targetUrl, _ := url.Parse(target)

  proxy := httputil.NewSingleHostReverseProxy(targetUrl)

  // Update the headers to allow for SSL redirection
  req.URL.Host = targetUrl.Host
  req.URL.Scheme = targetUrl.Scheme
  req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
  req.Host = targetUrl.Host

  proxy.ServeHTTP(res, req)
}
