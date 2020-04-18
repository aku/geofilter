package proxy

import (
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
)

type filterFunc func(string) bool
type actionFunc func(res http.ResponseWriter)

type GeoProxy struct {
	port      uint
	dbPath    string
	targetUrl string
	filter    filterFunc
	action    actionFunc
}

type StartOption func(*GeoProxy) (*GeoProxy, error)

func WithDefault() StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		return proxy, nil
	}
}

func WithMessage(message string) StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		const tmpl = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>%s</body></html>`
		responseData := []byte(fmt.Sprintf(tmpl, message))
		proxy.action = func(res http.ResponseWriter) {
			_, _ = res.Write(responseData)
		}

		return proxy, nil
	}
}

func WithAllowedCountries(countries []string) StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		if len(countries) == 0 {
			return nil, errors.New("allowed countries are not specified")
		}

		allowedCountries := make(map[string]bool)
		for _, c := range countries {
			allowedCountries[c] = true
		}

		proxy.filter = func(c string) bool {
			return allowedCountries[c]
		}

		return proxy, nil
	}
}

func WithBlockedCountries(countries []string) StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		if len(countries) == 0 {
			return nil, errors.New("blocked countries are not specified")
		}

		blockedCountries := make(map[string]bool)
		for _, c := range countries {
			blockedCountries[c] = true
		}

		proxy.filter = func(c string) bool {
			return !blockedCountries[c]
		}

		return proxy, nil
	}
}

func defaultAction(res http.ResponseWriter) {
	res.WriteHeader(http.StatusForbidden)
}

func (p *GeoProxy) getHandler(db *geoip2.Reader) func(http.ResponseWriter, *http.Request) {
	logger, _ := zap.NewProduction()

	defer func() {
		_ = logger.Sync()
	}()

	return func(res http.ResponseWriter, req *http.Request) {
		addr := getRemoteAddr(req)
		ip := getIP(addr)

		if ip == nil {
			logger.Info("can't get IP address for request",
				zap.String("addr", addr),
			)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		country, err := db.Country(ip)
		if err != nil {
			logger.Info("can't find a country by ip",
				zap.String("ip", ip.String()),
			)
			p.action(res)
			return
		}

		allowed := p.filter(country.Country.IsoCode)
		if allowed == false {
			logger.Info("forbidden country",
				zap.String("country", country.Country.Names["en"]),
			)
			p.action(res)
			return
		}

		serveReverseProxy(p.targetUrl, res, req)
	}
}

func New(port uint, database string, target string, opts ...StartOption) (*GeoProxy, error) {
	proxy := &GeoProxy{
		port:      port,
		dbPath:    database,
		targetUrl: target,
		action:    defaultAction,
	}

	for _, opt := range opts {
		_, err := opt(proxy)
		if err != nil {
			return nil, err
		}
	}

	return proxy, nil
}

func (p *GeoProxy) Start() error {
	//TODO: add support for DB update
	db, err := geoip2.Open(p.dbPath)
	if err != nil {
		var reason string
		if os.IsNotExist(err) {
			reason = fmt.Sprintf("file '%s' does not exist", p.dbPath)
		} else {
			reason = fmt.Sprintf("failed to open '%s' file", p.dbPath)
		}
		return errors.Errorf("Can not load GeoLite database, %s\n", reason)
	}
	defer func() {
		_ = db.Close()
	}()

	addr := fmt.Sprintf(":%d", p.port)

	log.Printf("starting server on %d\n", p.port)

	handler := p.getHandler(db)
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(addr, nil); err != nil {
		return errors.Errorf("Failed to start server: %v\n", err)
	}

	return nil
}
