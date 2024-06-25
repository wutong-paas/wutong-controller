#! /bin/bash
# References:
# 1. https://docs.docker.com/buildx/working-with-buildx/
# 2. https://docs.docker.com/engine/reference/commandline/buildx/

# Requirements:
# 1. docker run --privileged --rm tonistiigi/binfmt --install all
# 2. docker login dockerhub
# 3. docker login myhuaweicloud-swr

GOOS=linux GOARCH=amd64 go build -o bin/controller-linux-amd64
GOOS=linux GOARCH=arm64 go build -o bin/controller-linux-arm64

export NAMESPACE=wutong-controller
export VERSION=v1.0.3
docker buildx use swrbuilder || docker buildx create --use --name swrbuilder --driver docker-container --driver-opt image=swr.cn-southwest-2.myhuaweicloud.com/wutong/buildkit:stable
# docker buildx use swrbuilder
# docker buildx build --platform linux/amd64,linux/arm64 --push -t wutongpaas/${NAMESPACE}:${VERSION} -f Dockerfile.local . 
docker buildx build --platform linux/amd64,linux/arm64 --push -t swr.cn-southwest-2.myhuaweicloud.com/wutong/${NAMESPACE}:${VERSION} -f Dockerfile.local . 
# docker buildx rm swrbuilder