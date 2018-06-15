# kubeprovenance

A Kubernetes Aggregated API Server that shows dynamic composition information for various Kinds in your cluster.


## What is it?

kubeprovenance is a Kubernetes Aggregated API Server that shows dynamic composition information for various Kinds in your cluster. It builds this dynamic composition provenance by continuously tracking compositions for various Objects of different Kinds that are created in your cluster.


## How does it work?

In Kubernetes certain resources are composed of other resources.
For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods.
Today it is not straight forward to find out entire tree of children resources for a given parent resource.
kubeprovenance API Server solves this problem.

You provide it a YAML file that defines static composition relationship between different Resources/Kinds.
Using this information kubeprovenance API Server builds the dynamic provenance information by querying
the Kubernetes API. The Provenance information is currently stored in memory.

An example YAML file is provided (kind_composition.yaml).
It can contain both in-built Kinds (such as Deployment, Pod, Service), and
Custom Resource Kinds (such as EtcdCluster).

kubeprovenane API server registers REST endpoints for all the kinds that are defined in the YAML file.
These endpoints will be accessed by `kubectl get` command (see below).


## Try it


Scripts are provided to help with building the API server container image and deployment/cleanup.

1) Build the API Server container image:

   `$ ./build-provenance-artifacts.sh`

2) Deploy the kubeprovenance API Server in your cluster:

   `$ ./deploy-provenance-artifacts.sh`

3) Clean-up:

    `$ ./delete-provenance-artifacts.sh`


Once the kubeprovenance API server is running, you can find the dynamic composition information by using following commands:


1) Get dynamic composition for all deployments

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/*/compositions | python -mjson.tool
```

2) Get dynamic composition for a particular deployment

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/<dep-name>/compositions | python -mjson.tool
```

3) Get dynamic composition for all replicasets

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/replicasets/*/compositions | python -mjson.tool
```
