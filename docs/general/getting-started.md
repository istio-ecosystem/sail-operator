[Return to Project Root](../README.md)

# Table of Contents

- [Getting Started](#getting-started)
  - [Installation on OpenShift](#installation-on-openshift)
    - [Installing through the web console](#installing-through-the-web-console)
    - [Installing using the CLI](#installing-using-the-cli)
  - [Installation from Source](#installation-from-source)
  - [Migrating from Istio in-cluster Operator](#migrating-from-istio-in-cluster-operator)
    - [Converter Script to Migrate Istio in-cluster Operator Configuration to Sail Operator](#converter-script-to-migrate-istio-in-cluster-operator-configuration-to-sail-operator)
        - [Usage](#usage)
        - [Sample command only with input file:](#sample-command-only-with-input-file)
        - [Sample command with custom output, namespace, and version:](#sample-command-with-custom-output-namespace-and-version)
  - [Uninstalling](#uninstalling)
    - [Deleting Istio](#deleting-istio)
    - [Deleting IstioCNI](#deleting-istiocni)
    - [Deleting the Sail Operator](#deleting-the-sail-operator)
    - [Deleting the istio-system and istio-cni Projects](#deleting-the-istio-system-and-istio-cni-projects)
    - [Decide whether you want to delete the CRDs as well](#decide-whether-you-want-to-delete-the-crds-as-well)

## Getting Started

## Installation on OpenShift

### Installing through the web console

1. In the OpenShift Console, navigate to the OperatorHub by clicking **Operator** -> **Operator Hub** in the left side-pane.

1. Search for "sail".

1. Locate the Sail Operator, and click to select it.

1. When the prompt that discusses the community operator appears, click **Continue**, then click **Install**.

1. Use the default installation settings presented, and click **Install** to continue.

1. Click **Operators** -> **Installed Operators** to verify that the Sail Operator 
is installed. `Succeeded` should appear in the **Status** column.

### Installing using the CLI

*Prerequisites*

* You have access to the cluster as a user with the `cluster-admin` cluster role.

*Steps*

1. Create the `openshift-operators` namespace (if it does not already exist).

    ```bash
    kubectl create namespace openshift-operators
    ```

1. Create the `Subscription` object with the desired `spec.channel`.

   ```bash
   kubectl apply -f - <<EOF
       apiVersion: operators.coreos.com/v1alpha1
       kind: Subscription
       metadata:
         name: sailoperator
         namespace: openshift-operators
       spec:
         channel: "0.1-nightly"
         installPlanApproval: Automatic
         name: sailoperator
         source: community-operators
         sourceNamespace: openshift-marketplace
   EOF
   ```

1. Verify that the installation succeeded by inspecting the CSV status.

    ```console
    $ kubectl get csv -n openshift-operators
    NAME                                     DISPLAY         VERSION                    REPLACES                                 PHASE
    sailoperator.v0.1.0-nightly-2024-06-25   Sail Operator   0.1.0-nightly-2024-06-25   sailoperator.v0.1.0-nightly-2024-06-21   Succeeded
    ```

    `Succeeded` should appear in the sailoperator CSV `PHASE` column.

## Installation from Source

If you're not using OpenShift or simply want to install from source, follow the [instructions in the Contributor Documentation](../README.md#deploying-the-operator).

## Migrating from Istio in-cluster Operator

If you're planning to migrate from the [now-deprecated Istio in-cluster operator](https://istio.io/latest/blog/2024/in-cluster-operator-deprecation-announcement/) to the Sail Operator, you will have to make some adjustments to your Kubernetes Resources. While direct usage of the IstioOperator resource is not possible with the Sail Operator, you can very easily transfer all your settings to the respective Sail Operator APIs. As shown in the [Concepts](../README.md#concepts) section, every API resource has a `spec.values` field which accepts the same input as the `IstioOperator`'s `spec.values` field. Also, the [Istio resource](../README.md#istio-resource) provides a `spec.meshConfig` field, just like IstioOperator does.

Another important distinction between the two operators is that Sail Operator can manage and install different versions of Istio and its components, whereas the in-cluster operator always installs the version of Istio that it was released with. This makes managing control plane upgrades much easier, as the operator update is disconnected from the control plane update.

So for a simple Istio deployment, the transition will be very easy:

```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  meshConfig:
    accessLogFile: /dev/stdout
  values:
    pilot:
      traceSampling: 0.1
```

becomes

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
spec:
  meshConfig:
    accessLogFile: /dev/stdout
  values:
    pilot:
      traceSampling: 0.1
  version: v1.24.3
```

Note that the only field that was added is the `spec.version` field. There are a few situations however where the APIs are different and require different approaches to achieve the same outcome.

### Setting environments variables for Istiod

In Sail Operator, all `.env` fields are `map[string]string` instead of `struct{}`, so you have to be careful with values such as `true` or `false` - they need to be in quotes in order to pass the type checks!

That means the following YAML

```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:
  values:
    global:
      istiod:
        enableAnalysis: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: true
```

becomes

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    global:
      istiod:
        enableAnalysis: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: "true"
  version: v1.24.3
  namespace: istio-system
```

Note the quotes around the value of `spec.values.pilot.env.PILOT_ENABLE_STATUS`. Without them, Kubernetes would reject the YAML as it expects a value of type `string` but receives a `boolean`.

### Components field

Sail Operator's Istio resource does not have a `spec.components` field. Instead, you can enable and disable components directly by setting `spec.values.<component>.enabled: true/false`. Other functionality exposed through `spec.components` like the k8s overlays is not currently available.

### CNI lifecycle management

The CNI plugin's lifecycle is managed separately from the control plane. You will have to create a [IstioCNI resource](../README.md#istiocni-resource) to use CNI.

### Converter Script to Migrate Istio in-cluster Operator Configuration to Sail Operator

This script is used to convert an Istio in-cluster operator configuration to a Sail Operator configuration. Upon execution, the script takes an input YAML file and istio version and generates a sail operator configuration file.

#### Usage
To run the configuration-converter.sh script, you need to provide four arguments, only input file is required other arguments are optional:

1. Input file path (<input>): The path to your Istio operator configuration YAML file (required).
2. Output file path (<output>): The path where the converted Sail configuration will be saved. If not provided, the script will save the output with -sail.yaml appended to the input file name.
3. Namespace (-n <namespace>): The Kubernetes namespace for the Istio deployment. Defaults to istio-system if not provided.
4. Version (-v <version>): The version of Istio to be used. If not provided, the `spec.version` field will be omitted from the output file and the operator will deploy the latest version when the YAML manifest is applied.

```bash
./tools/configuration-converter.sh </path/to/input.yaml> [/path/to/output.yaml] [-n namespace] [-v version]
```

##### Sample command only with input file:

```bash
./tools/configuration-converter.sh /home/user/istio_config.yaml
```

##### Sample command with custom output, namespace, and version:

```bash
./tools/configuration-converter.sh /home/user/input/istio_config.yaml /home/user/output/output.yaml -n custom-namespace -v v1.24.3
```

> [!WARNING]
> This script is still under development.
> Please verify the resulting configuration carefully after conversion to ensure it meets your expectations and requirements.

## Uninstalling

### Deleting Istio
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Click **Istio** in the **Provided APIs** column.
1. Click the Options menu, and select **Delete Istio**.
1. At the prompt to confirm the action, click **Delete**.

### Deleting IstioCNI
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Click **IstioCNI** in the **Provided APIs** column.
1. Click the Options menu, and select **Delete IstioCNI**.
1. At the prompt to confirm the action, click **Delete**.

### Deleting the Sail Operator
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Locate the Sail Operator. Click the Options menu, and select **Uninstall Operator**.
1. At the prompt to confirm the action, click **Uninstall**.

### Deleting the istio-system and istio-cni Projects
1. In the OpenShift Container Platform web console, click  **Home** -> **Projects**.
1. Locate the name of the project and click the Options menu.
1. Click **Delete Project**.
1. At the prompt to confirm the action, enter the name of the project.
1. Click **Delete**.

### Decide whether you want to delete the CRDs as well
OLM leaves this [decision](https://olm.operatorframework.io/docs/tasks/uninstall-operator/#step-4-deciding-whether-or-not-to-delete-the-crds-and-apiservices) to the users.
If you want to delete the Istio CRDs, you can use the following command.
```bash
kubectl get crds -oname | grep istio.io | xargs kubectl delete
```
