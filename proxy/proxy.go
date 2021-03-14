package proxy

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const geoHeaderName = "X-Geo-Country"

type filterFunc func(string) bool
type actionFunc func(res http.ResponseWriter, req *http.Request)
type resolveCityFunc func(ipAddress net.IP) (*geoip2.Country, error)

type geoProxy struct {
	port      uint
	dbPath    string
	targetUrl string
	filter    filterFunc
	action    actionFunc
	resolve   resolveCityFunc
	db        *geoip2.Reader
	dbLock    *sync.RWMutex
	logger    *zap.Logger
}

// StartOption defines functions used to configure a proxy server
type StartOption func(*geoProxy) (*geoProxy, error)

func defaultAction(res http.ResponseWriter, _ *http.Request) {
	res.WriteHeader(http.StatusForbidden)
}

// WithMessage is used to configure a proxy to make it return a message when request is blocked.
func WithMessage(message string) StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
		const tmpl = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>%s</body></html>`
		responseData := []byte(fmt.Sprintf(tmpl, message))
		proxy.action = func(res http.ResponseWriter, req *http.Request) {
			_, _ = res.Write(responseData)
		}

		return proxy, nil
	}
}

// WithFile is used to configure a proxy to make it return a file content when request is blocked.
func WithFile(filePath string) StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
		proxy.action = func(res http.ResponseWriter, req *http.Request) {
			http.ServeFile(res, req, filePath)
		}
		return proxy, nil
	}
}

// WithAutoReload is used to configure a proxy to automatically reload when GeoIP database is updated.
func WithAutoReload() StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
		if err := proxy.startWatchingDb(); err != nil {
			return nil, err
		}

		proxy.resolve = proxy.resolveIpWithLock
		return proxy, nil
	}
}

// WithRedirect is used to configure a proxy to redirect a client to the specified URL when request is blocked.
func WithRedirect(redirectUrl string) StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
		proxy.action = func(res http.ResponseWriter, req *http.Request) {
			http.Redirect(res, req, redirectUrl, http.StatusTemporaryRedirect)
		}

		return proxy, nil
	}
}

// WithNoFilter is used by default when no other options are specified.
// It acts as a no-op and does not block any requests.
func WithNoFilter() StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
		proxy.filter = func(string) bool {
			return true
		}

		return proxy, nil
	}
}

// WithAllowedCountries is used to configure a proxy to allow requests coming form a list of specified countries.
// All other requests will be blocked.
func WithAllowedCountries(countries []string) StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
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

// WithBlockedCountries is used to configure a proxy to block requests coming form a list of specified countries.
// All other requests will be allowed.
func WithBlockedCountries(countries []string) StartOption {
	return func(proxy *geoProxy) (*geoProxy, error) {
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

// New is used to create a new instance of geoProxy
func New(port uint, database string, target string, opts ...StartOption) (*geoProxy, error) {
	proxy := &geoProxy{
		port:      port,
		dbPath:    database,
		targetUrl: target,
		action:    defaultAction,
		dbLock:    new(sync.RWMutex),
	}

	proxy.resolve = proxy.resolveIp

	for _, opt := range opts {
		_, err := opt(proxy)
		if err != nil {
			return nil, err
		}
	}

	return proxy, nil
}

func loadGeoDb(path string) (*geoip2.Reader, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		var reason string
		if os.IsNotExist(err) {
			reason = fmt.Sprintf("file '%s' does not exist", path)
		} else {
			reason = fmt.Sprintf("failed to open '%s' file", path)
		}
		return nil, errors.Errorf("Can not load GeoLite database, %s\n", reason)
	}

	return db, nil
}

func (p *geoProxy) reloadGeoDb() error {
	newDb, err := loadGeoDb(p.dbPath)
	if err != nil {
		return err
	}

	var oldDb *geoip2.Reader

	p.dbLock.Lock()
	oldDb = p.db
	p.db = newDb
	p.dbLock.Unlock()

	return oldDb.Close()
}

func (p *geoProxy) resolveIp(ip net.IP) (*geoip2.Country, error) {
	return p.db.Country(ip)
}

func (p *geoProxy) resolveIpWithLock(ip net.IP) (*geoip2.Country, error) {
	p.dbLock.RLock()
	defer p.dbLock.Unlock()

	return p.resolveIp(ip)
}

func (p *geoProxy) getHandler() func(http.ResponseWriter, *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		addr := getRemoteAddr(req)
		ip := getIP(addr)

		if ip == nil {
			p.logger.Info("can't get IP address for request",
				zap.String("addr", addr),
			)
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		country, err := p.resolve(ip)
		if err != nil {
			p.logger.Info("can't find a country by ip",
				zap.String("ip", ip.String()),
			)
			p.action(res, req)
			return
		}

		allowed := p.filter(country.Country.IsoCode)
		if !allowed {
			p.logger.Info("forbidden country",
				zap.String("ip", ip.String()),
				zap.String("country", country.Country.Names["en"]),
			)
			p.action(res, req)
			return
		}

		req.Header.Set(geoHeaderName, country.Country.IsoCode)

		serveReverseProxy(p.targetUrl, res, req)
	}
}

func (p *geoProxy) setupDbWatcher(wg *sync.WaitGroup) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		_ = watcher.Close()
	}()

	watcherWG := sync.WaitGroup{}
	watcherWG.Add(1)

	go func() {
		for {
			select {
			case event, more := <-watcher.Events:
				if !more {
					watcherWG.Done()
					p.logger.Info("failed watcher has stopped, Geo DB will not be reloaded automatically")
					return
				}

				realPath, _ := filepath.EvalSymlinks(p.dbPath)
				const writeOrCreateMask = fsnotify.Write | fsnotify.Create
				if filepath.Clean(event.Name) == realPath && event.Op&writeOrCreateMask != 0 {
					err := p.reloadGeoDb()
					if err != nil {
						p.logger.Error("failed to reload Geo DB",
							zap.Error(err),
						)
					} else {
						p.logger.Info("Geo DB is reloaded")
					}
				}

			case err, more := <-watcher.Errors:
				if more { // 'Errors' channel is not closed
					p.logger.Error("file watcher has failed, Geo DB will not be reloaded automatically",
						zap.Error(err),
					)
				}
				watcherWG.Done()
				return
			}
		}
	}()

	dir := filepath.Dir(p.dbPath)
	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	wg.Done()
	watcherWG.Wait()

	return nil
}

func (p *geoProxy) startWatchingDb() error {
	setupWG := sync.WaitGroup{}
	setupWG.Add(1)

	var err error
	go func() {
		err = p.setupDbWatcher(&setupWG)
	}()

	setupWG.Wait()

	return err
}

func (p *geoProxy) Start() error {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync()
	}()
	p.logger = logger

	db, err := loadGeoDb(p.dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := p.db.Close(); err != nil {
			p.logger.Error("failed to close Geo DB")
		}
	}()
	p.db = db

	addr := fmt.Sprintf(":%d", p.port)
	p.logger.Info("starting server",
		zap.String("addr", addr),
		zap.String("db", p.dbPath),
	)

	handler := p.getHandler()
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(addr, nil); err != nil {
		return errors.Errorf("Failed to start server: %v\n", err)
	}

	return nil
}
