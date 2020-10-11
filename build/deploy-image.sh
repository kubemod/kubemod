#!/bin/bash

# Get the version of the image - default to the latest tag.
KUBEMOD_IMAGE_VERSION=${KUBEMOD_IMAGE_VERSION:-$(git describe --tags)}

if [[ $KUBEMOD_IMAGE_VERSION == v* ]]; then
    KUBEMOD_IMAGE_VERSION=$(echo ${OPERATOR_VERSION} | grep -Po "(v[\d\.]+)")
fi

echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin

make docker-build IMG=kubemod/kubemod:$KUBEMOD_IMAGE_VERSION
make docker-push IMG=kubemod/kubemod:$KUBEMOD_IMAGE_VERSION
