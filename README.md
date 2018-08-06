# kPortal

Thanks to kubebuilder, tcpproxy and gdns!

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

### On other *nix

TODO! - just set all your DNS resolution to hit 127.0.0.1:1053, although this might break some coffee shop wifi.


### Everywhere

Once you're configured to use 127.0.0.1:1053 for dns resolution you actually need to run kPortal
Run kPortal
```
# You don't need sudo if none of your services are listening on ports <1000
sudo KUBECONFIG="${HOME}/.kube/config" go run ./cmd/manager/main.go
```

At this point you should find that all your internets are working fine because kPortal delegates to cloudflare DNS for addresses it doesn't know about and right now we've not given it anything to think about. 


Use `wildcards.kportal.io` to annotate your services with the subdomains that they provides (takes a comma separated list)
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
