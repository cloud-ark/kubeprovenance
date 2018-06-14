Steps:
-------
export GOOS=linux; go build provenance.go

docker build -t getprovenance:1 -f Dockerfile.provenance .

kubectl create configmap kind-compositions-config-map --from-file=kind_compositions.yaml

kubectl apply -f provenance-deployment.yaml

kubectl get pods

kubectl logs "provenance-deployment-pod"