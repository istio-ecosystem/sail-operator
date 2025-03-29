##### Pre-build functions to use in the scripts
# Insert here any function that is going to be used in more than one script
# The function can be inserted in the generated script by adding the tag 
# <!-- prebuilt function_name --> in the documentation file
# Take into account that the function name should match the function name in the script

# Install the Sail Operator
function install_sail_operator() {
    # The image of the operator is already being pushed to the registry in the setup_env function
    make -e IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" deploy

    # Wait for deployment to be ready
    kubectl wait --for=condition=available deployment/sail-operator -n sail-operator --timeout=5m

}

# Uninstall operator
function uninstall_operator() {
    # Running make undeploy will do the work for us
    make undeploy

    # We need to wait for the operator to be uninstalled
    kubectl wait --for=delete deployment/sail-operator -n sail-operator --timeout=5m
}

# setup_env setup a test environment to run the tests
# We are reusing the current script to run the e2e tests that includes build and push operator image.
function setup_env() {
    if [ -n "${OPENSHIFT:-}" ] && [ "${OPENSHIFT}" != "false" ]; then
        # Running the integ-suite-ocp.sh script to setup the environment for OpenShift
        echo "### Running Openshift setup..."
        SKIP_TESTS=true SKIP_CLEANUP=true ./tests/e2e/integ-suite-ocp.sh
    else
        # run the integ-suite-kind.sh script to setup the kind environment to run the tests
        echo "### Running Kind setup..."
        SKIP_TESTS=true SKIP_CLEANUP=true ./tests/e2e/integ-suite-kind.sh
    fi
}

# cleanup_env checks if we are running on OpenShift to cleanup the environment for the tests
function cleanup_env() {
    if [ -n "${OPENSHIFT:-}" ] && [ "${OPENSHIFT}" != "false" ]; then
        echo "Running cleanup for OpenShift"
        # TODO: Add cleanup steps for OpenShift, we need to ensure that the next test will be able to run
    else
        echo "Running cleanup for kind"
        # For Kind we need to delete the cluster
        kind delete cluster --name "${KIND_CLUSTER_NAME}"
    fi
}

# wait will wait for the specified resource to meet the condition
# <!-- wait condition resourceType resourceName namespace -->
function wait() {
    local condition=$1
    local resourceType=$2
    local resourceName=$3
    local namespace=$4

    sleep 5
    kubectl wait --for=condition="${condition}" "${resourceType}/${resourceName}" -n "${namespace}" --timeout=60s
}
