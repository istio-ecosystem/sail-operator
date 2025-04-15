# Example documentation where the guidelines are followed
This is an example doc where the guidelines are followed to achieve the best documentation possible. The doc is going to be used to test the automation workflow that is going to be used to run the tests over the documentation.

*Note:* the docs shown here may not be updated to the latest version of the project, so please take into account that the examples may not work as expected. The goal of this doc is to show how to use the guidelines and how to add new examples to the automation workflow.

## Runme Test: Installing the operator from the helm repo and creating a Istio resource

To run this test, you need to have a Kubernetes cluster running and have `kubectl` and `helm` installed on your local machine. Also, `istioctl` should be installed and configured to work with your cluster.

- Check the commands that are going to be executed:
```bash { ignore=true }
runme list --filename docs/guidelines/example-documentation-following-guidelines.md
```

- Run the commands:
```bash { ignore=true }
runme run --filename docs/guidelines/example-documentation-following-guidelines.md --all --skip-prompts
```
This will run *all* the commands in the file. If you want to run only a specific command, you can use the `--tag` option to filter the commands by tag. For example, to run only the commands with the tag `example`, you can use the following command:
```bash { ignore=true }
runme run --filename docs/common/runme-test.md --tag example --skip-prompts
```
For more information about tags and how to use them, you can check the [Runme documentation](https://docs.runme.dev/usage/run-tag).

More information:
- [Cell configuration keys](https://docs.runme.dev/configuration/cell-level#cell-configuration-keys)
- [Running from cli](https://docs.runme.dev/getting-started/cli)

### Setting a Istio resource

The `Istio` resource is a custom resource that is used to configure Istio in your cluster. An example of an Istio resource is shown below:

```yaml { ignore=true }
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
```
- To create the Istio resource, you can use the following command:
```bash { name=create-istio tag=example}
kubectl create ns istio-system
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: InPlace
    inactiveRevisionDeletionGracePeriodSeconds: 30
EOF
```

```bash { name=validation-print-istio-resource tag=example}
kubectl get istio -o yaml
kubectl get deployment sail-operator -n sail-operator -o yaml
```

```bash { name=validation-wait-istiod tag=example}
for i in 1 2 3 4 5; do
  pods=$(kubectl get pod -l app=istiod -n istio-system -o jsonpath='{.items[*].status.phase}')
  echo "Waiting for istiod pod to be running... (current: $pods)"
  echo "$pods" | grep -q Running && break
  sleep 5
done
```

- To check the status of the Istio resource, you can use the following command:
```bash { name=check-istio tag=example}
kubectl get pods -n istio-system
kubectl get istio
```

- Deploy sample application:
```bash { name=deploy-sample-app tag=example}
kubectl create namespace sample
kubectl label namespace sample istio-injection=enabled
kubectl apply -n sample -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/platform/kube/bookinfo.yaml
```
```bash { name=validation-wait-sample-app tag=example}
for i in {1..5}; do kubectl wait --for=condition=available --timeout=600s deployment/productpage-v1 -n sample && break || sleep 5; done
```

- Check the status of the sample application:
```bash { name=check-sample-app tag=example}
kubectl get pods -n sample
```
```bash { name=check-sidecar.exist tag=example}
if ! kubectl get pods -n sample -l app=productpage -o jsonpath='{range .items[*]}{@.metadata.name}{" "}{range .spec.containers[*]}{@.name}{" "}{end}{"\n"}{end}' | grep -q istio-proxy; then
  echo "No Istio sidecar (istio-proxy) injected in productpage pod!"
  exit 1
fi
```

- Check the proxy version of the sample application:
```bash { name=check-proxy-version tag=example}
istioctl proxy-status 
```
