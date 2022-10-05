IMAGE ?= strict-supplementalgroups-install

KIND_CLUSTER ?= strict-supplementalgroups-e2e
KIND_CRIO_NODE_IMAGE = kindest/node-crio:v1.24.3
E2E_KUBECONFIG = e2e/kind/.work/kubeconfig
E2E_TEST_IMAGE = bypass-supplementalgroups-in-image

.PHONY: build fmt vet clean e2e e2e-cluster e2e-install e2e-clean

build: fmt vet
	GOOS=linux go build -o ./bin/strict-supplementalgroups-container-runtime ./cmd/strict-supplementalgroups-container-runtime/ 
	GOOS=linux go build -o ./bin/strict-supplementalgroups-install ./cmd/strict-supplementalgroups-install/ 

build-docker-image:
	docker build -t $(IMAGE) .

test:
	go test ./pkg/...

fmt: 
	go fmt ./...

vet:
	go vet $(shell go list ./... | grep -v vendor)

clean:
	rm -rf bin/*


e2e: e2e-cluster e2e-install
	docker build -t $(E2E_TEST_IMAGE) -f e2e/kind/Dockerfile \
		.
	$(call kind-load-image,$(E2E_TEST_IMAGE))
	go test ./e2e -kubeconfig=$(shell readlink -f $(E2E_KUBECONFIG)) \
		-test-image=$(E2E_TEST_IMAGE)

e2e-cluster:
	docker build -t $(KIND_CRIO_NODE_IMAGE) -f e2e/kind/Dockerfile.crio .
	kind create cluster --name=$(KIND_CLUSTER) --config e2e/kind/config.yaml --kubeconfig=$(E2E_KUBECONFIG)

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

e2e-clean:
	kind delete cluster --name=$(KIND_CLUSTER)
	rm  -rf e2e/kind/.work/*

define kind-load-image
	mkdir -p e2e/kind/.work/image
	for n in $$(kubectl --kubeconfig=$(E2E_KUBECONFIG) get node -l cri=containerd -o jsonpath="{range .items[*]}{.metadata.name}{'\t'}{end}"); do \
		kind load docker-image ${1} --name=$(KIND_CLUSTER) --nodes=$${n}; \
	done
	docker save ${1} -o e2e/kind/.work/image/${1}.tar
	for n in $$(kubectl --kubeconfig=$(E2E_KUBECONFIG) get node -l cri=cri-o -o jsonpath="{range .items[*]}{.metadata.name}{'\t'}{end}"); do \
		docker exec $${n} skopeo copy docker-archive:/host/image/${1}.tar containers-storage:docker.io/${1}:latest; \
	done
endef
