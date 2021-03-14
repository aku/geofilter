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
	portFlag     = "port"
	databaseFlag = "database"
	targetFlag   = "target"
	messageFlag  = "message"
	redirectFlag = "redirect"
	fileFlag     = "file"
	watchFlag    = "watch"
	allowFlag    = "allow"
	blockFlag    = "block"
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
			}

			return nil, errors.Errorf("empty countries list")
		}

		return proxy.WithAllowedCountries(allowedCountries), nil
	}

	if len(blocked) > 0 {
		if len(blockedCountries) == 0 {
			if len(unknownCountries) == 0 {
				return nil, errors.Errorf("unknown country names: %v\n", unknownCountries)
			}

			return nil, errors.Errorf("empty countries list")
		}

		return proxy.WithBlockedCountries(blockedCountries), nil
	}

	return proxy.WithNoFilter(), nil
}

func startProxy(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetUint(portFlag)
	database, _ := cmd.Flags().GetString(databaseFlag)
	watch, _ := cmd.Flags().GetBool(watchFlag)
	target, _ := cmd.Flags().GetString(targetFlag)
	message, _ := cmd.Flags().GetString(messageFlag)
	redirect, _ := cmd.Flags().GetString(redirectFlag)
	file, _ := cmd.Flags().GetString(fileFlag)
	allowed, _ := cmd.Flags().GetString(allowFlag)
	blocked, _ := cmd.Flags().GetString(blockFlag)

	allowed = strings.TrimSpace(allowed)
	blocked = strings.TrimSpace(blocked)

	if len(allowed) > 0 && len(blocked) > 0 {
		return errors.Errorf("--%s and --%s options are mutually exclusive", allowFlag, blockFlag)
	}

	if len(message) > 0 && len(redirect) > 0 {
		return errors.Errorf("--%s and --%s options are mutually exclusive", redirectFlag, messageFlag)
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

	file = strings.TrimSpace(file)
	if len(file) > 0 {
		opts = append(opts, proxy.WithFile(file))
	}

	if watch {
		opts = append(opts, proxy.WithAutoReload())
	}

	geoProxy, err := proxy.New(port, database, target, opts...)
	if err != nil {
		return err
	}

	return geoProxy.Start()
}

// RunApp starts a proxy
func RunApp() {
	if err := startProxyCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	startProxyCmd.Flags().UintP(portFlag, "p", 80, "port")
	startProxyCmd.Flags().StringP(databaseFlag, "d", "GeoLite2-Country.mmdb", "Path to MaxMind database")
	startProxyCmd.Flags().BoolP(watchFlag, "w", false, "Watch for database file changes and reload automatically")
	startProxyCmd.Flags().StringP(targetFlag, "t", "", "Target URL")
	startProxyCmd.Flags().StringP(messageFlag, "m", "", "Message to show when request is blocked")
	startProxyCmd.Flags().StringP(redirectFlag, "r", "", "Redirect to the specified URL")
	startProxyCmd.Flags().StringP(fileFlag, "f", "", "File to show when request is blocked")
	startProxyCmd.Flags().StringP(allowFlag, "a", "", "List of allowed countries")
	startProxyCmd.Flags().StringP(blockFlag, "b", "", "List of blocked countries")

	_ = startProxyCmd.MarkFlagFilename(databaseFlag, "mmdb")
	_ = startProxyCmd.MarkFlagRequired(targetFlag)
}
