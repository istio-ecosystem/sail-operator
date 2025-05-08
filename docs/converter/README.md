# Running Istio Integration Tests With Sail Operator

This document explains the changes implemented on upstream Istio to let users execute the integration tests with OSSM 3.x Operator on Openshift or any Kubernetes environment that has installed the Sail Operator.

**Note:** Currently, the Integration test runner script [integ-suite-ocp.sh](https://github.com/openshift-service-mesh/istio_ossm/blob/master/prow/integ-suite-ocp.sh) from the midstream repository is going to be used to execute these tests.

## Setting Environment Variables
To set the necessary test parameters and call the converter script, you need to set: ```CONTROL_PLANE_SOURCE=sail``` env var in your environment to let the Istio testing framework know that it is going to use sail as the control plane installer

If you want the test runner script to install the sail operator, you need to set the environment variable ```INSTALL_SAIL_OPERATOR=true```. This will set the script to automatically install the control plane using the logic in the runner script.

## Executing Tests:
### Prerequisites:
    - To execute integration tests you need to locally clone the [service-mesh-istio](https://github.com/openshift-service-mesh/istio) project from github.

    - To run pilot test suite execute:
        ```sh
        prow/integ-suite-ocp.sh pilot TestGatewayConformance|TestCNIVersionSkew|TestGateway|TestAuthZCheck|TestKubeInject|TestRevisionTags|TestUninstallByRevision|TestUninstallWithSetFlag|TestUninstallCustomFile|TestUninstallPurge|TestCNIRaceRepair|TestValidation|TestWebhook|TestMultiRevision
        ```
        Note: As you can see there are some skips that are not working yet over Openshift, this is managed under the Jira ticket https://issues.redhat.com/browse/OSSM-9328 and this documentation is going to be updated as soon the Jira is solved. 

    - To run telemetry suite execute:
        ```sh
        prow/integ-suite-ocp.sh telemetry
        ```

### Debugging the converter with the script logs:
Every execution of the converter script creates a log file, which you can follow for errors that might happen during the creation of elements such as istio-cni, istio-gateways, etc.

The log file is created under the execution directory, which is set by --istio.test.work_dir. You can also see the iop-sail.yaml file that has the Sail Operator configuration converted from Istio Operator control plane configuration.

The following is an example of the folder where you can find artifacts created in test execution:
https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift-service-mesh_istio/374/pull-ci-openshift-service-mesh-istio-master-istio-integration-sail-pilot/1920053129898889216/artifacts/istio-integration-sail-pilot/integ-sail-pilot-test-run/artifacts/pilot-4def8a9fdff144de8e4f22463/_suite_context/istio-deployment-611939208/

### Debugging test with dlv + vscode:
#### Prerequisites:
    - To debug test/s in integration test suite you need to have Sail Operator installed in the cluster

#### Executing Debugger
    - Put your breakpoints on desired test in vscode
    - Add following launch config to .vscode/launch.json
        ```json
        {
            "version": "0.2.0",
            "configurations": [
                {
                    "name": "Attach to Delve",
                    "type": "go",
                    "request": "attach",
                    "mode": "remote",
                    "port": 2345,
                    "host": "127.0.0.1",
                    "apiVersion": 2
                }
            ]
        }
        ```
    - Execute following dlv command on terminal with modifying -test.run as desired
        ```sh
        dlv test --headless --listen=:2345 --api-version=2 --log --build-flags "-tags=integ" -- \
        -test.v -test.count=1 -test.timeout=60m -test.run TestTraffic/externalname/routed/auto-http \
        --istio.test.ci \
        --istio.test.pullpolicy=IfNotPresent \
        --istio.test.work_dir=/home/ubuntu/istio_ossm/prow/artifacts \
        --istio.test.skipTProxy=true \
        --istio.test.skipVM=true \
        --istio.test.kube.helm.values=global.platform=openshift \
        --istio.test.istio.enableCNI=true \
        --istio.test.hub=image-registry.openshift-image-registry.svc:5000/istio-system \
        --istio.test.tag=istio-testing \
        --istio.test.openshift \
        --istio.test.kube.deploy=false \
        --istio.test.kube.controlPlaneInstaller=/home/ubuntu/istio_ossm/prow/setup/sail-operator-setup.sh
        ```
    - When the dlv command starts go to vscode and execute "Attach to Delve" debugger

