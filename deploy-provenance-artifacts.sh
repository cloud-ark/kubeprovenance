#!/bin/bash

kubectl.sh create -f artifacts/example/ns.yaml
kubectl.sh create configmap -n provenance kind-compositions-config-map --from-file=kind_compositions.yaml
kubectl.sh create -f artifacts/example/sa.yaml -n provenance
kubectl.sh create -f artifacts/example/auth-delegator.yaml -n kube-system
kubectl.sh create -f artifacts/example/auth-reader.yaml -n kube-system
kubectl.sh create -f artifacts/example/grant-cluster-admin.yaml
kubectl.sh create -f artifacts/example/rc.yaml -n provenance
kubectl.sh create -f artifacts/example/service.yaml -n provenance
kubectl.sh create -f artifacts/example/apiservice.yaml
#FOR MINIKUBE UNCOMMENT THESE: todo: code to automate this
#kubectl create -f artifacts/example/ns.yaml
#kubectl create configmap -n provenance kind-compositions-config-map --from-file=kind_compositions.yaml
#kubectl create -f artifacts/example/sa.yaml -n provenance
#kubectl create -f artifacts/example/auth-delegator.yaml -n kube-system
#kubectl create -f artifacts/example/auth-reader.yaml -n kube-system
#kubectl create -f artifacts/example/grant-cluster-admin.yaml
#kubectl create -f artifacts/example/rc.yaml -n provenance
#kubectl create -f artifacts/example/service.yaml -n provenance
#kubectl create -f artifacts/example/apiservice.yaml

