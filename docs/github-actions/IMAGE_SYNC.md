# Image Sync Documentation

This script automatically syncs sample container images to match the exact tags used in upstream Istio samples, ensuring compatibility with official Istio examples.

## Quick Start

### 1. Automatic Sync (Recommended)
It's runs with the GitHub Actions workflow, which automatically updates and syncs images based on the latest tags from Istio samples. This is incorporated inside the the update dependency workflow, so it will run automatically when you update dependencies. For more information check `.github/workflows/update-deps.yml`.

### 2. Manual Operations
```bash
# Auto-update to newer tags + sync (recommended)
make auto-sync-sample-images

# Just auto-update tags (no sync)
make auto-update-sample-images
```

## Configuration

Images are configured in `hack/image_sync/image-sync-config.json`:

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

## Available Commands

### Make Targets
```bash
make auto-sync-sample-images      # Auto-update + sync (recommended)
make auto-update-sample-images    # Check for newer tags and update config + samples
```

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
