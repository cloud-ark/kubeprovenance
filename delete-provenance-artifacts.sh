#!/bin/bash

#export GOOS=linux; go build .
#cp kubeprovenance ./artifacts/simple-image/kube-provenance-apiserver
#docker build -t kube-provenance-apiserver:latest ./artifacts/simple-image

kubectl delete ns provenance
kubectl delete -f artifacts/example/auth-delegator.yaml -n kube-system
kubectl delete -f artifacts/example/auth-reader.yaml -n kube-system
kubectl delete -f artifacts/example/apiservice.yaml
kubectl delete -f artifacts/example/grant-cluster-admin.yaml

#kubectl create ns wardle
#kubectl create configmap -n wardle kind-compositions-config-map --from-file=kind_compositions.yaml

#kubectl create -f artifacts/example/sa.yaml -n wardle
#kubectl create -f artifacts/example/auth-delegator.yaml -n kube-system
#kubectl create -f artifacts/example/auth-reader.yaml -n kube-system
#kubectl create -f artifacts/example/rc.yaml -n wardle
#kubectl create -f artifacts/example/service.yaml -n wardle
#kubectl create -f artifacts/example/apiservice.yaml