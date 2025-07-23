# Samples kustomize files for e2e tests

This directory contains the kustomize files used in end-to-end tests to be used as samples apps for testing purposes.

## Why using kustomize?
Upstream sample yaml are located under the [samples folder](https://raw.githubusercontent.com/istio/istio/master/samples) in the upstream repo. This yaml files use images usually located in docker hub, but this can cause issue while running intensive test because of rate limiting. To avoid this, we use kustomize to patch the upstream yaml files to use images located in the `quay.io/sail-dev` registry.

## How to use these samples?
To use these samples, you can run the following command:

```bash
kubectl apply -k tests/e2e/samples/<sample_name>
```

Where `<sample_name>` is the name of the sample you want to use. For example, to use the `httpbin` sample, you can run:

```bash
kubectl apply -k tests/e2e/samples/httpbin.yaml
```

## Available samples
- [httpbin](httpbin.yaml): A simple HTTP request and response service.
- [sleep](sleep.yaml): A simple sleep service that can be used to test Istio features.
- [helloworld](helloworld.yaml): A simple hello world service that can be used to test Istio features.
- [tcp-echo-dual-stack](tcp-echo-dual-stack.yaml): A simple TCP echo service that can be used to test Istio features in dual-stack mode.
- [tcp-echo-ipv4](tcp-echo-ipv4.yaml): A simple TCP echo service that can be used to test Istio features in IPv4 mode.
- [tcp-echo-ipv6](tcp-echo-ipv6.yaml): A simple TCP echo service that can be used to test Istio features in IPv6 mode.

## How to add a new sample?
To add a new sample, you just need to create a new file under `tests/e2e/samples/` with the name of the sample and the following configuration:

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
To keep the images up to date in the `quay.io/sail-dev` registry, we use an automatic job that check the tags used upstream and use `crane` to do a copy into the `quay.io/sail-dev` registry. To run this job, you can use the following command:

```bash
make update-samples-images
```

This command will check the tags used in the upstream sample yaml files and copy the images to the `quay.io/sail-dev` registry using `crane`.
