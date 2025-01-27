# API Reference

## Packages
- [sailoperator.io/v1alpha1](#sailoperatoriov1alpha1)


## sailoperator.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the sailoperator.io v1alpha1 API group

### Resource Types
- [ZTunnel](#ztunnel)
- [ZTunnelList](#ztunnellist)



#### ZTunnel



ZTunnel represents a deployment of the Istio ztunnel component.



_Appears in:_
- [ZTunnelList](#ztunnellist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sailoperator.io/v1alpha1` | | |
| `kind` _string_ | `ZTunnel` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ZTunnelSpec](#ztunnelspec)_ |  | \{ namespace:ztunnel profile:ambient version:v1.24.2 \} |  |
| `status` _[ZTunnelStatus](#ztunnelstatus)_ |  |  |  |


#### ZTunnelCondition



ZTunnelCondition represents a specific observation of the ZTunnel object's state.



_Appears in:_
- [ZTunnelStatus](#ztunnelstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ZTunnelConditionType](#ztunnelconditiontype)_ | The type of this condition. |  |  |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#conditionstatus-v1-meta)_ | The status of this condition. Can be True, False or Unknown. |  |  |
| `reason` _[ZTunnelConditionReason](#ztunnelconditionreason)_ | Unique, single-word, CamelCase reason for the condition's last transition. |  |  |
| `message` _string_ | Human-readable message indicating details about the last transition. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |


#### ZTunnelConditionReason

_Underlying type:_ _string_

ZTunnelConditionReason represents a short message indicating how the condition came
to be in its present state.



_Appears in:_
- [ZTunnelCondition](#ztunnelcondition)
- [ZTunnelStatus](#ztunnelstatus)

| Field | Description |
| --- | --- |
| `ReconcileError` | ZTunnelReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.  |
| `DaemonSetNotReady` | ZTunnelDaemonSetNotReady indicates that the ztunnel DaemonSet is not ready.  |
| `ReadinessCheckFailed` | ZTunnelReasonReadinessCheckFailed indicates that the DaemonSet readiness status could not be ascertained.  |
| `Healthy` | ZTunnelReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.  |


#### ZTunnelConditionType

_Underlying type:_ _string_

ZTunnelConditionType represents the type of the condition.  Condition stages are:
Installed, Reconciled, Ready



_Appears in:_
- [ZTunnelCondition](#ztunnelcondition)

| Field | Description |
| --- | --- |
| `Reconciled` | ZTunnelConditionReconciled signifies whether the controller has successfully reconciled the resources defined through the CR.  |
| `Ready` | ZTunnelConditionReady signifies whether the ztunnel DaemonSet is ready.  |


#### ZTunnelList



ZTunnelList contains a list of ZTunnel





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `sailoperator.io/v1alpha1` | | |
| `kind` _string_ | `ZTunnelList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ZTunnel](#ztunnel) array_ |  |  |  |


#### ZTunnelSpec



ZTunnelSpec defines the desired state of ZTunnel



_Appears in:_
- [ZTunnel](#ztunnel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Defines the version of Istio to install. Must be one of: v1.24.2, v1.24.1. | v1.24.2 | Enum: [v1.24.2 v1.24.1]   |
| `profile` _string_ | The built-in installation configuration profile to use. The 'default' profile is 'ambient' and it is always applied. Must be one of: ambient, default, demo, empty, external, preview, remote, stable. | ambient | Enum: [ambient default demo empty external openshift-ambient openshift preview remote stable]   |
| `namespace` _string_ | Namespace to which the Istio ztunnel component should be installed. | ztunnel |  |
| `values` _[ZTunnelValues](#ztunnelvalues)_ | Defines the values to be passed to the Helm charts when installing Istio ztunnel. |  |  |


#### ZTunnelStatus



ZTunnelStatus defines the observed state of ZTunnel



_Appears in:_
- [ZTunnel](#ztunnel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this ZTunnel object. It corresponds to the object's generation, which is updated on mutation by the API Server. The information in the status pertains to this particular generation of the object. |  |  |
| `conditions` _[ZTunnelCondition](#ztunnelcondition) array_ | Represents the latest available observations of the object's current state. |  |  |
| `state` _[ZTunnelConditionReason](#ztunnelconditionreason)_ | Reports the current state of the object. |  |  |


