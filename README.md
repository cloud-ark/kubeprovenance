# kubediscovery

A Kubernetes Aggregated API Server that helps with discovery of dynamic information about your cluster.


## What is it?

kubediscovery is a tool that helps you find dynamic information about your cluster. 
Examples of such information include - dynamic composition tree of Kubernetes Objects, current and
past values of config maps, actions that have been performed within your cluster.
Today it is not straightforward to find out this information about your cluster.


kubediscovery is a Kubernetes Aggregated API Server that solves this problem.
The information obtained from kubediscovery can be used to determine how your cluster
has evolved over time (its provenance/lineage). 
Using this information one can answer questions such as:

- How many deployments have occurred on a cluster?

- What actions have been executed on a cluster?

- How configuration values have changed over time?

Current kubediscovery supports showing dynamic composition tree of Kubernetes Objects. 
In Kubernetes there are top-level resources which are composed of other resources. 
For example, a Deployment is composed of a ReplicaSet which in turn is composed of one or more Pods. 
Using kubediscovery you can find out this information easily today.


## How does it work?

You provide it a YAML file that defines static composition relationship between different Resources/Kinds.
Using this information kubediscovery API Server builds the dynamic composition information by 
continuously querying the Kubernetes API for various Objects of different Kinds that are created in your cluster.

The YAML file can contain both in-built Kinds (such as Deployment, Pod, Service), and
Custom Resource Kinds (such as EtcdCluster).
kubeprovenane API server registers REST endpoints for all the kinds that are defined in the YAML file.
These endpoints will be accessed by `kubectl get` command when you want to retrieve the dynamic
composition information (see examples below). An example YAML file is provided (kind_compositions.yaml).
There is also kind_compositions.yaml.with-etcd which shows definition for the EtcdCluster custom resource.
Use this YAML only after you deploy the [Etcd Operator](https://github.com/coreos/etcd-operator)
(Rename this file to kind_compositions.yaml before deploying the API server).

The dynamic composition information is currently collected for the "default" namespace.
It is stored in memory. In the future we will store it in the Etcd instance that we run along with
the API server. We use OwnerReferences to build the dynamic composition tree for Objects.
For querying the main API server, we use direct REST calls instead of typed clients. 
Note that this is the only option that we can use as we want to be able to query for Objects 
based on what is defined in kind_compositions.yaml.
Since we won't know what will be defined in this file in advance, we cannot use typed clients inside
kubediscovery to query the main API server to build the dynamic composition tree.

In building this API server we tried several approaches. You can read about our experience  
[here](https://medium.com/@cloudark/our-journey-in-building-a-kubernetes-aggregated-api-server-29a4f9c1de22).


## Try it on Minikube


Scripts are provided to help with building the API server container image and deployment/cleanup.

0) Allow Minikube to use local Docker images: 

   `$ eval $(minikube docker-env)`


1) Build the API Server container image:

   `$ ./build-provenance-artifacts.sh`

2) Deploy the API Server in your cluster:

   `$ ./deploy-provenance-artifacts.sh`

3) Clean-up:

    `$ ./delete-provenance-artifacts.sh`


Once the kubediscovery API server is running, you can find the dynamic composition information by using following type of commands:


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

The Aggregated API Server has been developed by refering to [sample-apiserver](https://github.com/kubernetes/sample-apiserver)
and [custom-metrics-apiserver](https://github.com/kubernetes-incubator/custom-metrics-apiserver).
