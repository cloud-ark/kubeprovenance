# kubeprovenance

A Kubernetes Aggregated API Server to find out Provenance information for different Kuberentes Kinds.


## What is it?

kubeprovenance is a tool that helps you find Provenance information about different Kubernetes Kinds
in your cluster. 
The information obtained from kubeprovenance can be used to determine how your cluster
has evolved over time (its provenance/lineage). 
Using this information one can answer questions such as:

- How many deployments have occurred on a cluster?

- What actions have been executed on a cluster?

- How configuration values have changed over time?


## How does it work?

kubeprovenance uses Kubernetes Audit logs to build the provenance information.

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


Once the kubediscovery API server is running, you can find provenance information by using following type of commands:

1) Get list of versions for testobj deployment

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/testobj/versions
```

2) Get Spec history for testobj deployment

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/testobj/spechistory
```

3) Get diff of Spec for testobj deployment between version v1 and version v2

```
kubectl get --raw /apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/testobj/diff?start=v1&end=v2
```

4) Get diff of field abc for testobj deployment between version v1 and version v2

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/testobj/diff?start=v1&end=v2&field=abc"
```

5) Find out which version field abc for testobj deployment was given value def

```
kubectl get --raw "/apis/kubeprovenance.cloudark.io/v1/namespaces/default/deployments/testobj/bisect?field=abc&value=def"
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
