IMAGE ?= strict-supplementalgroups-install
VERSION     := $(shell git describe --tags --abbrev=1 --dirty 2>/dev/null)
KIND_CLUSTER ?= strict-supplementalgroups-e2e
KIND_CRIO_NODE_IMAGE = kindest/node-crio:v1.24.3
E2E_WORKDIR = ./e2e/kind/.work
E2E_KUBECONFIG = $(E2E_WORKDIR)/kubeconfig
E2E_TEST_IMAGE = bypass-supplementalgroups-in-image

LDFLAGS := -ldflags="-s -w -X main.Version=$(VERSION:v%=%)"

.PHONY: build build-only
build: fmt lint build-only
build-only:
	GOOS=linux go build $(LDFLAGS) -o ./dist/strict-supplementalgroups-container-runtime ./cmd/strict-supplementalgroups-container-runtime/ 
	GOOS=linux go build $(LDFLAGS) -o ./dist/strict-supplementalgroups-install ./cmd/strict-supplementalgroups-install/ 

.PHONY: test
test:
	go test ./pkg/...

.PHONY: fmt
fmt: 
	$(shell go env GOPATH)/bin/goimports -w ./ pkg/ e2e/
	go fmt ./...

.PHONY: lint
lint:
	$(shell go env GOPATH)/bin/golangci-lint run -v --timeout=3m

.PHONY: clean
clean:
	rm -rf dist/*

.PHONY: build-docker-image
build-docker-image:
	docker build -t $(IMAGE) .

.PHONY: e2e
e2e: e2e-cluster e2e-install
	docker build -t $(E2E_TEST_IMAGE) -f e2e/kind/Dockerfile .
	$(call kind-load-image,$(E2E_TEST_IMAGE))
	go test ./e2e -kubeconfig=$(shell readlink -f $(E2E_KUBECONFIG)) \
		-test-image=$(E2E_TEST_IMAGE)

.PHONY: e2e-cluster
e2e-cluster:
	mkdir -p $(E2E_WORKDIR)
	docker build -t $(KIND_CRIO_NODE_IMAGE) -f e2e/kind/Dockerfile.crio .
	kind create cluster --name=$(KIND_CLUSTER) --config e2e/kind/config.yaml --kubeconfig=$(E2E_KUBECONFIG)

.PHONY: e2e-install
e2e-install:
	docker build -t $(IMAGE) .
	$(call kind-load-image,$(IMAGE))
	kustomize build e2e/deploy | kubectl --kubeconfig=$(E2E_KUBECONFIG) apply -f -
	kubectl --kubeconfig=$(E2E_KUBECONFIG) -n strict-supplementalgroups-system rollout status ds/strict-supplementalgroups-install-containerd
	kubectl --kubeconfig=$(E2E_KUBECONFIG) -n strict-supplementalgroups-system rollout status ds/strict-supplementalgroups-install-crio

	# we have to restart crio manually
	# bcause crio doesn't support reload runtime configuration: https://github.com/cri-o/cri-o/issues/6036
	for n in $$(kubectl --kubeconfig=$(E2E_KUBECONFIG) get node -l cri=cri-o -o jsonpath="{range .items[*]}{.metadata.name}{'\t'}{end}"); do \
		docker exec $${n} systemctl restart crio; \
	done

.PHONY: e2e-clean
e2e-clean:
	kind delete cluster --name=$(KIND_CLUSTER)
	rm  -rf $(E2E_WORKDIR)

define kind-load-image
	mkdir -p $(E2E_WORKDIR)/image
	for n in $$(kubectl --kubeconfig=$(E2E_KUBECONFIG) get node -l cri=containerd -o jsonpath="{range .items[*]}{.metadata.name}{'\t'}{end}"); do \
		kind load docker-image ${1} --name=$(KIND_CLUSTER) --nodes=$${n}; \
	done
	docker save ${1} -o $(E2E_WORKDIR)/image/${1}.tar
	for n in $$(kubectl --kubeconfig=$(E2E_KUBECONFIG) get node -l cri=cri-o -o jsonpath="{range .items[*]}{.metadata.name}{'\t'}{end}"); do \
		docker exec $${n} skopeo copy docker-archive:/host/image/${1}.tar containers-storage:docker.io/${1}:latest; \
	done
endef

.PHONY: ci-setup
ci-setup:
	cd $(shell go env GOPATH) && \
	go install golang.org/x/tools/cmd/goimports@latest && \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.49.0 && \
	curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.16.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind
	curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash && sudo mv ./kustomize /usr/local/bin
	curl -LO "https://storage.googleapis.com/kubernetes-release/release/$$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl" && chmod +x ./kubectl && sudo mv kubectl /usr/local/bin
