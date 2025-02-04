# OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin is an extension to the OpenShift web console that provides visibility into your Service Mesh. With the OSSMC plugin installed, a new **Service Mesh** menu category is available in the navigation menu on the left side of the web console, as well as new **Service Mesh** tabs that enhance the existing **Workloads** and **Services** OpenShift console detail pages.

The features of the OSSMC plugin are the same as those of the standalone Kiali Console, but the pages are organized differently to better integrate with the OpenShift console. The OSSMC plugin does not replace the Kiali Console, and after installing the OSSMC plugin, you can still access the standalone Kiali Console.

[!IMPORTANT]
If you are using a certificate that your browser does not initially trust, you must tell your browser to trust the certificate first before you are able to access the OSSMC plugin. To do this, go to the Kiali standalone user interface (UI) and tell the browser to accept its certificate.

## About the OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin is an extension to the OpenShift web console that provides visibility into your Service Mesh.

[!WARNING]
The OSSMC plugin only supports a single Kiali instance. Whether that Kiali instance is configured to access only a subset of OpenShift projects or has access cluster-wide to all projects does not matter. However, only a single Kiali instance can be accessed.

The OSSMC plugin provides a new category, **Service Mesh**, in the main OpenShift web console navigation.

The following menu options are available under **Service Mesh** category:

* **Overview** for a summary of your mesh, displayed as cards that represent the namespaces in the mesh.
* **Traffic Graph** for a full topology view of your mesh, represented by nodes and edges, each node representing a component of the mesh and each edge representing traffic flowing through the mesh between components.
* **Istio config** for a list of all Istio configuration files in your mesh, with a column that provides a quick way to know if the configuration for each resource is valid.
* **Mesh** for detailed information about the Istio infrastructure status. It shows an infrastructure topology view with core and add-on components, their health, and how they are connected to each other.

Under the OpenShift **Workloads** details page, the OSSMC plugin adds a **Service Mesh** tab that contains the following subtabs:

* **Overview** subtab provides a summary of the selected workload, including a localized topology graph showing the workload with all inbound and outbound edges and nodes.
* **Traffic** subtab displays information about all inbound and outbound traffic to the workload.
* **Logs** subtab shows the logs for the workload's containers.

  * You can view container logs individually or in a unified fashion, ordered by log time. This is especially helpful to see how the Envoy sidecar proxy logs relate to your workload's application logs.
  * You can enable the tracing span integration which then allows you to see which logs correspond to trace spans.

* **Metrics** subtab shows both inbound and outbound metric graphs in the corresponding subtabs. All the workload metrics can be displayed here, providing you with a detail view of the performance of your workload.

  * You can enable the tracing span integration which allows you to see which spans occurred at the same time as the metrics. Click a span marker in the graph to view the specific spans associated with that timeframe.

* **Traces** provides a chart showing the trace spans collected over the given timeframe.

  * Click a bubble to drill down into those trace spans; the trace spans can provide you the most low-level detail within your workload application, down to the individual request level. The trace details view gives further details, including heatmaps that provide you with a comparison of one span in relation to other requests and spans in the same timeframe.
  * If you hover over a cell in a heatmap, a tooltip gives some details on the cell data.

* **Envoy** subtab provides information about the Envoy sidecar configuration. This is useful when you need to dig down deep into the sidecar configuration when debugging things such as connectivity issues.

Under OpenShift **Networking** details page, the OSSMC plugin adds a **Service Mesh** tab to Services and contains the **Overview**, **Traffic**, **Inbound Metrics**, and **Traces** subtabs that are similar to the same subtabs found in **Workloads**.

## Installing the OpenShift Service Mesh Console plugin

The OSSMC plugin can be installed in two ways: using the OpenShift web console or the OpenShift CLI.

[!NOTE]
The OSSMC plugin is only supported on OpenShift 4.15 and above. For OCP 4.14 users, only the standalone Kiali Console will be accessible.

### Install via the OpenShift web console

The following steps show how to install the OSSMC plugin via the OpenShift web console.

**Prerequisites**

* Access to the OpenShift web console with administrator access.
* Red Hat OpenShift Service Mesh (OSSM) is installed.
* `Istio` control plane from OSSM 3.0 is installed.
* Kiali Server 2.4 is installed.

**Procedure**

1. Navigate to **Installed Operators**.

2. Click the **Kiali Operator** item to access to the operator details page.

3. Click **Create instance** on the **Red Hat OpenShift Service Mesh Console** tile. Another way is to click **Create OSSMConsole** button under the **OpenShift Service Mesh Console** tab.

4. Use the **Create OSSMConsole** form to create an instance of the `OSSMConsole` custom resource (CR).

    * **Name** and **Version** are required fields.

    [!NOTE]
    The **Version** field must match the `spec.version` field in your Kiali CR. If the versions do not match, OSSMC will not work properly. In case the **Version** value is the string "default", the Kiali Operator will install OSSMC whose version is the same as the operator itself.

5. Click **Create**.

6. Once the OpenShift Console UI detects the availability of the OSSM Console plugin, a message will appear in the OpenShift Console asking you to refresh it. You can use OSSMC once you refresh the OpenShift Console UI.

### Install via the OpenShift CLI

The following steps show how to install the OSSMC plugin via the OpenShift CLI.

**Prerequisites**

* Access to the OpenShift cluster via CLI with administrator privileges.
* Red Hat OpenShift Service Mesh (OSSM) is installed.
* `Istio` control plane from OSSM 3.0 is installed.
* Kiali Server 2.4 is installed.

**Procedure**

1. Create a small `OSSMConsole` custom resource (CR) to instruct the operator to install the plugin:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: OSSMConsole
    metadata:
      namespace: openshift-operators
      name: ossmconsole
    spec:
      version: default
    EOM
    ```

    [!NOTE]
    If the `spec.version` field is not specified (or if set explicitly to the string “default”), then the Kiali Operator will install OSSMC whose version is the same as the operator itself. It is very important that the version of OSSMC be the same version as the Kiali Server that is installed. If the versions do not match, OSSMC will not work properly.

    The plugin resources are deployed in the same namespace where the `OSSMConsole` CR is created.

    If you have more than one Kiali installed in your cluster, you should tell OSSMC which one to communicate with. You do so via the “spec.kiali” settings in the OSSMConsole CR. For example:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: OSSMConsole
    metadata:
      namespace: openshift-operators
      name: ossmconsole
    spec:
      kiali:
        serviceName: kiali
        serviceNamespace: istio-system-two
        servicePort: 20001
    EOM
    ```

2. Go to the OpenShift web console.

3. If the OSSMC plugin is not availble yet, a message will appear in the OpenShift Console asking to refresh once the OSSM Console plugin is ready. You can use OSSMC once you refresh the OpenShift Console UI.

## Uninstalling the OpenShift Service Mesh Console plugin

The OSSMC plugin can be uninstalled in two ways: using the OpenShift web console or the OpenShift CLI.

[!NOTE]
If you intend to also uninstall the Kiali Operator provided by Red Hat, you must first uninstall the OSSMC plugin, then uninstall Kiali and finally uninstall the Operator. If you uninstall the Operator before ensuring the `OSSMConsole` and `Kiali` CRs are deleted then you may have difficulty removing that CRs and their namespaces. If this occurs then you must manually remove the finalizer on the CR in order to delete it and its namespace. You can do this using: `$ oc patch <CR type> <CR name> -n <CR namespace> -p '{"metadata":{"finalizers": []}}' --type=merge`, where CR type is `kialis` or `ossmconsoles`.

### Uninstall via the OpenShift web console

The following steps show how to uninstall the OSSMC plugin via the OpenShift web console.

**Procedure**

1. Navigate to **Installed Operators**.

2. Click the **Kiali Operator** item to access to the operator details page.

3. Select the **OpenShift Service Mesh Console** tab.

4. Click **Delete OSSMConsole** option from the ossmconsole entry menu.

5. Confirm the delete in the modal confirmation message.

### Uninstall via the OpenShift CLI

The following steps show how to uninstall the OSSMC plugin via the OpenShift CLI.

**Procedure**

1. Remove the `OSSMC` custom resource (CR) by running the following command:

    ```console
    oc delete ossmconsoles <custom_resource_name> -n <custom_resource_namespace>
    ```

2. Verify all CRs are deleted from all namespaces by running the following command:

    ```console
    for r in $(oc get ossmconsoles --ignore-not-found=true --all-namespaces -o custom-columns=NS:.metadata.namespace,N:.metadata.name --no-headers | sed 's/  */:/g'); do oc delete ossmconsoles -n $(echo $r|cut -d: -f1) $(echo $r|cut -d: -f2); done
    ```
