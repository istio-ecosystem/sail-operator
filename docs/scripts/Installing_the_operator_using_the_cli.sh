#!/bin/bash

# This script was generated from the documentation file docs/README.md
# Please check the documentation file for more information

# <!-- generate-docs-test-init Installing_the_operator_using_the_cli-->
# *Prerequisites*
# 
# * You have access to the cluster as a user with the `cluster-admin` cluster role.
# 
# *Steps*
# 
# 1. Create the `openshift-operators` namespace (if it does not already exist).
# 

kubectl create namespace openshift-operators

# 
# 1. Create the `Subscription` object with the desired `spec.channel`.
# 

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

# 
# 1. Verify that the installation succeeded by inspecting the CSV status.
# 

kubectl get csv -n openshift-operators

#     console
#     NAME                                     DISPLAY         VERSION                    REPLACES                                 PHASE
#     sailoperator.v0.1.0-nightly-2024-06-25   Sail Operator   0.1.0-nightly-2024-06-25   sailoperator.v0.1.0-nightly-2024-06-21   Succeeded
#     
# 
#     `Succeeded` should appear in the sailoperator CSV `PHASE` column.
# <!-- generate-docs-test-end -->
