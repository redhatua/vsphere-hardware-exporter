// Command vcsim runs a govmomi vCenter simulator for local development and demos.
// It is not part of the exporter and ships in no release artifact.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	listen := flag.String("l", "127.0.0.1:8989", "listen address")
	user := flag.String("username", "user", "login name")
	pass := flag.String("password", "pass", "password")
	esx := flag.Bool("esx", false, "simulate a standalone ESXi host instead of vCenter")
	flag.Parse()

	m := simulator.VPX()
	if *esx {
		m = simulator.ESX()
	}
	if err := m.Create(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer m.Remove()

	m.Service.TLS = new(tls.Config)
	m.Service.Listen = &url.URL{Host: *listen, User: url.UserPassword(*user, *pass)}
	s := m.Service.NewServer() // self-signed TLS, like a lab vCenter
	defer s.Close()

	fmt.Printf("vcsim listening on https://%s/sdk (user %q)\n", *listen, *user)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
