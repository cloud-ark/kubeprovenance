# kubeprovenance

A Kubernetes Aggregated API Server to find out Provenance/Lineage information for Kuberentes Custom Resources.

## What is it?


Kubernetes custom resources extend base API to manage third-party platform elements declaratively.
It is important to track chronology of declarative operations performed on custom resources to understand
how these operations affect underlying platform elements - e.g. for an instance of Postgres custom resource we may want to know:
how many db users were created in a month, when was password changed for a db user, etc.
For this, a generic approach is needed to maintain provenance information of custom resources.

kubeprovenance is a tool that helps you find Provenance information about different Kubernetes custom resources in your cluster.

Kubeprovenance is a Kubernetes aggregated API server. It uses Kubernetes audit logs for building custom resource provenance.
Provenance query operators like history, diff, bisect are defined for custom resource instance tracking. Provenance information is accessible via kubectl.


## Try it Out:

**1. Setting Up The Environment.**

Reference: https://dzone.com/articles/easy-step-by-step-local-kubernetes-source-code-cha<br/>
ssh to your VM <br/>
sudo su - <br/>
apt-get install -y gcc make socat git wget<br/>

**2. Install Golang 1.10.3:** <br/>
wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz <br/>
sudo tar -C /usr/local -xzf go1.10.3.linux-amd64.tar.gz <br/>
export PATH=$PATH:/usr/local/go/bin <br/>
export GOROOT=$PATH:/usr/local/go <br/>

Set up your Go workspace, set the GOPATH to it. This is where all your Go code should be. <br/>
mkdir $HOME/goworkspace <br/>
mkdir $HOME/goworkspace/src <br/>
mkdir $HOME/goworkspace/bin <br/>

export GOPATH=$HOME/goworkspace <br/>

**3. Install etcd3.2.18:**
curl -L https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz -o etcd-v3.2.18-linux-amd64.tar.gz && tar xzvf etcd-v3.2.18-linux-amd64.tar.gz && /bin/cp -f etcd-v3.2.18-linux-amd64/{etcd,etcdctl} /usr/bin && rm -rf etcd-v3.2.18-linux-amd64* <br/>


**4. Install Docker**<br/>
Follow steps here: reference: https://docs.docker.com/install/linux/docker-ce/ubuntu/#set-up-the-repository <br/>
docker version //check if it is installed <br/>

**5. Get The Kubernetes Source Code:** <br/>
git clone https://github.com/kubernetes/kubernetes $GOPATH/src/k8s.io/kubernetes <br/>
cd $GOPATH/src/k8s.io/kubernetes <br/>

**6. Compile and Run Kubernetes** <br/>
export KUBERNETES_PROVIDER=local <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# hack/local-up-cluster.sh <br/>

In a new shell, test that it is working : <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# cluster/kubectl.sh cluster-info <br/>
Kubernetes master is running at http://127.0.0.1:8080 # => works! <br/>

Add $GOPATH/src/k8s.io/kubernetes/cluster to PATH: <br/>

export PATH=$PATH:$GOPATH/src/k8s.io/kubernetes/cluster <br/>

Now, commands look like kubectl.sh get pods instead of kubectl get pods...

**7. Enabling Auditing:** <br/>

We have to enable auditing. reference: https://kubernetes.io/docs/tasks/debug-application-cluster/audit/ <br/>
Setting up Log backend (To be added)... <br/>

If not in kubernetes directory... <br/>
cd $GOPATH/src/k8s.io/kubernetes <br/>

vi hack/local-up-cluster.sh <br/>

line 87: Change ENABLE_APISERVER_BASIC_AUDIT to true
   ENABLE_APISERVER_BASIC_AUDIT=${ENABLE_APISERVER_BASIC_AUDIT:-true}

line 486: add audit-policy file to audit_args:   <br/>
   Now you need to add an audit-arg for the audit-policy. add the following line after audit_arg+=" --audit-log-maxbackup=0"

   audit_arg += " --audit-policy-file=/root/audit-policy.yaml" <br/>

   The value of --audit-policy-file is where you created your audit-policy.yaml file.  <br/>
   There is an example-policy for a Postgres custom resource saved in this repository. <br/>

   This file defines what actions and resources will generate logs.

   Reference the docs if you are looking to make one: <br/>
      https://kubernetes.io/docs/tasks/debug-application-cluster/audit/ <br/>
   For running kubeprovenance to track only a Postgres custom resource, audit-policy would look like this:  <br/>
   Note: Add more rules to the audit-policy to track different or more than one custom resource:

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

   Note: The audit log for your custom resource will be saved where this variable is set:
      APISERVER_BASIC_AUDIT_LOG=/tmp/kube-apiserver-audit.log <br/>

**8. Running kubeprovenance** <br/>

Install dep:  <br/>
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh <br/>
Move dep executable to somewhere on your $PATH <br/>
dep version -- to verify that it is installed correctly <br/>

go get github.com/cloud-ark/kubeprovenance <br/>
cd $GOPATH/src/github.com/cloud-ark/kubeprovenance <br/>
dep ensure -v <br/>

Make sure Kubernetes is running:<br/>
$ kubectl.sh cluster-info

Now to deploy this aggregated api server use these commands:
1) Build the API Server container image:  <br/>
   `$ ./build-provenance-artifacts.sh`
2) Deploy the API Server in your cluster: <br/>
   `$ ./deploy-provenance-artifacts.sh`
3) Clean-up:  <br/>
   `$ ./delete-provenance-artifacts.sh`


**9. Deploy Sample Postgres Operator** <br/>

Follow the steps given [here](https://github.com/cloud-ark/kubeplus/tree/master/postgres-crd-v2)

Once the kubeprovenance API server is running, you can find provenance information by using the following commands:

1) Get list of version for a Postgres custom resource instance (client25)

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/versions"
```

2) Get Spec history for Postgres custom resource instance

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/spechistory"
```

3) Get diff of Postgres custom resource instance between version 1 and version 5

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=5"
```

4) Get diff of the field databases for a Postgres custom resource instance between version 1 and version 2

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=2&field=databases"
```

5) Get diff of the field users for a Postgres custom resource instance between version 1 and version 3

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=3&field=users"
```

6) Find out in which version the user 'pallavi' was given password 'pass123'

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field1=username&value1=pallavi&field2=password&value2=pass123"
```

## Try it on Minikube

Note: Since audit-logging is not supported on minikube yet (https://github.com/kubernetes/minikube/issues/2934), I included a static, pre-generated audit-log to use to see how it works.

**1. Setting up environment.** <br/>
sudo su - <br/>
apt-get install -y gcc make socat git wget<br/>
**2. Install Minikube** <br/>
curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.28.2/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/ <br/>
minikube start <br/>
minikube ip -- verify that minikube is up and running <br/>
**3. Install Golang 1.10.3:** <br/>
wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz <br/>
sudo tar -C /usr/local -xzf go1.10.3.linux-amd64.tar.gz <br/>
export PATH=$PATH:/usr/local/go/bin <br/>
export GOROOT=$PATH:/usr/local/go <br/>

Set up your Go workspace, set the GOPATH to it. This is where all your Go code should be. <br/>
mkdir $HOME/goworkspace <br/>
mkdir $HOME/goworkspace/src <br/>
mkdir $HOME/goworkspace/bin <br/>

export GOPATH=$HOME/goworkspace <br/>

**4. Install etcd3.2.18:**
curl -L https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz -o etcd-v3.2.18-linux-amd64.tar.gz && tar xzvf etcd-v3.2.18-linux-amd64.tar.gz && /bin/cp -f etcd-v3.2.18-linux-amd64/{etcd,etcdctl} /usr/bin && rm -rf etcd-v3.2.18-linux-amd64* <br/>


**5. Install Docker**<br/>
Follow steps here: reference: https://docs.docker.com/install/linux/docker-ce/ubuntu/#set-up-the-repository <br/>
docker version //check if it is installed <br/>


**6. Install dep:**<br/>
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh <br/>
Move dep executable to somewhere on your $PATH <br/>
dep version -- to verify that it is installed correctly <br/>


**7. Running kubeprovenance**<br/>

go get github.com/cloud-ark/kubeprovenance <br/>
cd $GOPATH/src/github.com/cloud-ark/kubeprovenance <br/>
dep ensure -v <br/>

0) Allow Minikube to use local Docker images:   <br/>
   `$ eval $(minikube docker-env)`
1) Build the API Server container image:  <br/>
   `$ ./build-provenance-artifacts.sh`
2) Deploy the API Server in your cluster:  <br/>
   `$ ./deploy-provenance-artifacts.sh`
3) Clean-up:  <br/>
   `$ ./delete-provenance-artifacts.sh`

Once the kubeprovenance API server is running, you can find provenance information by using the following commands:


1) Get list of version for a Postgres custom resource instance (client25)

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/versions"
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/versions.png)

2) Get Spec history for Postgres custom resource instance

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/spechistory"
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/spechistory.png)


3) Get diff of Postgres custom resource instance between version 1 and version 5

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=5"
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/getfulldiff.png)


4) Get diff of the field databases for a Postgres custom resource instance between version 1 and version 2

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=2&field=databases"
```
![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/getfielddiff.png)


5) Get diff of the field users for a Postgres custom resource instance between version 1 and version 3

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=3&field=users"
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/usersfielddiff.png)


6) Find out in which version the user 'pallavi' was given password 'pass123'

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field1=username&value1=pallavi&field2=password&value2=pass123"
```
![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/bisect.png)

## Troubleshooting tips:

1) Check that the API server Pod is running:

   `$ kubectl get pods -n provenance`

2) Get the Pod name from output of above command and then check logs of the container.
   For example:

   `$ kubectl logs -n provenance kube-provenance-apiserver-klzpc  -c kube-provenance-apiserver`


### Details:

Our experience in building this API server is [here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).
