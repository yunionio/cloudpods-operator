# onecloud-operator 工作原理

onecloud operator 作为一个长期运行的服务运行在 kubernetes 集群内部，作用是自动搭建和维护 onecloud 所有服务。

原理是 watch [OnecloudCluster](https://github.com/yunionio/onecloud-operator/blob/4c871ae1d3d6774a827834464c480287b7b8b433/pkg/apis/onecloud/v1alpha1/types.go#L76) 自定义资源(CRD) 的创建、更新和删除，然后对应 onecloud 不同服务做不同的操作。

## 举例

### 创建

要创建一个 onecloud 集群，只需要如下的定义:

```yaml
apiVersion: "onecloud.yunion.io/v1alpha1"
kind: OnecloudCluster
metadata:
  name: "example-onecloud-cluster"
  namespace: "onecloud"
spec:
  mysql:
    host: 10.168.222.218
    username: root
    password: "your-sql-passwd"
  region: "region0"
```

然后使用 `kubectl apply` 创建一个 OnecloudCluster 实例，operator 就会收到一个 OnecloudCluster 实例创建的请求，然后开始搭建 onecloud 的 *keystone、region、glance、web前端等* 各个服务

### 更新

创建完一个 OnecloudCluster 集群后，可以使用 `kubectl get onecloudclusters` 或者简写 `kubectl get oc` 获得创建的集群列表。

使用 `kubectl edit oc <name>` 查看集群的定义，就会发现 spec 字段里面默认填充了各个服务的定义，operator 会根据这些服务的定义创建对应 kubernetes Deployment 、Statefulset 或者 Daemonset 等资源。

比如一个已经创建的 OnecloudCluster spec 部分定义如下，完整的定义请看结构定义 [OnecloudClusterSpec](https://github.com/yunionio/onecloud-operator/blob/4c871ae1d3d6774a827834464c480287b7b8b433/pkg/apis/onecloud/v1alpha1/types.go#L97):

```yaml
...
spec:
  # 默认所有服务镜像的上游仓库
  imageRepository: registry.cn-beijing.aliyuncs.com/yunionio
  # 默认所有服务的版本
  version: latest
  # keystone 服务的配置
  keystone:
    # keystone 服务 bootstrap 用户的密码
    bootstrapPassword: ixtPb1Z3_crqS8YG
    # 是否禁用该服务
    disable: false
    # 该服务使用的镜像
    image: registry.cn-beijing.aliyuncs.com/yunionio/keystone:latest
    # 对应 deployment 的副本数
    replicas: 1
    # 对应 deployment 的 tolerations
    tolerations:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
    - effect: NoSchedule
      key: node-role.kubernetes.io/controlplane
  # glance 服务的配置
  glance:
    disable: false
    image: registry.cn-beijing.aliyuncs.com/yunionio/glance:latest
    # 选择调度到哪台 node 上运行
    nodeSelector:
      kubernetes.io/hostname: yunion-hq-gpu01
    replicas: 1
    # 对应 pvc 的用量
    requests:
      storage: 100G
    # pvc 使用 local-path storageClass 的 CSI
    storageClassName: local-path
```

如果修改 OnecloudCluster spec 里面的属性，operator 就会收到集群定义更新的通知，然后做出对应的操作。比如更新了 spec.version 为 v2.11，那么该集群所有的服务都会默认使用 v2.11 tag 的镜像；如果修改 spec.glance.disable 为 true，那么就会停止 glance 服务；修改 spec.keystone.image 就会使用对应修改的镜像地址。

### 删除

当 OnecloudCluster 实例被删除后，operator 会收到集群删除的通知，删除对应所有服务占用的自用，比如自动删除对应服务的: Deployment, Statefulset 等。

## 开发相关

### 环境搭建

目前推荐的方式是通过开源文档的 [部署集群](https://docs.yunion.io/docs/setup/controlplane/) 搭建环境，搭建好后在 onecloud namespace 里面就会有默认的 onecloud-operator。

```bash
# 找到 onecloud-operator deployment
$ kubectl get deployments -n onecloud | grep operator

# 找到 operator 对应的容器
$ kubectl get pods -n onecloud | grep operator
```

### 部署

部署的流程是在本地编译并打包成 docker image，然后上传到指定的 registry，然后更新 operator deployment 的 spec.image 属性，对应的 pod 就会重建。如果 image 是同一个 tag，直接删除对应的 operator pod 就会拉取新的镜像。

```bash
# 编译并上传到 registry，以下命令会把编译好的 operator 上传到 zexi/onecloud-operator:dev
$ REGISTRY=zexi VERSION=dev make image-push

# 更新 deployment
$ kubectl set image -n onecloud deployment/onecloud-operator onecloud-operator=zexi/onecloud-operator:dev

# 如果每次 make 都是同一个镜像名，push 完镜像后，直接删除对应的 operator 就会自动拉取新的景象了
$ kubectl get pods -n onecloud | grep operator
$ kubectl delete pods -n onecloud <pod_name>
```

### 查看日志

查看日志直接用 `kubectl logs` 就行

```bash
kubectl logs -f <pod_name> --since <time_format>
```

### 代码简介

以下做一些关键代码的说明。

**cmd/controller-manager/main.go**: operator 服务的入口，主要是连接内部 k8s 集群，创建 OnecloudCluster CRD 资源，运行 OnecloudCluster Controller。

**pkg/controller/cluster/onecloud_cluster_controller.go**: 里面的 NewController 函数会创建一个整体资源的 Controller 对象，初始化 k8s deployment, configmap, pvc, ingress 等各个资源的 informer，用于操作这些资源。同步资源的入口是 Controller.syncCluster 函数，该函数会调用 **pkg/controller/cluster/onecloud_cluster_control.go** 里面的 defaultClusterControl 的 UpdateOnecloudCluster 函数，里面有具体针对 OnecloudCluster 集群资源的同步操作。

**pkg/controller/cluster/onecloud_cluster_control.go**：关键的函数是 defaultClusterControl.updateOnecloudCluster，该函数会在一个 for 循环中调用各个服务组件的 Sync 方法，创建对应的资源，部分代码如下：

```go
...
for _, component := range []manager.Manager{
    // keystone 组件
    components.Keystone(),
    // logger 组件
    components.Logger(),
    // climc 组件
    components.Climc(),
    // influxdb 组件
    components.Influxdb(),
    // region 组件
    components.Region(),
    components.Scheduler(),
    components.Glance(),
    components.Webconsole(),
    components.Yunionagent(),
    components.Yunionconf(),
    components.KubeServer(),
    components.APIGateway(),
    components.Web(),
    components.Notify(),
} {
    if err := component.Sync(oc); err != nil {
        return err
    }
}
```

各个 component 的业务操作代码在 **pkg/manager/component/** 目录里面:

```bash
$ tree pkg/manager/component
pkg/manager/component
├── apigateway.go
├── climc.go
├── component.go
├── configer.go
├── glance.go
├── influxdb.go
├── keystone.go
├── kubeserver.go
├── logger.go
├── notify.go
├── regiondns.go
├── region.go
├── scheduler.go
├── sync.go
├── utils.go
├── webconsole.go
├── web.go
├── yunionagent.go
└── yunionconf.go
```

以 keystone 服务为例，对应的 **pkg/manager/component/keystone.go** 里面有定义 keystoneManager，该结构需要实现 Sync 方法和 cloudComponentFactory 接口，然后由上层的 updateOnecloudCluster 函数调用。默认情况已经编写了一个 syncComponent 的函数，该函数会依次创建一个服务需要的 k8s 资源。比如 keystoneManager 绑定了以下的函数：

```go
// 该函数表示 keystone 服务需要创建数据库
func (m *keystoneManager) getDBConfig(oc *v1alpha1.OnecloudClusterConfig) *v1alpha1.DBConfig

// 该函数表示 keystone 服务需要做的额外配置
func (m *keystoneManager) getPhaseControl(man controller.ComponentManager) controller.PhaseControl

// 表示 keystone 服务需要创建或更新 k8s service
func (m *keystoneManager) getService(oc *v1alpha1.OnecloudCluster) *corev1.Service

// 表示 keystone 服务需要创建或更新 k8s configmap，一般是生成服务的配置
func (m *keystoneManager) getConfigMap(oc *v1alpha1.OnecloudCluster, clusterCfg *v1alpha1.OnecloudClusterConfig) (*corev1.ConfigMap, error)

// 表示 keystone 服务需创建或更新 k8s deployment
func (m *keystoneManager) getDeployment(oc *v1alpha1.OnecloudCluster, _ *v1alpha1.OnecloudClusterConfig) (*apps.Deployment, error)
```

这些 get 函数的作用是生成对应的 k8s 资源或者配置，外层的 sync 机制会周期性的调用这些方法拿到返回值，如果对应资源不存在就创建，存在则根据差异更新。
还有一些其他的函数像 getPVC，getIngress，机制都和 keystone 上面的 get 函数一样，如果该服务需要这些 k8s 资源，则实现对应的函数即可。

### 添加新服务

添加新服务的大概流程如下：

1. 到 **pkg/apis/onecloud/v1alpha1/types.go** 里面的 OnecloudClusterSpec 结构体添加对应服务的 Spec 定义。

2. 添加完服务的 Spec 定义后需要到 **pkg/apis/onecloud/v1alpha1/defaults.go** 里面注册服务配置的默认值生成函数，需要这一步的作用是自动生成数据库密码，镜像信息等。

3. 调用 **hack/codegen.sh** 脚本自动生成更新代码。

4. 到 **pkg/manager/component/** 目录编写服务相关服务依赖资源和配置的代码，然后像 **pkg/manager/component/component.go** 里面的 ComponentManager 绑定 `func (m *ComponentManager) YourService() manager.Manager { return newYourServiceManager(m) }` 方法。

5. 到 **pkg/controller/cluster/onecloud_cluster_control.go** 里面的 updateOnecloudCluster 函数里面添加 **components.YourService()** 方法。
