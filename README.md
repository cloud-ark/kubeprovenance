Steps:
-------

To Run the kubeprovenance server
---------------------------------

Following steps are taken from the sample-apiserver README (https://github.com/kubernetes/sample-apiserver)

a) Build the Provenance Server binary:

unset GOOS

dep init (or dep ensure)

go build

go install

b) Generate certificates

openssl req -nodes -new -x509 -keyout ca.key -out ca.crt

openssl req -out client.csr -new -newkey rsa:4096 -nodes -keyout client.key -subj "/CN=development/O=system:masters"

openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out client.crt

openssl pkcs12 -export -in ./client.crt -inkey ./client.key -out client.p12 -passout pass:password

c) Install etcd

d) Start etcd

etcd &

e) Run kubeprovenance server

kubeprovenance --secure-port 8443 --etcd-servers http://127.0.0.1:2379 --v=7 \
	--client-ca-file ca.crt \
	--kubeconfig ~/.kube/config \
	--authentication-kubeconfig ~/.kube/config \
	--authorization-kubeconfig ~/.kube/config

f) Verify (Steps for Mac)

brew install httpie

http --verify=no --cert client.crt --cert-key client.key https://localhost:8443/apis/kubeprovenance.cloudark.io/v1/compositions/Deployments/abcasjdkjsljs

g) Verify

curl -fv -k --cert client.p12:password \
   https://localhost:8443/apis/kubeprovenance.cloudark.io/v1/compositions/Deployments/abcasjdkjsljs


To Deploy the Provenance container:
------------------------------------

cd pkg/provenance

export GOOS=linux; go build provenance.go

cd ../../

docker build -t getprovenance:1 -f Dockerfile.provenance .

kubectl create configmap kind-compositions-config-map --from-file=kind_compositions.yaml

kubectl apply -f provenance-deployment.yaml

kubectl get pods

kubectl logs "provenance-deployment-pod"








