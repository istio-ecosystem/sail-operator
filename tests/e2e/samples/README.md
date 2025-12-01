# Samples kustomize files for e2e tests

This directory contains the kustomize files used in end-to-end tests to be used as sample apps for testing purposes.

## Why using kustomize?
Upstream sample yamls are located under the [samples folder](https://github.com/istio/istio/tree/master/samples) in the upstream repo. These yaml files use images usually located in Docker Hub, but this can cause issues while running intensive tests because of rate limiting. To avoid this, we use kustomize to patch the upstream yaml files to use images located in the `quay.io/sail-dev` registry.

## How to use these samples?
To use these samples, you can run the following command:

```bash
kubectl apply -k tests/e2e/samples/<sample_name>
```

Where `<sample_name>` is the name of the sample directory you want to use. For example, to use the `httpbin` sample, you can run:

```bash
kubectl apply -k tests/e2e/samples/httpbin
```

## Available samples
- [httpbin](httpbin/): A simple HTTP request and response service.
- [sleep](sleep/): A simple sleep service that can be used to test Istio features.
- [helloworld](helloworld/): A simple hello world service that can be used to test Istio features.
- [tcp-echo-dual-stack](tcp-echo-dual-stack/): A simple TCP echo service that can be used to test Istio features in dual-stack mode.
- [tcp-echo-ipv4](tcp-echo-ipv4/): A simple TCP echo service that can be used to test Istio features in IPv4 mode.
- [tcp-echo-ipv6](tcp-echo-ipv6/): A simple TCP echo service that can be used to test Istio features in IPv6 mode.

## How to add a new sample?
To add a new sample, follow these steps:

1. Create a new directory under `tests/e2e/samples/` with the name of the sample (e.g., `tests/e2e/samples/my-sample/`)
2. Create a `kustomization.yaml` file in the new directory with the following configuration:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
    - <path_to_upstream_sample_yaml>
images:
    - name: <image_name>
      newName: quay.io/sail-dev/<image_name>
```
    
Where `<path_to_upstream_sample_yaml>` is the path to the upstream sample yaml file, `<image_name>` is the name of the image used in the upstream sample yaml file, and `<newName>` is the new name of the image in the `quay.io/sail-dev` registry. Please keep the images name the same as the original because we use the original name to match the image in the upstream sample yaml file.

## Keep images up to date in quay.io/sail-dev
To keep the images up to date in the `quay.io/sail-dev` registry, we use an automatic script that checks the tags used upstream and uses `crane` to copy them into the `quay.io/sail-dev` registry. To run this script, you can use the following command:

```bash
make update-istio-samples
```

This command will check the tags used in the upstream sample yaml files and copy the images to the `quay.io/sail-dev` registry using `crane`.
