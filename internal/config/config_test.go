package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestParseDefaults(t *testing.T) {
	c, err := Parse([]string{"--vsphere.url=https://vc.example.com"}, env(map[string]string{
		"VSPHERE_USERNAME": "ro@vsphere.local", "VSPHERE_PASSWORD": "pw",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.URL.String() != "https://vc.example.com/sdk" {
		t.Errorf("url = %s (want /sdk appended)", c.URL)
	}
	if c.ListenAddress != ":9877" || c.RefreshInterval != time.Hour || c.ExportSerial || c.Insecure {
		t.Errorf("defaults wrong: %+v", c)
	}
	if c.Password != "pw" || c.Username != "ro@vsphere.local" {
		t.Errorf("credentials not read from env")
	}
}

func TestParseEnvAndFlagPrecedence(t *testing.T) {
	c, err := Parse([]string{"--refresh-interval=30m", "--export-serial"}, env(map[string]string{
		"VSPHERE_URL": "https://vc.example.com/sdk", "VSPHERE_USERNAME": "u", "VSPHERE_PASSWORD": "p",
		"REFRESH_INTERVAL": "5m", "VSPHERE_INSECURE_SKIP_VERIFY": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.RefreshInterval != 30*time.Minute {
		t.Errorf("flag must beat env, got %v", c.RefreshInterval)
	}
	if !c.Insecure || !c.ExportSerial {
		t.Errorf("insecure/serial not set: %+v", c)
	}
}

func TestPasswordFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(f, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Parse([]string{"--vsphere.url=https://vc.example.com", "--vsphere.username=u", "--vsphere.password-file=" + f}, env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Password != "from-file" {
		t.Errorf("password = %q", c.Password)
	}
}

func TestParseErrors(t *testing.T) {
	base := map[string]string{"VSPHERE_URL": "https://vc.example.com", "VSPHERE_USERNAME": "u", "VSPHERE_PASSWORD": "secret-pw"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for kk, vv := range base {
			m[kk] = vv
		}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{"no url", nil, with("VSPHERE_URL", ""), "url"},
		{"no username", nil, with("VSPHERE_USERNAME", ""), "username"},
		{"no password", nil, with("VSPHERE_PASSWORD", ""), "password"},
		{"creds in url", nil, with("VSPHERE_URL", "https://u:hunter2@vc.example.com"), "credentials"},
		{"bad scheme", nil, with("VSPHERE_URL", "ftp://vc.example.com"), "scheme"},
		{"http rejected", nil, with("VSPHERE_URL", "http://vc.example.com"), "https"},
		{"bad bool env", nil, func() map[string]string { m := with("X", ""); m["EXPORT_SERIAL"] = "yes-please"; return m }(), "EXPORT_SERIAL"},
		{"bad insecure env", nil, func() map[string]string { m := with("X", ""); m["VSPHERE_INSECURE_SKIP_VERIFY"] = "tru"; return m }(), "VSPHERE_INSECURE_SKIP_VERIFY"},
		{"bad duration env", nil, func() map[string]string { m := with("X", ""); m["REFRESH_INTERVAL"] = "1 hour"; return m }(), "REFRESH_INTERVAL"},
		{"bad regex", []string{"--host-include=("}, base, "host-include"},
		{"short interval", []string{"--refresh-interval=1s"}, base, "refresh-interval"},
		{"bad level", []string{"--log-level=loud"}, base, "log-level"},
		{"both pw sources", []string{"--vsphere.password-file=/nonexistent"}, base, "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args, env(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want mention of %q", err, tt.want)
			}
			for _, secret := range []string{"hunter2", "secret-pw"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error leaks secret: %v", err)
				}
			}
		})
	}
}
