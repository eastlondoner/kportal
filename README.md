# kPortal

Thanks to kubebuilder, tcpproxy and gdns!

This is a sketchy PoC - there aren't proper tests and it may not work for you - please open issues if you try it and it doesnt work.

## Why?

I use Minikube, it's awesome and allows me to do fast development cycles. But it has some painful limitations and doesnt always behave like a real K8s cluster. Which results in painful workarounds, extra client code for workarounds and general dissatisfaction.

First: In my client applications I want to tell them to communicate with public services using hostname:standard port e.g. for HTTPS I want to tell my client to use dev.mydomain.com:443 (:443 is implied when you use a https url). I don't want to pollute my client applications with code that knows about Kubernetes/Minikube (why should I have to do that?).

Second: When developing applications to run inside the cluster I want them to be able to communicate with "well known" addresses like `my-sql.my-namespace.svc.cluster.local:3306` _when I am running the code on my laptop but not inside of minikube_.

Third: I want these things to use SSL by default and not have to futz around with special/different certificate management

Common solutions to these problems either involve adding application to discover how to address your services at runtime or long winded configuration management / templating / generation to tell your applications to behave in a special way for local minikube work.
You typically have to futz around with DNS servers (like  dnsmasq), SSL tunnels and other runtime hacks. Often these don't play well together e.g. if you mhave multiple services that use the same public port.

kPortal is an independent DNS & Proxy server that monitors your minikube cluster to automatically do the futzing for you. So your applicaitons don't need to be aware of minikube and can use the same hostnames and ports they would use in a "real" K8s cluster. 


## How?

The DNS server part allows kPortal to tell your system to route hosts that minikube provides to the kPortal proxy.

The proxy part allows kPortal to listen on the official port of your services and uses SNI or http HOST header to route requests to the correct NodePort in your minikube. So even if you have multiple services with the same public port they are all supported.  


## Running kPortal

run `dep ensure` to install runtime dependencies.

### On OSX

Assuming you have minikube already running

Tell resolver to resolve DNS queries for **.domain.com at 127.0.0.1:1053 
```
echo "nameserver 127.0.0.1
port 1053" > /etc/resolver/domain.com
# You have to restart or log out and log in again for this change to take effect
```

n.b. `/etc/resolver/foo` is a neat feature of OSX that allows you to specify a nameserver to use to lookup *.foo domains. 

For cluster.local service routing:
```
echo "nameserver 127.0.0.1
port 1053" > /etc/resolver/cluster.local
# You have to restart or log out and log in again for this change to take effect
```

### On other *nix

TODO! - I have a mac, so I haven't tried this but it should be fine to set all your DNS resolution to hit 127.0.0.1:1053, although this might break some coffee shop wifi.


### Everywhere

Once you're configured to use 127.0.0.1:1053 for dns resolution you actually need to run kPortal
Run kPortal
```
# You don't need sudo if none of your services are listening on ports <1000
sudo KUBECONFIG="${HOME}/.kube/config" go run ./cmd/manager/main.go
```

At this point you should find that all your internets are working fine because kPortal delegates to cloudflare DNS for addresses it doesn't know about.
# TODO: allow the upstream DNS to be configurable

Now kPortal allows you to call the cluster internal `*.svc.cluster.local:port` addresses as though you were 'inside' minikube from your local machine, provided you are using TLS or HTTP.
The TLS/HTTP restriction is because kPortal needs to know how to route your connections. With TLS SNI information is used for routing, with HTTP the HOST header is used. An insecure TCP connection contains no routing information so we don't know what to do with it.

You can tell kPortal to route other domains to a service by annotating the services in K8s.

Use `wildcards.kportal.io` to annotate your services with the subdomains that they provide (the wildcards.kportal.io takes a comma separated list - no spaces!)
```
kubectl annotate svc my-foo-service wildcards.kportal.io="*.foo.mydomain.com,*.test.mydomain.com"
kubectl annotate svc my-bar-service wildcards.kportal.io="*.bar.mydomain.com"
```

TODO: run kPortal in docker with its own IP so it's not using all your ports!  


## What happens

*.foo.mydomain.com, *.test.mydomain.com and *.bar.mydomain.com will all resolve to 127.0.0.1
n.b. these wildcards match any depth (unlike wildcard DNS)

kPortal looks at your services in K8s and listens on 127.0.0.1 on each public service port.
 
When you make a TLS connection (e.g. HTTPS) on any of those ports kPortal looks at the Server Name Indication to determine which service you wanted to talk to.
If the SNI hostname on the TLS connection matches the annotation on a service then the request is routed to the nodeport for that service.
Because of this routing it's possible to have multiple services on a single port (e.g. on port 443).

N.b. kPortal is not terminating SSL - that's up to you
N.b. kPortal is not able to route insecure connections (e.g. HTTP)
N.b. Normally on minikube you cannot connect via the service port (e.g. for load balancer services) and you have to figure out the (dynamically assigned) nodePort. No longer! 


## Coming soon

1) Example using the K8s cluster to provide signed certs of *.svc.cluster.local domains for your services.
1) Example using a cloud DNS provider and letsencrypt staging server to get SSL certs for wildcard domains _in minikube_.