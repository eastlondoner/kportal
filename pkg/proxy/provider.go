package proxy

import (
	"context"
	"fmt"
	"github.com/google/tcpproxy"
	"k8s.io/api/core/v1"
	"log"
	"strings"
	"sync"
)

func Run() {
	var p tcpproxy.Proxy
	p.AddHTTPHostRoute(":80", "foo.com", tcpproxy.To("10.0.0.1:8081"))
	p.AddHTTPHostRoute(":80", "bar.com", tcpproxy.To("10.0.0.2:8082"))
	p.AddRoute(":80", tcpproxy.To("10.0.0.1:8081")) // fallback
	p.AddSNIRoute(":443", "foo.com", tcpproxy.To("10.0.0.1:4431"))
	p.AddSNIRoute(":443", "bar.com", tcpproxy.To("10.0.0.2:4432"))
	p.AddRoute(":443", tcpproxy.To("10.0.0.1:4431")) // fallback
	log.Fatal(p.Run())
}

type Proxies struct {
	*tcpproxy.Proxy
	MinikubeIP string
	ProxyIP    string
	mutex      sync.Mutex
	dns        *DNSNameserver
}

func New(minikubeIP, proxyIP string, dnsBindPort int) *Proxies {
	// TODO add a signalhandler to close the tcpProxy
	return &Proxies{
		Proxy:      nil,
		MinikubeIP: minikubeIP,
		ProxyIP:    proxyIP,
		mutex:      sync.Mutex{},
		dns:        NewNameserver(minikubeIP, dnsBindPort),
	}
}

func (p *Proxies) RunDNS() {
	p.dns.Run()
}

// Using map[string]bool to implement set[string] because this is go
func (p *Proxies) ReconfigureProxies(servicesByNamespace map[string]map[string]v1.Service) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.Proxy != nil {
		// TODO find a proxy that will allow me to change rules at runtime or modify google's tcpproxy to allow that
		err := p.Close()
		if err != nil {
			panic(err)
		}
	}
	p.Proxy = &tcpproxy.Proxy{}

	for namespace, serviceList := range servicesByNamespace {
		for serviceName, svc := range serviceList {
			wildcards := make([]string, 0)
			if routesAnnotation, ok := svc.Annotations["wildcards.kportal.io"]; ok {
				wildcards = strings.Split(routesAnnotation, ",")
			}
			for _, port := range svc.Spec.Ports {
				if port.NodePort == 0 {
					continue
				}
				clusterHostname := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
				target := tcpproxy.To(fmt.Sprintf("%s:%d", p.MinikubeIP, port.NodePort))

				ipPort := fmt.Sprintf("0.0.0.0:%d", port.Port)
				//ipPort := fmt.Sprintf(":%d", port.Port)
				log.Printf("Routing %s:%s to %s", clusterHostname, ipPort, target.Addr)

				p.AddSNIRoute(ipPort, clusterHostname, target)
				p.dns.AddHost(clusterHostname, p.ProxyIP)
				p.dns.AddHost(clusterHostname, "::1") // TODO don't know how to do hostname -i for ipv6

				for _, wildcard := range wildcards {
					log.Printf("Routing %s:%s to %s", wildcard, ipPort, target.Addr)
					p.AddSNIMatchRoute(ipPort, hasSuffix(strings.Replace(wildcard, "*", "", 1)), target)
					p.dns.AddHost(wildcard, p.ProxyIP)
					p.dns.AddHost(wildcard, "::1")
				}
			}
		}
	}

	// TODO: what if run fails?
	go func() {
		log.Printf("Run...")
		log.Print(p.Run())
		log.Printf("End...")
	}()
	log.Printf("Done providing")
	return nil
}

// equals is a trivial Matcher that implements string equality.
func hasSuffix(want string) tcpproxy.Matcher {
	return func(_ context.Context, got string) bool {
		result := strings.HasSuffix(got, want)
		log.Printf("checking %s vs %s, %v", got, want, result)
		return result
	}
}
