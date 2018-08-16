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

## How does it work?

kubeprovenance uses Kubernetes Auditing to build the provenance information.

In building this API server we tried several approaches. You can read about our experience  
[here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).

## Status

Work in Progress.

Note that currently kubeprovenance uses kube-apiserver-audit.log file included in artifacts/simple-image folder
to build provenance information. So when you try out kubeprovenance you will get provenance information that is build from this file.
We are working on changing kubeprovenance's information source from static audit log file to live audit logs that are continuously collected in the cluster.

## Try it Out:
Steps to Run Kubernetes Local Cluster on a GCE or AWS instance (or any node), configure auditing and running/testing Kubeprovenance aggregated api server

**1. Setting up environment. reference: https://dzone.com/articles/easy-step-by-step-local-kubernetes-source-code-cha** <br/>
ssh to your VM <br/>
sudo su - <br/>
apt-get install -y gcc make socat git<br/>

**2. Install Golang 1.10.3:** <br/>
wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz <br/>
sudo tar -C /usr/local -xzf go1.10.3.linux-amd64.tar.gz <br/>
export PATH=$PATH:/usr/local/go/bin <br/>

**3. Install etcd3.2.18:**
curl -L https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz -o etcd-v3.2.18-linux-amd64.tar.gz && tar xzvf etcd-v3.2.18-linux-amd64.tar.gz && /bin/cp -f etcd-v3.2.18-linux-amd64/{etcd,etcdctl} /usr/bin && rm -rf etcd-v3.2.18-linux-amd64* <br/>
**4. Install Docker**<br/>
Follow steps here: reference: https://docs.docker.com/install/linux/docker-ce/ubuntu/#set-up-the-repository <br/>
docker version //check if it is installed <br/>

set up your go workspace, set the GOPATH to it. this is where all your go code should be. <br/>
mkdir $HOME/goworkspace <br/>
export GOPATH=$HOME/goworkspace <br/>

**5. Get The Kubernetes Source Code:** <br/>
git clone https://github.com/kubernetes/kubernetes $GOPATH/src/k8s.io/kubernetes <br/>
cd $GOPATH/src/k8s.io/kubernetes <br/>

**6. Compile and run kubernetes** <br/>
export KUBERNETES_PROVIDER=local <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# hack/local-up-cluster.sh <br/>

In a new shell, test that it is working : <br/>
root@host: $GOPATH/src/k8s.io/kubernetes# cluster/kubectl.sh cluster-info <br/>
Kubernetes master is running at http://127.0.0.1:8080 # => works! <br/>

Add $GOPATH/src/k8s.io/kubernetes/cluster to PATH: <br/>

export PATH=$PATH:$GOPATH/src/k8s.io/kubernetes/cluster <br/>
Now, Commands look like kubectl.sh get pods instead of kubectl get pods...

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

   reference the docs if you are looking to make one: <br/>
      https://kubernetes.io/docs/tasks/debug-application-cluster/audit/ <br/>
   For running kubeprovenance to track only a postgres custom resource, audit-policy would look like this:  <br/>
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

   Note: our approach may change to a webhook backend instead of a log backend <br/>


**8. Running kubeprovenance** <br/>

Install dep:  <br/>
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh <br/>
cp $GOPATH/bin/dep /usr/bin/dep <br/>

git clone https://github.com/cloud-ark/kubeprovenance.git $GOPATH/src/github.com/cloud-ark<br/>
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

4) Get diff of the field databases for a Postgres custom resource instance between version 1 and version 5

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=2&field=databases"
```

5) Get diff of the field username for a Postgres custom resource instance between version 1 and version 3

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=3&field=username"
```

6) Find out in which version the user 'pallavi' was given password 'pass123'

```
kubectl.sh get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field1=username&value1=pallavi&field2=password&value2=pass123"
```


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


4) Get diff of the field databases for a Postgres custom resource instance between version 1 and version 5

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=2&field=databases"
```
![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/getfielddiff.png)


5) Get diff of the field username for a Postgres custom resource instance between version 1 and version 3

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/diff?start=1&end=3&field=username"
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/usersfielddiff.png)


6) Find out in which version the user 'pallavi' was given password 'pass123'

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/postgreses/client25/bisect?field1=username&value1=pallavi&field2=password&value2=pass123"
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
