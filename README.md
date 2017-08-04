# Kubernetes test utilities (aka ktestutil)

## Motivation

CoreOS develops a lot of Kubernetes related projects, including bootkube, installer, operators, check pointers.

These projects needs to be tested (or checked) against the Kubernetes API. For example, bootkube needs to verify the Kubernetes cluster is bootstrapped as expected by checking control panel pod manifests; etcd operators needs to verify the etcd clustered it creates are deployed correctly as Kubernetes pods. Some of the projects need to be tested against a faulty Kubernetes cluster. For example, bootkube wants to ensure the multi-node cluster it creates can tolerate a minor disaster; etcd-operator wants to ensure it can handle pod failures. When test fails, we also want to have a easy way to retrieve all relevant logs. We want to ship the plain logs to a cloud storage, so that we can analysis the failure afterwards.


## Assumptions

We make some assumptions on the environment to make it easier for the testing utilities to interact with Kubernetes clusters and the machines’ OS.

### Environment
  * The cluster is created by bootkube
  * The cluster is running on Container Linux

### Feature Requirements
  * Util
    * Test pod status [Create, Running, Deleted]
    * Test TPR status
  * Log collecting
    * Pods
    * Node components (Kubelet, docker, etc.)
    * Master components (API server, controller manager, scheduler, etcd, etc.)
  * Failure injections
    * Shutdown machine
    * Restart machine
    * Reboot machine
    * Network partition between machines
    * Kill pods
  * Machine utilities
    * SSH
