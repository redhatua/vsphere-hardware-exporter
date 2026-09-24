// Package config parses flags and environment into a validated Config.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config is the validated runtime configuration.
type Config struct {
	URL      *url.URL
	Username string
	Password string
	CAFile   string
	Insecure bool

	ListenAddress   string
	RefreshInterval time.Duration
	RefreshTimeout  time.Duration
	Include         *regexp.Regexp
	Exclude         *regexp.Regexp
	ExportSerial    bool
	LogLevel        slog.Level
	ShowVersion     bool
}

// Parse reads args (without the program name) with getenv as the fallback source for defaults.
// Flags override environment variables. Credentials are never accepted as flags or URL userinfo.
func Parse(args []string, getenv func(string) string) (*Config, error) {
	flags := flag.NewFlagSet("vsphere-hardware-exporter", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	str := func(name, envKey, def, usage string) *string {
		d := def
		if v := getenv(envKey); v != "" {
			d = v
		}
		return flags.String(name, d, usage+" [env "+envKey+"]")
	}
	envErrs := map[string]error{} // flag name -> error in its environment default
	boolean := func(name, envKey, usage string) *bool {
		var d bool
		if v := getenv(envKey); v != "" {
			var err error
			if d, err = strconv.ParseBool(v); err != nil {
				envErrs[name] = fmt.Errorf("invalid boolean in environment variable %s", envKey)
			}
		}
		return flags.Bool(name, d, usage+" [env "+envKey+"]")
	}
	dur := func(name, envKey string, def time.Duration, usage string) *time.Duration {
		d := def
		if v := getenv(envKey); v != "" {
			p, err := time.ParseDuration(v)
			if err != nil {
				envErrs[name] = fmt.Errorf("invalid duration in environment variable %s", envKey)
			} else {
				d = p
			}
		}
		return flags.Duration(name, d, usage+" [env "+envKey+"]")
	}

	rawURL := str("vsphere.url", "VSPHERE_URL", "", "vCenter or ESXi URL, e.g. https://vcenter.example.com")
	user := str("vsphere.username", "VSPHERE_USERNAME", "", "read-only account name (password comes from VSPHERE_PASSWORD or --vsphere.password-file)")
	pwFile := str("vsphere.password-file", "VSPHERE_PASSWORD_FILE", "", "file containing the password")
	caFile := str("vsphere.ca-file", "VSPHERE_CA_FILE", "", "PEM CA bundle used to verify the server certificate")
	insecure := boolean("vsphere.insecure-skip-verify", "VSPHERE_INSECURE_SKIP_VERIFY", "skip TLS verification (insecure; explicit opt-in)")
	listen := str("web.listen-address", "WEB_LISTEN_ADDRESS", ":9877", "address to listen on")
	interval := dur("refresh-interval", "REFRESH_INTERVAL", time.Hour, "how often to refresh the inventory")
	timeout := dur("refresh-timeout", "REFRESH_TIMEOUT", 2*time.Minute, "timeout of one refresh")
	include := str("host-include", "HOST_INCLUDE", "", "regexp; only hosts whose name matches are exported")
	exclude := str("host-exclude", "HOST_EXCLUDE", "", "regexp; hosts whose name matches are skipped")
	serial := boolean("export-serial", "EXPORT_SERIAL", "export vsphere_host_serial_info (serial numbers / service tags)")
	level := str("log-level", "LOG_LEVEL", "info", "debug, info, warn or error")
	version := flags.Bool("version", false, "print version and exit")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	// A malformed environment value is an error only if no flag overrides it.
	flags.Visit(func(f *flag.Flag) { delete(envErrs, f.Name) })
	if len(envErrs) > 0 {
		names := make([]string, 0, len(envErrs))
		for n := range envErrs {
			names = append(names, n)
		}
		sort.Strings(names)
		errs := make([]error, 0, len(names))
		for _, n := range names {
			errs = append(errs, envErrs[n])
		}
		return nil, errors.Join(errs...)
	}
	c := &Config{
		Username: *user, CAFile: *caFile, Insecure: *insecure, ListenAddress: *listen,
		RefreshInterval: *interval, RefreshTimeout: *timeout, ExportSerial: *serial, ShowVersion: *version,
	}
	if c.ShowVersion {
		return c, nil
	}

	if err := c.LogLevel.UnmarshalText([]byte(*level)); err != nil {
		return nil, fmt.Errorf("invalid --log-level %q", *level)
	}
	if c.RefreshInterval < time.Minute {
		return nil, errors.New("--refresh-interval must be at least 1m")
	}
	if c.RefreshTimeout <= 0 {
		return nil, errors.New("--refresh-timeout must be positive")
	}
	var err error
	if c.URL, err = parseURL(*rawURL); err != nil {
		return nil, err
	}
	if c.Username == "" {
		return nil, errors.New("vsphere username is required (--vsphere.username or VSPHERE_USERNAME)")
	}
	if c.Password, err = password(*pwFile, getenv("VSPHERE_PASSWORD")); err != nil {
		return nil, err
	}
	if c.Include, err = compile("host-include", *include); err != nil {
		return nil, err
	}
	if c.Exclude, err = compile("host-exclude", *exclude); err != nil {
		return nil, err
	}
	return c, nil
}

func parseURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("vsphere url is required (--vsphere.url or VSPHERE_URL)")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New("vsphere url is not a valid URL")
	}
	if u.Scheme != "https" {
		return nil, errors.New("vsphere url scheme must be https; credentials are never sent over plain HTTP")
	}
	if u.User != nil {
		return nil, errors.New("vsphere url must not contain credentials; use VSPHERE_USERNAME and VSPHERE_PASSWORD or --vsphere.password-file")
	}
	if strings.Trim(u.Path, "/") == "" {
		u.Path = "/sdk"
	}
	return u, nil
}

func password(file, envValue string) (string, error) {
	switch {
	case file != "" && envValue != "":
		return "", errors.New("set only one password source: VSPHERE_PASSWORD or --vsphere.password-file")
	case file != "":
		b, err := os.ReadFile(file) //nolint:gosec // path is an operator-supplied flag
		if err != nil {
			reason := err.Error()
			var pe *fs.PathError
			if errors.As(err, &pe) {
				reason = pe.Err.Error()
			}
			return "", errors.New("cannot read password file: " + reason)
		}
		pw := strings.TrimRight(string(b), "\r\n")
		if pw == "" {
			return "", errors.New("password file is empty")
		}
		return pw, nil
	case envValue != "":
		return envValue, nil
	}
	return "", errors.New("vsphere password is required (VSPHERE_PASSWORD or --vsphere.password-file)")
}

func compile(name, expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid --%s regexp: %w", name, err)
	}
	return re, nil
}
