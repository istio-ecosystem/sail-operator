# Image Sync Documentation

This documentation describes the automatic process for the sync of the sample container images to match the exact tags used in upstream Istio samples, ensuring compatibility with official Istio examples. Across this documentation it will be described the process, configuration, and commands available for managing the image synchronization.

## Overview

### 1. Automatic Sync (Recommended)
It runs with the GitHub Actions workflow, which automatically updates and syncs images based on the latest tags from Istio samples. This is incorporated inside the the update dependency workflow, so it will run automatically when you update dependencies. For more information please check [`update-deps.yml`](../../.github/workflows/update-deps.yaml).

### 2. Manual Operations
You can use the make targets or the provided scripts to manually update or sync images. This is useful for testing or when you need to perform specific operations without waiting for the automatic workflow.

```bash
# Auto-update to newer tags + sync (recommended)
make auto-sync-sample-images

# Just auto-update tags (no sync)
make auto-update-sample-images
```

## Configuration

Images are configured in [`image-sync-config.json`](../../hack/image_sync/image-sync-config.json). This file contains the list of images to be synced, their upstream sources, target locations, and the specific tags to be used.

```json
{
  "images": [
    {
      "name": "examples-helloworld-v1",
      "upstream": "docker.io/istio/examples-helloworld-v1",
      "target": "quay.io/sail-dev/examples-helloworld-v1",
      "tags": ["1.0", "latest"]
    }
  ]
}
```

Take into account that only listed tags will be synced into quay.io/sail-dev, so you can control which tags are available in the Sail Dev registry.

## Setup Requirements

### Dependencies
The system uses:
- `crane` (for image copying)
- `jq` (for JSON processing)

Both are automatically installed by the GitHub workflow.

## Available Commands in the sync script

Besides the make targets, you can use the provided script to manage image synchronization. The script is located at [`hack/image_sync/sync-images.sh`](../../hack/image_sync/sync-images.sh) and provides several commands to manage the image sync process.

### Script Commands
```bash
./hack/image_sync/sync-images.sh <command>

Commands:
  validate              Check configuration file
  list                  Show configured images
  status                Check current sync status
  auto-update           Check for newer tags and update config + samples
  auto-sync             Full workflow: auto-update + sync
  sync-all              Sync all configured images
  dry-run               Show what would be synced
  sync <image>          Sync specific image
  discover  <image>     Discover new tags for an image
  help                  Show all commands
```

## Currently Synced Images

- `examples-helloworld-v1` - HelloWorld example v1
- `examples-helloworld-v2` - HelloWorld example v2
- `examples-httpbin` - HTTPBin testing tool
- `examples-tcp-echo-server` - TCP echo server

## Adding New Images

1. Edit `hack/image_sync/image-sync-config.json`
2. Add your image configuration:
   ```json
   {
     "name": "my-new-image",
     "upstream": "docker.io/istio/my-image",
     "target": "quay.io/sail-dev/my-image", 
     "tags": ["latest", "1.0"]
   }
   ```
3. Commit and push - it will sync automatically on next run

## How It Works

### Auto-Update Workflow (Used on gh-actions)
1. **Fetch Istio Samples**: Downloads official sample YAML files from github.com/istio/istio
2. **Extract Official Tags**: Parses YAML files to find exact image tags Istio uses
3. **Compare Tags**: Compares our config tags with Istio's official tags
4. **Update Config**: Updates `.github/image-sync-config.json` to match Istio exactly
5. **Update Samples**: Updates image references in `samples/` directory to match
6. **Sync Images**: Copy using crane the updated images to Quay.io/sail-dev
