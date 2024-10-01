## Installing the istioctl tool

The `istioctl` tool is a configuration command line utility that allows service 
operators to debug and diagnose Istio service mesh deployments.


### Prerequisites

Use an `istioctl` version that is the same version as the Istio control plane 
for the Service Mesh deployment. See [Istio Releases](https://github.com/istio/istio/releases) for a list of valid 
releases, including Beta releases. 

### Procedure

1. Confirm if you have `istioctl` installed, and if so which version, by running 
the following command at the terminal:

    ```sh
    $ istioctl version
    ```

2. Confirm the version of Istio you are using by running the following command 
at the terminal:

    ```sh
    $ oc get istio
    ```

3. Install `istioctl` by running the following command at the terminal: 

    ```sh
    $ curl -sL https://istio.io/downloadIstioctl | ISTIO_VERSION=<version> sh -
    ```
    Replace `<version>` with the version of Istio you are using.

4. Put the `istioctl` directory on path by running the following command at the terminal:
  
    ```sh
    $ export PATH=$HOME/.istioctl/bin:$PATH
    ```

5. Confirm that the `istioctl` client version and the Istio control plane 
version now match (or are within one version) by running the following command
at the terminal:

    ```sh
    $ istioctl version
    ```
For more information on usage, see the [Istioctl documentation](https://istio.io/latest/docs/ops/diagnostic-tools/istioctl/).

*Note*: `istioctl install` is not supported. The Sail Operator installs Istio.
