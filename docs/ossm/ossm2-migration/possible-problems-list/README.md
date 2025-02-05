# OpenShift Service Mesh 2 --> 3 Migration risks and recommendations
Given the nature of the migration (not upgrading the operator but migrating to brand new operator), there are a few problems which can't be handled by any of the OpenShift Service Mesh 2 or OpenShift Service Mesh 3 operators. We are listing those problems and recommendations here so users can prepare in advance.

## Recommendations
Following are recommendations to keep the risk of misconfigurations or possible conflicts to minimum.

1. Do not upgrade OpenShift Service Mesh 2 operator or control planes in the middle of the migration

    After the OpenShift Service Mesh 3 operator is installed and migration of data plane is in progress, it's not recommended to upgrade OpenShift Service Mesh 2 operator or control planes. This can be achieved by switching from `Automatic` Operator Update approval to `Manual`.
1. Keep the amount of the service mesh configuration changes to minimum during the migration

    It's not recommended to change Istio resources for traffic management or security or add new workloads or gateways to the service mesh in the middle of the migration.
    > **_NOTE:_** If it's necessary to add a new workload namespace during the migration, it should be managed by 3.0 control plane and it MUST be labeled with `maistra.io/ignore-namespace: "true"` to avoid conflicts between 3.0 and 2.6 control planes.
1. Finish the migration without unnecessary delays

Once the migration is started, it should be completed as quickly as possible without unnecessary delays.
