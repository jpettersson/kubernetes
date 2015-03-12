#!/bin/bash

docker build -t kube-builder .

# Copy files form build environment image back to host
CONTAINER_ID=$(docker run -d kube-builder)
docker cp ${CONTAINER_ID}:/gopath/src/github.com/GoogleCloudPlatform/kubernetes/cluster/addons/kube-dynamic-router/kube-dynamic-router kube-dynamic-router
docker kill ${CONTAINER_ID}
docker rm ${CONTAINER_ID}
