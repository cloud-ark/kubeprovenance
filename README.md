# kubeprovenance

A Kubernetes Aggregated API Server to find out Provenance information for different Kuberentes Custom Resources.

## What is it?

kubeprovenance is a tool that helps you find Provenance information about different Kubernetes custom resources
in your cluster. <br/>
The information obtained from kubeprovenance can be used to determine how your cluster <br/>
has evolved over time (its provenance/lineage). <br/>

## How does it work?

kubeprovenance uses Kubernetes Auditing to build the provenance information.

In building this API server we tried several approaches. You can read about our experience  
[here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).

## Try it Out: 
**Steps to Run Kubernetes Local Cluster on a GCE or AWS instance (or any node), configure auditing and running/testing Kubeprovenance aggregated api server** <br/> 

**1. Setting up environment. reference: https://dzone.com/articles/easy-step-by-step-local-kubernetes-source-code-cha** <br/>
ssh to your VM <br/>
sudo su - <br/>
apt-get install -y gcc make socat git<br/>

**2. Install Golang 1.10.3:** <br/>
wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.10.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin <br/>

**3. Install etcd3.2.18:**
curl -L https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz -o etcd-v3.2.18-linux-amd64.tar.gz && tar xzvf etcd-v3.2.18-linux-amd64.tar.gz && /bin/cp -f etcd-v3.2.18-linux-amd64/{etcd,etcdctl} /usr/bin && rm -rf etcd-v3.2.18-linux-amd64* <br/>
**4. Install Docker**<br/>
sudo apt-get update <br/>
sudo apt-get install docker-ce <br/>

set up your go workspace, set the GOPATH to it. this is where all your go code should be. <br/>
export GOPATH=/gopath <br/>

**5. Get The Kubernetes Source Code:** <br/>
git clone https://github.com/kubernetes/kubernetes $GOPATH/src/k8s.io/kubernetes <br/>
cd $GOPATH/src/k8s.io/kubernetes <br/>

**6. Compile and run kubernetes** <br/>
export KUBERNETES_PROVIDER=local <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# hack/local-up-cluster.sh <br/>

In a new shell, test that it is working : <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# cluster/kubectl.sh cluster-info <br/>
Kubernetes master is running at http://127.0.0.1:8080 # => works! <br/>

Add $GOPATH/src/k8s.io/kubernetes/cluster to PATH. <br/>

export PATH=$PATH:$GOPATH/src/k8s.io/kubernetes/cluster <br/>
Commands look like kubectl.sh get pods instead of kubectl get pods... 

**7. Enabling auditing:**
We have to enable auditing. reference: https://kubernetes.io/docs/tasks/debug-application-cluster/audit/ <br/>
Setting up Log backend ... <br/>

If not in kubernetes directory... <br/>
cd $GOPATH/src/k8s.io/kubernetes <br/>

vi hack/local-up-cluster.sh <br/>

line 87: Change ENABLE_APISERVER_BASIC_AUDIT to true
   ENABLE_APISERVER_BASIC_AUDIT=${ENABLE_APISERVER_BASIC_AUDIT:-true}

line 486: add audit-policy file to audit_args:   
   Now you need to add an audit-arg for the audit-policy. add the following line after audit_arg+=" --audit-log-maxbackup=0" 
   
   audit_arg += " --audit-policy-file=/root/audit-policy.yaml" <br/>
         
   The value of --audit-policy-file is where you created your audit-policy.yaml file.  <br/>
   There is an example-policy for a postgres custom resource saved in this repository. <br/>
      
   Note: the audit log for your custom resource will be saved where this variable is set:
      APISERVER_BASIC_AUDIT_LOG=/tmp/kube-apiserver-audit.log
   
   
   This file defines what actions and resources will generate logs.
   
   An example of a audit-policy file: reference the docs if you are looking to make one: <br/>
      https://kubernetes.io/docs/tasks/debug-application-cluster/audit/
      
   For running kubeprovenance to track only a postgres custom resource, audit-policy would look like this:  <br/>
   Add more rules to the audit-policy to track different or more than one custom resource:
   
      root@provenance:~# more audit-policy.yaml 
      apiVersion: audit.k8s.io/v1beta1
      kind: Policy
      omitStages:
        - "RequestReceived"
      rules:
        - level: Request
          verbs:
            - create
            - delete
            - patch
          resources:
            - group: "postgrescontroller.kubeplus"
              version: "v1"
              resources: ["postgreses"]
    
   Note: our approach may change to a webhook backend instead of a log backend <br/>
   

**8. Running kubeprovenance** <br/>
   
Install dep:  <br/>
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh <br/>

git clone https://github.com/cloud-ark/kubeprovenance.git <br/>
mv kubeprovenance $GOPATH/src/github.com/cloud-ark <br/>
cd $GOPATH/src/github.com/cloud-ark/kubeprovenance <br/>
dep ensure <br/>

Make sure kubernetes is running:
$ kubectl.sh cluster-info

Now to deploy this aggregated api server use these commands:
1) Build the API Server container image:  <br/>
   `$ ./build-provenance-artifacts.sh`
2) Deploy the API Server in your cluster: <br/>
   `$ ./deploy-provenance-artifacts.sh`
3) Clean-up:  <br/>
   `$ ./delete-provenance-artifacts.sh`

Test using these following commands:   
1) Get list of versions for client25 postgres

```
kubectl.sh get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/versions
```

2) Get Spec history for client25 postgres

```
kubectl.sh get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/spechistory
```

3) Get diff of Spec for client25 postgres between version v1 and version v2

```
kubectl.sh get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=v1&end=v2
```

4) Get diff of field abc for client25 postgres between version v1 and version v2

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=v1&end=v2&field=abc"
```

5) Find out which version field abc for client25 postgres was given value def

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field=abc&value=def"
```
![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/spechistory.png)


## Try it on Minikube


Scripts are provided to help with building the API server container image and deployment/cleanup.

0) Allow Minikube to use local Docker images:   <br/>
   `$ eval $(minikube docker-env)`
1) Build the API Server container image:  <br/>
   `$ ./build-provenance-artifacts.sh`
2) Deploy the API Server in your cluster:  <br/>
   `$ ./deploy-provenance-artifacts.sh`
3) Clean-up:  <br/>
   `$ ./delete-provenance-artifacts.sh`

Once the kubediscovery API server is running, you can find provenance information by using following type of commands: 

1) Get list of versions for client25 postgres

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/versions
```

2) Get Spec history for client25 postgres

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/spechistory
```

3) Get diff of Spec for client25 postgres between version v1 and version v2

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=v1&end=v2
```

4) Get diff of field abc for client25 postgres between version v1 and version v2

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=v1&end=v2&field=abc"
```

5) Find out which version field abc for client25 postgres was given value def

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field=abc&value=def"
```

## Troubleshooting tips:

1) Check that the API server Pod is running: 

   `$ kubectl get pods -n provenance`

2) Get the Pod name from output of above command and then check logs of the container.
   For example:

   `$ kubectl logs -n provenance kube-provenance-apiserver-klzpc  -c kube-provenance-apiserver`


### References:

The Aggregated API Server has been developed by refering to [sample-apiserver](https://github.com/kubernetes/sample-apiserver)
and [custom-metrics-apiserver](https://github.com/kubernetes-incubator/custom-metrics-apiserver).

