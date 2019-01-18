# Image URL to use all building/pushing image targets
IMG ?= quay.io/postmates/configurable-hpa

# To perform tests we need a lot of additional packages the image, including kubebuilder
BUILD_TAG ?= test-v2

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/postmates/configurable-hpa/cmd/manager

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

# Build the docker image, should be done only for test
# For production, the image is built in the DroneCI
#   (check .drone.yml)
docker-build: test
	docker build . -t ${IMG}:stable

# Build the docker image for test
docker-build-test-image:
	docker build . --squash -f Dockerfile.test -t ${IMG}:${BUILD_TAG}

docker-push-test-image:
	docker push ${IMG}:${BUILD_TAG}

e2e:
	cd tests; python -m unittest *.py
