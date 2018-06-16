# kubeprovenance

A Kubernetes Aggregated API Server that shows dynamic composition information for various Kinds in your cluster.


## What is it?

kubeprovenance is a Kubernetes Aggregated API Server that shows dynamic composition information for various Kinds in your cluster. In Kubernetes certain resources are composed of other resources.
For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods.
Today it is not straightforward to find out entire tree of children resources for a given parent resource.
kubeprovenance API Server solves this problem.


## How does it work?

You provide it a YAML file that defines static composition relationship between different Resources/Kinds.
Using this information kubeprovenance API Server builds the dynamic provenance information by 
continuously querying the Kubernetes API for various Objects of different Kinds that are created in your cluster.

The YAML file can contain both in-built Kinds (such as Deployment, Pod, Service), and
Custom Resource Kinds (such as EtcdCluster).
kubeprovenane API server registers REST endpoints for all the kinds that are defined in the YAML file.
These endpoints will be accessed by `kubectl get` command when you want to retrieve the dynamic
composition information (see below). An example YAML file is provided (kind_compositions.yaml).
There is also kind_compositions.yaml.with-etcd which shows definition for the EtcdCluster custom resource.
Use this YAML only after you deploy the [Etcd Operator](https://github.com/coreos/etcd-operator)
(Rename this file to kind_compositions.yaml before deploying the API server).


The Provenance information is currently collected for the "default" namespace.
The information is stored in memory.


You can read about various approaches that we tried when building this server [here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22)


## Try it on Minikube


Scripts are provided to help with building the API server container image and deployment/cleanup.

0) Allow Minikube to use local Docker images: 

   `$ eval $(minikube docker-env)`


1) Build the API Server container image:

   `$ ./build-provenance-artifacts.sh`

2) Deploy the kubeprovenance API Server in your cluster:

   `$ ./deploy-provenance-artifacts.sh`

3) Clean-up:

    `$ ./delete-provenance-artifacts.sh`


Once the kubeprovenance API server is running, you can find the dynamic composition information by using following type of commands:


1) Get dynamic composition for all deployments

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/*/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/deployments.png)


2) Get dynamic composition for a particular deployment

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/<dep-name>/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/hello-minikube-deployment.png)


3) Get dynamic composition of all etcdclusters custom resource

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/etcdclusters/*/compositions | python -mjson.tool
```

![alt text](https://github.com/cloud-ark/kubeprovenance/raw/master/docs/etcd-clusters.png)

You can use above style of commands with all the Kinds that you have defined in kind_compositions.yaml


## Troubleshooting tips:

1) Check that the API server Pod is running: 

   `$ kubectl get pods -n provenance`

2) Get the Pod name from output of above command and then check logs of the container.
   For example:

   `$ kubectl logs -n provenance kube-provenance-apiserver-klzpc  -c kube-provenance-apiserver`


### References:

The Aggregated API Server has been developed by refering to the [sample-apiserver](https://github.com/kubernetes/sample-apiserver)
