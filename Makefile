SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

# Image URL to use all building/pushing image targets
IMG ?= controller:latest

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/eastlondoner/kportal/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	gsed -i 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# Run in a docker image
kubeconfig = "/root/.kube/config"
docker-run: docker-build
	# By running kportal as a docker daemon on a bridge network, we get a dedicated routable IP;
	# that means we can expose a DNS server at port 53, which is required if we want to configure resolv.conf
	# to go to it (no way to override the port). It also means we don't need root to expose other ports we
	# proxy.
	docker run \
	  --name neo4j-cloud-dns \
	  -d \
	  -e MINIKUBE_IP="$$(minikube ip)" \
	  -v "${HOME}/.minikube":"${HOME}/.minikube":ro \
	  -v "${HOME}/.kube/config":"$(kubeconfig)":ro -e KUBECONFIG="$(kubeconfig)" \
	  ${IMG}

	# Find the IP kportal got assigned
	ip="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' neo4j-cloud-dns)"
	# And tell the host system it should do DNS via that IP
	bin/configure-kportal-as-dns "${ip}"
