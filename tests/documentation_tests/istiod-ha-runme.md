# Running Istiod in HA mode
By default, istiod is deployed with replica count set to 1, to be able to run it in HA mode, you can achieve it in two different ways:
* Setting `replicaCount` to 2 or more in Istio resource and disabling autoscale (by default enabled).
* Setting `autoscaleMin` to 2 or more in Istio resource and keeping `autoscaleMax` to 2 or more.

Pros and Cons of each approach:
- **Setting `replicaCount` to 2 or more**:
  - Pros: Simplicity, easy to understand and manage.
  - Cons: Fixed number of replicas, no autoscaling based on load. For single-node clusters, you may need to disable the default Pod Disruption Budget (PDB) as outlined in the [Considerations for Single-Node Clusters ](#considerations-for-single-node-clusters) section.
- **Setting `autoscaleMin` to 2 or more**:
  - Pros: Autoscaling based on load, can handle increased traffic without manual intervention, more efficient resource usage.
  - Cons: Requires monitoring to ensure proper scaling.

Now, let's see how to achieve this in Sail.

# Prerequisites
- Sail Operator installed and running in your cluster.
- kubernetes client configured to access your cluster.

## Setting up Istiod in HA mode: increasing replicaCount
To set up Istiod in HA mode by increasing the `replicaCount`, you can create/modify the Istio resource:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  values:
    pilot:
      autoscaleEnabled: false # <-- disable autoscaling
      replicaCount: 2   # <-- number of desired replicas
```
```bash { name=validation-istio-expected-version tag=istio-ha-replicacount }
kubectl create ns istio-system
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  values:
    pilot:
      autoscaleEnabled: false # <-- disable autoscaling
      replicaCount: 2   # <-- number of desired replicas
EOF
```

After applying this configuration, you can check the status of the Istiod pods:
```bash
kubectl get pods -n istio-system -l app=istiod
```
You should see two pods running, indicating that Istiod is now in HA mode.
```console
NAME                      READY   STATUS    RESTARTS   AGE
istiod-7c5947b8d7-88z7m   1/1     Running   0          14m
istiod-7c5947b8d7-ssnmt   1/1     Running   0          54m
```
```bash { name=validation-wait-istio-pods tag=istio-ha-replicacount }
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system"
    with_retries istiod_pods_count "2"
    print_istio_info
```

Let's break down the configuration:
- `spec.values.pilot.replicaCount: 2`: This sets the number of Istiod replicas to 2 (or the desired value), enabling HA mode.
- `spec.values.pilot.autoscaleEnabled: false`: This disables autoscaling, ensuring that the number of replicas remains fixed at 2 (or the desired value).

## Setting up Istiod in HA mode: using autoscaling
To set up Istiod in HA mode using autoscaling, you can create/modify the Istio resource as follows:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  values:
    pilot:
      autoscaleMin: 2   # <-- number of desired min replicas
      autoscaleMax: 5   # <-- number of desired max replicas
```
```bash { name=validation-istio-expected-version tag=istio-ha-autoscaling }
kubectl create ns istio-system
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  values:
    pilot:
      autoscaleMin: 2   # <-- number of desired min replicas
      autoscaleMax: 5   # <-- number of desired max replicas
EOF
```

After applying this configuration, you can check the status of the Istiod pods:
```bash
kubectl get pods -n istio-system -l app=istiod
```
You should see at least two pods running, indicating that Istiod is now in HA mode.
```console
NAME                      READY   STATUS    RESTARTS   AGE
istiod-7c7b6564c9-nwhsg   1/1     Running   0          70s
istiod-7c7b6564c9-xkmsl   1/1     Running   0          85s
```
```bash { name=validation-wait-istio-pods tag=istio-ha-autoscaling }
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system"
    with_retries istiod_pods_count "2"
    print_istio_info
```
Let's break down the configuration:
- `spec.values.pilot.autoscaleMin: 2`: This sets the minimum number of Istiod replicas to 2, ensuring that there are always at least 2 replicas running.
- `spec.values.pilot.autoscaleMax: 5`: This sets the maximum number of Istiod replicas to 5, allowing for scaling based on load.

## Considerations for Single-Node Clusters
For single-node clusters, it is crucial to disable the default Pod Disruption Budget (PDB) to prevent issues during node operations (e.g., draining) or scaling in HA mode. You can do this by adding the following configuration to your Istio resource:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  global:
    defaultPodDisruptionBudget:
      enabled: false # <-- disable default Pod Disruption Budget
```

`spec.global.defaultPodDisruptionBudget.enabled: false` disables the default Pod Disruption Budget for Istiod. In single-node clusters, a PDB can block operations such as node drains or pod evictions, as it prevents the number of available Istiod replicas from falling below the PDB's minimum desired count. Disabling it ensures smooth operations in this specific topology.
