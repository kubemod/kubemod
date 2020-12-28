# Image URL to use all building/pushing image targets
IMG ?= kubemod/kubemod:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./core ./util -coverprofile cover.out

# Run benchmarks
bench: generate fmt vet manifests
	go test ./core ./util -run=XXX -bench=.


# Build manager binary
manager: generate fmt vet
	go build -o bin/kubemod main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

bundle: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default > bundle.yaml

# Deploy kubemod in production mode in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Deploy kubemod in production mode in the configured Kubernetes cluster in ~/.kube/config
undeploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen wire
	$(CONTROLLER_GEN) object:headerFile="misc/boilerplate.go.txt" paths="./..."
	$(WIRE) ./...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	docker image prune -f

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download wire
wire:
ifeq (, $(shell which wire))
	@{ \
	set -e ;\
	WIRE_TMP_DIR=$$(mktemp -d) ;\
	cd $$WIRE_TMP_DIR ;\
	go mod init tmp ;\
	go get github.com/google/wire/cmd/wire@v0.4.0 ;\
	rm -rf $$WIRE_TMP_DIR ;\
	}
WIRE=$(GOBIN)/wire
else
WIRE=$(shell which wire)
endif
