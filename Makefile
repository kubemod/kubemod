# Image URL to use all building/pushing image targets
IMG ?= kubemod/kubemod:latest
# We use Kubernetes 1.16 features. No need to support CRD v1beta1 API.
CRD_OPTIONS ?= "crd:crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: manager

# Run tests
.PHONY: test
test: generate fmt vet manifests
	go test ./core ./util ./jsonpath -coverprofile cover.out

# Run tests -v
.PHONY: testv
testv: generate fmt vet manifests
	go test -v ./core ./util ./jsonpath -coverprofile cover.out

# Run benchmarks
.PHONY: bench
bench: generate fmt vet manifests
	go test ./core ./util -run=XXX -bench=.

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/kubemod main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

.PHONY: bundle
bundle: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default > bundle.yaml

# Deploy kubemod in production mode in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Deploy kubemod in production mode in the configured Kubernetes cluster in ~/.kube/config
.PHONY: undeploy
undeploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate
generate: controller-gen wire mockgen
	$(MOCKGEN) -destination ./mocks/k8s_client_mock.go -package mocks sigs.k8s.io/controller-runtime/pkg/client Client
	$(CONTROLLER_GEN) object:headerFile="misc/boilerplate.go.txt" paths="./..."
	$(WIRE) ./...

# Build the docker image
.PHONY: docker-build
docker-build: test
	docker build . -t ${IMG} --build-arg TARGETARCH=amd64
	docker image prune -f

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

# Develop in docker
.PHONY: docker-develop
docker-develop:
	docker run --rm -it -v $(PWD):/go/src/kubemod -w /go/src/kubemod \
			--entrypoint bash golang:1.18.0

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download wire
.PHONY: wire
wire:
ifeq (, $(shell which wire))
	@{ \
	set -e ;\
	WIRE_TMP_DIR=$$(mktemp -d) ;\
	cd $$WIRE_TMP_DIR ;\
	go mod init tmp ;\
	go install github.com/google/wire/cmd/wire@v0.5.0 ;\
	rm -rf $$WIRE_TMP_DIR ;\
	}
WIRE=$(GOBIN)/wire
else
WIRE=$(shell which wire)
endif

# find or download mockgen
.PHONY: mockgen
mockgen:
ifeq (, $(shell which mockgen))
	@{ \
	set -e ;\
	MOCKGEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$MOCKGEN_TMP_DIR ;\
	go mod init tmp ;\
	go install github.com/golang/mock/mockgen@v1.6.0 ;\
	rm -rf $$MOCKGEN_TMP_DIR ;\
	}
MOCKGEN=$(GOBIN)/mockgen
else
MOCKGEN=$(shell which mockgen)
endif
