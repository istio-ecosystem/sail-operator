# Image Synchronization System

This system automates the synchronization of sample container images from the Istio upstream registry (`docker.io/istio`) to our Quay.io registry (`quay.io/sail-dev`).

## Overview

The image synchronization system provides:

- **Scheduled synchronization**: Automatically runs daily at 2:00 AM UTC
- **Change detection**: Compares image digests to detect updates
- **New tag discovery**: Automatically discovers new versions in upstream registries
- **Automated copying**: Uses `crane` for efficient image copying
- **Manual control**: Utility script for manual operations

## Quick Start

### 1. Setup GitHub Secrets

We will use repository secret to store credentials for Quay.io:
*  QUAY_USER: for the Quay.io username
*  QUAY_PDW: for the Quay.io password

### 2. Configure Images

Images are configured in `.github/image-sync-config.json`:

```json
{
  "images": [
    {
      "name": "examples-helloworld-v1",
      "upstream": "docker.io/istio/examples-helloworld-v1",
      "target": "quay.io/sail-dev/examples-helloworld-v1",
      "tags": ["1.0", "latest"]
    }
  ],
  "check_new_tags": true,
  "max_tags_to_check": 10
}
```

### 3. Run Synchronization

**Automatic:** The workflow runs daily according to the configured schedule.

**Manual:** Go to GitHub Actions → "Sync Images from Upstream" → "Run workflow"

## Files Structure

```
.github/
├── workflows/
│   └── sync-images.yml          # GitHub Actions workflow
├── image-sync-config.json       # Image configuration
scripts/
└── sync-images.sh              # Utility script for manual operations
docs/
└── IMAGE_SYNC.md               # Detailed documentation (Spanish)
```

## Configuration Reference

### Image Configuration

Each image in the configuration file has the following structure:

```json
{
  "name": "descriptive-name",
  "upstream": "docker.io/istio/upstream-image",
  "target": "quay.io/sail-dev/target-image",
  "tags": ["tag1", "tag2", "latest"]
}
```

### Configuration Options

- **`check_new_tags`**: Boolean to enable/disable new tag discovery
- **`max_tags_to_check`**: Maximum number of tags to check for new versions

## Currently Synchronized Images

- **examples-helloworld-v1**: HelloWorld example application v1
- **examples-helloworld-v2**: HelloWorld example application v2  
- **examples-httpbin**: HTTPBin example application
- **examples-tcp-echo-server**: TCP echo server

## Manual Operations

The utility script `scripts/sync-images.sh` provides several commands:

```bash
# Check system dependencies
./scripts/sync-images.sh check-deps

# Validate configuration
./scripts/sync-images.sh validate

# List configured images
./scripts/sync-images.sh list

# Check status of all images
./scripts/sync-images.sh status

# Sync a specific image
./scripts/sync-images.sh sync examples-helloworld-v1

# Sync a specific tag
./scripts/sync-images.sh sync examples-helloworld-v1 1.0

# Discover new tags for an image
./scripts/sync-images.sh discover examples-httpbin 20
```

## Adding New Images

1. Edit `.github/image-sync-config.json`
2. Add the new image configuration:

```json
{
  "name": "new-image-name",
  "upstream": "docker.io/istio/new-image",
  "target": "quay.io/sail-dev/new-image",
  "tags": ["1.0", "latest"]
}
```

3. Commit and push changes
4. The image will be synchronized in the next run

### Logs

Detailed logs are available in:
- GitHub Actions → "Sync Images from Upstream" → Latest execution

## Troubleshooting

### Authentication Failure

**Error:** `unauthorized: authentication required`

**Solution:** Verify that `QUAY_USERNAME` and `QUAY_PASSWORD` secrets are correctly configured.

### Image Not Found

**Error:** `MANIFEST_UNKNOWN` or `NOT_FOUND`

**Solution:**
1. Verify the upstream image exists in `docker.io/istio/`
2. Check that the image name is correct
3. Confirm the specified tag exists

### Rate Limiting

**Error:** `too many requests`

**Solution:** The system includes automatic retries, but you can adjust `max_tags_to_check` to reduce queries.

## Dependencies

- **[crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane)**: Container image manipulation tool
- **[jq](https://stedolan.github.io/jq/)**: Command-line JSON processor

## License

This project is licensed under the same terms as the sail-operator project.