# kube-extended-resource

## scheduler-extender

`scheduler-extender` 通过扩展的方式实现了自定义调度器，从而避免直接修改 `Kubernetes Scheduler` 组件源码。此扩展，实际上是默认调度器提供了一个称作 `extender` 的方式，`extender` 是一个 `HTTP Server`，在 `extender` 中我们可以实现自定义的 `priority`, `filter`, `bind` 方法。在此项目中，仅实现了 `filter` 和 `bind` 方法。

如果使用 `extender` 的方式扩展默认调度器，则需要修改默认调度器的配置，参考：[Schduler extensibility](https://github.com/kubernetes/community/blob/master/contributors/devel/scheduler.md) ，这样导致所有的 `Pod` 都会经过这个扩展，但是实际使用过程中，我们可能只是希望部分应用的 `Pod` 使用这个 `extender` 进行调度，而且此方式的弊端也很明显，因为 `extender` 是以 `HTTP` 的方式实现，如果 `Kubernetes` 集群比较大，那么 `extender` 可能成为一个瓶颈。为此，我们通过多调度器的方式来避免此问题，多调度器参考：[Configure multiple scheduler](https://kubernetes.io/docs/tasks/administer-cluster/configure-multiple-schedulers/)，为指定应用指定调度器名称，其它应用还是按照默认调度器的方式调度。

### 使用

通过以下命令将项目打包成镜像：

```shell
$ make docker-build
```

在集群中安装，以实现多调度器：

```shell
$ kubectl create -f deploy/extender.yaml
```

当然，多调度器还需要修改 `system:kube-scheduler` ClusterRole:

```shell
$ kubectl edit clusterrole system:kube-scheduler
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    annotations:
      rbac.authorization.kubernetes.io/autoupdate: "true"
    labels:
      kubernetes.io/bootstrapping: rbac-defaults
    name: system:kube-scheduler
  rules:
  - apiGroups:
    - ""
    resourceNames:
    - kube-scheduler
    - custom scheduler name # 此处是自定义调度器的名称，例如在此项目中是 `extended-resource-scheduler`
    resources:
    - endpoints
    verbs:
    - delete
    - get
    - patch
    - update
```

部署应用时，具体参考：`config/nginx.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gpu-resource
  namespace: default
spec:
  selector:
    matchLabels:
      app: gpu
  replicas: 1
  template:
    metadata:
      labels:
        app: gpu
    spec:
      schedulerName: extended-resource-scheduler # 指定调度器名称
      containers:
      - name: nginx
        image: "nginx:1.14.2"
        extendedResourceClaims:
        - "k80-erc"
```

## extended-resource-controller

`extended-resource-controller` 是为了维护集群中 `ExtendedResourceClaim` 和 `ExtendedResource` 的状态。

当应用删除后，与之绑定的 `ExtendedResourceClaim`  更新成可用状态，以及对应的 `ExtendedResource` 也跟随着更新成可用状态。

## 使用

通过以下命令将项目打包成镜像：

```shell
$ make docker-build
```

在集群中安装部署:

```shell
$ kubectl create -f deploy/deployment.yaml
```
