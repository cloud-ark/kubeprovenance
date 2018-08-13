#!/bin/bash

#export GOOS=linux; go build .
#cp kubeprovenance ./artifacts/simple-image/kube-provenance-apiserver
#docker build -t kube-provenance-apiserver:latest ./artifacts/simple-image

#cluster/kubectl.sh should be in your PATH env var
kubectl.sh delete ns provenance
kubectl.sh delete -f artifacts/example/auth-delegator.yaml -n kube-system
kubectl.sh delete -f artifacts/example/auth-reader.yaml -n kube-system
kubectl.sh delete -f artifacts/example/apiservice.yaml
kubectl.sh delete -f artifacts/example/grant-cluster-admin.yaml
#FOR MINIKUBE UNCOMMENT THESE:
#kubectl delete ns provenance
#kubectl delete -f artifacts/example/auth-delegator.yaml -n kube-system
#kubectl delete -f artifacts/example/auth-reader.yaml -n kube-system
#kubectl delete -f artifacts/example/apiservice.yaml
#kubectl delete -f artifacts/example/grant-cluster-admin.yaml

