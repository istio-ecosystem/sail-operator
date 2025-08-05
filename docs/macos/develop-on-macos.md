# macOS Considerations for Sail Operator Development

This guide covers important considerations for developing, debugging, and working on the Sail Operator project on macOS, especially when using Podman instead of Docker.

## Using Podman Instead of Docker

If you use Podman, ensure your installation is properly configured for the Sail Operator project. This includes setting environment variables and making the Podman socket accessible. Initial Podman setup is important; run the following command to configure the Podman machine:

```bash
podman machine init --cpus 4 --memory 8192 --rootful
```

The `--rootful` flag allows Podman to run with root privileges, which is required for some Sail Operator operations. Adjust the `--cpus`, `--memory`, and `--disk-size` flags based on your system, but at least 8GB of memory and 4 CPUs are recommended.

> **Important:**  
> Follow Podman guides to manage Docker compatibility and ensure your Podman installation supports Docker commands.  
> For more information, see the [Podman documentation](https://podman-desktop.io/docs/migrating-from-docker/managing-docker-compatibility).

## Running Make Commands

The Sail Operator Makefile uses Docker commands. If you encounter issues with the `docker` command not being found, you can either create an alias for `docker` to point to `podman`, or set the `CONTAINER_CLI` environment variable to `podman`. Add the following to your shell configuration file (e.g., `.bashrc`, `.zshrc`) or run it in your terminal:

```bash
export CONTAINER_CLI=podman
```

Alternatively, you can set the variable inline when running `make`:

```bash
CONTAINER_CLI=podman make <target>
```

This ensures all Makefile commands that rely on Docker use Podman instead.

## Common Issues and Troubleshooting

If you encounter issues running Sail Operator on macOS, consider these troubleshooting steps:

- **Errors deploying the Sail Operator image in a kind or external cluster:**

    By default, Sail Operator runs make targets in a container. If using Podman, these containers run in the Podman machine, which may use a different architecture (e.g., if Rosetta 2 is enabled). This can cause issues when deploying images to a kind or external cluster. Ensure the image is built with the correct architecture and OS:

    ```bash
    make CONTAINER_CLI=podman \
             TARGET_OS=linux \
             TARGET_ARCH=arm64 \
             test.e2e.kind
    ```
Additionally, we set by default KIND_IMAGE to latest docker.io kind image which are compatible with the Podman machine. If you need to use a specific version, set the `KIND_IMAGE` environment variable:

    ```bash
    export KIND_IMAGE=kindest/node:v1.27.0
    ```
Note: the default image used by the kind script from upstream does not work on macOS with Podman, so you need to set it to a compatible version.

- **Unique UID error when running make commands:**

    For example, when running:

    ```bash
    CONTAINER_CLI=podman make deploy
    ```

    You may see:

    ```
    useradd: UID 501 is not unique
    go: creating work dir: mkdir /tmp/go-build1916180706: permission denied
    ```

    This occurs when the UID of the user running the command is not unique in the Podman machine. To resolve this, set a unique UID or set the GID to 0 (root):

    ```bash
    CONTAINER_CLI=podman DOCKER_GID=0 make deploy
    ```

    You can also set the `DOCKER_GID` environment variable in your shell configuration file or terminal session.


**Note**: Please submit a PR to this document if you find any issues or have additional tips for developing on macOS with the Sail Operator project.
