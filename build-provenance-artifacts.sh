#!/bin/bash

export GOOS=linux; go build .
cp kubeprovenance ./artifacts/simple-image/kube-provenance-apiserver
docker build -t kube-provenance-apiserver:latest ./artifacts/simple-image
