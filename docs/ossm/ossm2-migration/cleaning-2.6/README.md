# Cleaning of OpenShift Service Mesh 2.6 after migration
When the migration of all workloads is finished, it's possible to remove OpenShift Service Mesh 2.6 installation.

## Remove 2.6 control planes
1. Find all Service Mesh 2.6 resources:
    ```sh
    oc get smcp,smm,smmr -A
    ```
1. Remove all found `ServiceMeshControlPlane`
    ```sh
    oc delete smcp basic -n istio-system
    ```
1. Remove all found `ServiceMeshMemberRoll`
    ```sh
    oc delete smmr default -n istio-system
    ```
1. Remove all found `ServiceMeshMembers`
    ```sh
    oc delete smm default -n bookinfo
    ```
1. Verify that all resources were removed:
    ```sh
    oc get smcp,smm,smmr -A
    No resources found
    ```

> **_NOTE:_** that depending on how you created `ServiceMeshMembers` and `ServiceMeshMemberRoll`, those resources might be removed automatically with removal of `ServiceMeshControlPlane` after step 2.

## Remove 2.6 operator and CRDs
1. Make sure there are no Service Mesh 2.6 resources left:
    ```sh
    oc get smcp,smm,smmr -A
    No resources found
    ```
1. Remove the operator
    ```sh
    csv=$(oc get subscription servicemeshoperator -n openshift-operators -o yaml | grep currentCSV | cut -f 2 -d ':')
    oc delete subscription servicemeshoperator -n openshift-operators
    oc delete clusterserviceversion $csv -n openshift-operators
    ```
1. Remove Maistra CRDs
    ```sh
    oc get crds -o name | grep ".*\.maistra\.io" | xargs -r -n 1 oc delete
    ```

## Remove Maistra labels
Optionally you can remove namespace labels created during the migration.
1. Following resources should be already removed in previous steps but it's important to be sure there are no Service Mesh 2.6 resources left before removing the label:
    ```sh
    oc get smcp,smm,smmr -A
    No resources found
    ```
1. Find namespaces with the `maistra.io/ignore-namespace="true"` label:
    ```sh
    oc get namespace -l maistra.io/ignore-namespace="true"
    NAME            STATUS   AGE
    bookinfo   Active   127m
    ```
1. Remove the label:
    ```sh
    oc label namespace bookinfo maistra.io/ignore-namespace-
    namespace/bookinfo unlabeled
    ```