[Return to Project Root](../)

# User Documentation
tbd

## Concepts
tbd

## Getting Started
tbd

## Gateways

The Sail-operator does not manage Gateways. You can deploy a gateway manually either through [gateway-api](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) or through [gateway injection](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway). As you are following the gateway installation instructions, skip the step to install Istio since this is handled by the Sail-operator.

**Note:** The `IstioOperator` / `istioctl` example is separate from the Sail-operator. Setting `spec.components` or `spec.values.gateways` on your Sail-operator `Istio` resource **will not work**.

## Multicluster
tbd

## Examples
tbd

## Observability Integrations
tbd

## Uninstalling
tbd
