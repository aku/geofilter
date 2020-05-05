package commands

import (
	"geofilter/proxy"
	"github.com/biter777/countries"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

const (
	PortFlag     = "port"
	DatabaseFlag = "database"
	TargetFlag   = "target"
	MessageFlag  = "message"
	RedirectFlag = "redirect"
	WatchFlag    = "watch"
	AllowFlag    = "allow"
	BlockFlag    = "block"
)

var startProxyCmd = &cobra.Command{
	Use:     "geofilter",
	Short:   "Geo IP filter",
	Long:    "",
	Example: "geofilter --database=GeoLite2-Country.mmdb --port 3000 --allow US --target http://localhost:4001",
	Version: "", //BuildVersion,
	RunE:    startProxy,
}

func getCountriesOpt(allowed string, blocked string) (proxy.StartOption, error) {
	unknownCountries := make([]string, 0)
	allowedCountries := make([]string, 0)
	blockedCountries := make([]string, 0)

	for _, c := range strings.Split(allowed, ",") {
		if len(c) == 0 {
			continue
		}
		country := countries.ByName(c)
		if country == countries.Unknown {
			unknownCountries = append(unknownCountries, c)
		} else {
			allowedCountries = append(allowedCountries, country.Alpha2())
		}
	}

	for _, c := range strings.Split(blocked, ",") {
		if len(c) == 0 {
			continue
		}
		country := countries.ByName(c)
		if country == countries.Unknown {
			unknownCountries = append(unknownCountries, c)
		} else {
			blockedCountries = append(blockedCountries, country.Alpha2())
		}
	}

	if len(allowed) > 0 {
		if len(allowedCountries) == 0 {
			if len(unknownCountries) == 0 {
				return nil, errors.Errorf("unknown country names: %v\n", unknownCountries)
			} else {
				return nil, errors.Errorf("empty countries list")
			}
		} else {
			return proxy.WithAllowedCountries(allowedCountries), nil
		}
	}

	if len(blocked) > 0 {
		if len(blockedCountries) == 0 {
			if len(unknownCountries) == 0 {
				return nil, errors.Errorf("unknown country names: %v\n", unknownCountries)
			} else {
				return nil, errors.Errorf("empty countries list")
			}
		} else {
			return proxy.WithBlockedCountries(blockedCountries), nil
		}
	}

	return proxy.WithNoFilter(), nil
}

func startProxy(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetUint(PortFlag)
	database, _ := cmd.Flags().GetString(DatabaseFlag)
	watch, _ := cmd.Flags().GetBool(WatchFlag)
	target, _ := cmd.Flags().GetString(TargetFlag)
	message, _ := cmd.Flags().GetString(MessageFlag)
	redirect, _ := cmd.Flags().GetString(RedirectFlag)
	allowed, _ := cmd.Flags().GetString(AllowFlag)
	blocked, _ := cmd.Flags().GetString(BlockFlag)

	allowed = strings.TrimSpace(allowed)
	blocked = strings.TrimSpace(blocked)

	if len(allowed) > 0 && len(blocked) > 0 {
		return errors.Errorf("--%s and --%s options are mutually exclusive", AllowFlag, BlockFlag)
	}

	if len(message) > 0 && len(redirect) > 0 {
		return errors.Errorf("--%s and --%s options are mutually exclusive", RedirectFlag, MessageFlag)
	}

	countriesOpt, err := getCountriesOpt(allowed, blocked)
	if err != nil {
		return err
	}

	var opts []proxy.StartOption

	opts = append(opts, countriesOpt)

	message = strings.TrimSpace(message)
	if len(message) > 0 {
		opts = append(opts, proxy.WithMessage(message))
	}

	redirect = strings.TrimSpace(redirect)
	if len(redirect) > 0 {
		opts = append(opts, proxy.WithRedirect(redirect))
	}

	if watch {
		opts = append(opts, proxy.WithAutoReload())
	}

	geoProxy, err := proxy.New(port, database, target,  opts...)
	if err != nil {
		return err
	}

	return geoProxy.Start()
}

func RunApp() {
	if err := startProxyCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	startProxyCmd.Flags().UintP(PortFlag, "p", 80, "port")
	startProxyCmd.Flags().StringP(DatabaseFlag, "d", "GeoLite2-Country.mmdb", "Path to MaxMind database")
	startProxyCmd.Flags().BoolP(WatchFlag, "w", false, "Watch for database file changes and reload automatically")
	startProxyCmd.Flags().StringP(TargetFlag, "t", "", "Target URL")
	startProxyCmd.Flags().StringP(MessageFlag, "m", "", "Message to show when request is blocked")
	startProxyCmd.Flags().StringP(RedirectFlag, "r", "", "Redirect to the specified URL")
	startProxyCmd.Flags().StringP(AllowFlag, "a", "", "List of allowed countries")
	startProxyCmd.Flags().StringP(BlockFlag, "b", "", "List of blocked countries")

	_ = startProxyCmd.MarkFlagFilename(DatabaseFlag, "mmdb")
	_ = startProxyCmd.MarkFlagRequired(TargetFlag)
}
