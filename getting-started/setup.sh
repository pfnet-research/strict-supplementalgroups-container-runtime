#! /usr/bin/env bash
set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
IMAGE=strict-supplementalgroups-container-runtime-install

cd "$DIR/.."
make build-docker-image IMAGE="${IMAGE}"

kind create cluster
kind load docker-image "${IMAGE}"

cd "${DIR}"
kustomize build ../deploy/ | kubectl apply -f -
kubectl -n strict-supplementalgroups-system rollout status ds/strict-supplementalgroups-container-runtime-install

docker build . -t bypass-supplementalgroups-image
kind load docker-image bypass-supplementalgroups-image
