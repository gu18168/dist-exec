# dist-exec

An Operator ensures that when a `distexec` custom resource is created, all Nodes will execute the command specified by the `distexec`. The result of the execution will be recorded in the status of the `distexec` resource.

The Operator makes it easy to implement distributed command execution on a per-Node basis.

## Description

The Operator is deployed by DaemonSet and ensures that the command execution environment is available on every Node.

Operator listens to the `distexec.exec.yuhong.test` custom resource. When the resource is created, the Operators execute the commands specified by the resource in their respective nodes. The result of the execution is stored in the status of the resource.

For example, the `distexec-sample` resource wants to execute the `ls` command on each node. The corresponding YAML configuration is as follows:

```yaml
apiVersion: exec.yuhong.test/v1
kind: DistExec
metadata:
  name: distexec-sample
spec:
  command: ls
```

Suppose our Kubernetes cluster has two nodes, `node1` and `node2`. When `distexec-sample` is created, Operator will execute commands in both nodes. The final status of the `distexec-sample` resource looks like this:

```yaml
Status:
  results:
    node1: a.txt
    node2: b.txt
```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

1. Install CRD and run Operator (Based on helm)

```sh
make deploy
```

2. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/exec_v1_distexec.yaml
```

### Uninstall 
To delete the CRDs and Operator from the cluster:

```sh
make undeploy 
```

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

