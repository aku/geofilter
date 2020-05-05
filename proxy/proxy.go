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

const GeoHeaderName = "X-Geo-Country"

type filterFunc func(string) bool
type actionFunc func(res http.ResponseWriter, req *http.Request)

type GeoProxy struct {
	port      uint
	dbPath    string
	targetUrl string
	filter    filterFunc
	action    actionFunc
	db        *geoip2.Reader
}

type StartOption func(*GeoProxy) (*GeoProxy, error)

func WithMessage(message string) StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		const tmpl = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>%s</body></html>`
		responseData := []byte(fmt.Sprintf(tmpl, message))
		proxy.action = func(res http.ResponseWriter, req *http.Request) {
			_, _ = res.Write(responseData)
		}

		return proxy, nil
	}
}

func WithRedirect(redirectUrl string) StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		proxy.action = func(res http.ResponseWriter, req *http.Request) {
			http.Redirect(res, req, redirectUrl, http.StatusTemporaryRedirect)
		}

		return proxy, nil
	}
}

func WithNoFilter() StartOption {
	return func(proxy *GeoProxy) (*GeoProxy, error) {
		proxy.filter = func(string) bool {
			return true
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

func defaultAction(res http.ResponseWriter, _ *http.Request) {
	res.WriteHeader(http.StatusForbidden)
}

func (p *GeoProxy) getHandler() func(http.ResponseWriter, *http.Request) {
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

		country, err := p.db.Country(ip)
		if err != nil {
			logger.Info("can't find a country by ip",
				zap.String("ip", ip.String()),
			)
			p.action(res, req)
			return
		}

		allowed := p.filter(country.Country.IsoCode)
		if allowed == false {
			logger.Info("forbidden country",
				zap.String("country", country.Country.Names["en"]),
			)
			p.action(res, req)
			return
		}

		req.Header.Set(GeoHeaderName, country.Country.IsoCode)

		serveReverseProxy(p.targetUrl, res, req)
	}
}

func (p *GeoProxy) loadGeoDb() error {
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

	p.db = db
	return nil
}

func (p *GeoProxy) closeGeoDb() error {
	if p.db != nil {
		return p.db.Close()
	}

	return nil
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
	if err := p.loadGeoDb(); err != nil {
		return err
	}

	defer func() {
		p.closeGeoDb()
	}()

	addr := fmt.Sprintf(":%d", p.port)

	log.Printf("starting server on %d\n", p.port)

	handler := p.getHandler()
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(addr, nil); err != nil {
		return errors.Errorf("Failed to start server: %v\n", err)
	}

	return nil
}
