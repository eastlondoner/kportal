package proxy

import (
	"fmt"
	"github.com/eastlondoner/gdns"
	"github.com/miekg/dns"
	"github.com/subchen/go-log"
)

type DNSNameserver struct {
	conf    gdns.Conf
	handler dns.Handler
}

func NewNameserver(myIP string, bindPort int) *DNSNameserver {

	myDomains := new(gdns.Hostitem)

	myDomains.Add(
		"*.cluster.local",
		myIP,
	)

	config := gdns.Conf{
		Listen: []gdns.Addr{
			gdns.Addr{
				Host:    "0.0.0.0",
				Port:    bindPort,
				Network: "udp",
			},
			gdns.Addr{
				Host:    "0.0.0.0",
				Port:    bindPort,
				Network: "tcp",
			},
		},
		ForwardRules: []gdns.Rule{},
		DefaultUpstream: []gdns.Addr{
			gdns.Addr{
				Host:    "1.1.1.1",
				Port:    53,
				Network: "udp",
			},
			gdns.Addr{
				Host:    "1.1.1.1",
				Port:    53,
				Network: "tcp",
			},
		},
		Hosts:   myDomains,
		Timeout: 30,
	}

	h := gdns.NewDNSHandler(&config)
	return &DNSNameserver{
		conf:    config,
		handler: h,
	}
}

func (d *DNSNameserver) Run() {
	// TODO take a stop ch

	for _, l := range d.conf.Listen {
		go func(l gdns.Addr) {
			log.Infof("listen on %s %s:%d", l.Network, l.Host, l.Port)
			if err := dns.ListenAndServe(
				fmt.Sprintf("%s:%d", l.Host, l.Port), l.Network, d.handler); err != nil {
				log.Fatal(err)
			}
		}(l)
	}
}

func (d *DNSNameserver) AddHost(domain, ip string) {
	d.conf.Hosts.Add(domain, ip)
}

func (d *DNSNameserver) RemoveHost(domain, ip string, t int) {
	d.conf.Hosts.Remove(domain, ip)
}
