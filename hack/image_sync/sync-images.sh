#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CONFIG_FILE="${CONFIG_FILE:-hack/image_sync/image-sync-config.json}"
REGISTRY_UPSTREAM="${REGISTRY_UPSTREAM:-docker.io/istio}"
REGISTRY_TARGET="${REGISTRY_TARGET:-quay.io/sail-dev}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored messages
print_info() {
    echo -e "${BLUE}Info: $1${NC}"
}

print_success() {
    echo -e "${GREEN}Success: $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}Warning: $1${NC}"
}

print_error() {
    echo -e "${RED}Error: $1${NC}"
}

# Function to check if required tools are installed
check_dependencies() {
    print_info "Checking dependencies..."
    
    local missing_deps=()
    
    if ! command -v crane &> /dev/null; then
        missing_deps+=("crane")
    fi
    
    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        echo "Please install the missing dependencies:"
        echo "  - crane: https://github.com/google/go-containerregistry/tree/main/cmd/crane"
        echo "  - jq: https://stedolan.github.io/jq/"
        exit 1
    fi
    
    print_success "All dependencies are installed"
}

# Function to validate configuration file
validate_config() {
    print_info "Validating configuration file: $CONFIG_FILE"
    
    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_error "Configuration file not found: $CONFIG_FILE"
        exit 1
    fi
    
    if ! jq empty "$CONFIG_FILE" 2>/dev/null; then
        print_error "Invalid JSON in configuration file: $CONFIG_FILE"
        exit 1
    fi
    
    print_success "Configuration file is valid"
}

# Function to list configured images
list_images() {
    print_info "Configured images:"
    echo
    
    jq -r '.images[] | "  Package: \(.name) (tags: \(.tags | join(", ")))\n      \(.upstream) -> \(.target)"' "$CONFIG_FILE"
}

# Function to check if an image exists
check_image() {
    local image="$1"
    if crane manifest "$image" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to get image digest
get_digest() {
    local image="$1"
    crane digest "$image" 2>/dev/null || echo "unknown"
}

# Function to check status of all configured images
check_status() {
    print_info "Checking status of configured images..."
    echo
    
    while read -r encoded_image; do
        image_config=$(echo "$encoded_image" | base64 -d)
        
        name=$(echo "$image_config" | jq -r '.name')
        upstream=$(echo "$image_config" | jq -r '.upstream')
        target=$(echo "$image_config" | jq -r '.target')
        tags=$(echo "$image_config" | jq -r '.tags[]')
        
        echo -e "${BLUE}Package: $name${NC}"
        
        while IFS= read -r tag; do
            if [[ -n "$tag" ]]; then
                upstream_full="${upstream}:${tag}"
                target_full="${target}:${tag}"
                
                echo -n "  Tag: $tag: "
                
                if check_image "$upstream_full"; then
                    if check_image "$target_full"; then
                        upstream_digest=$(get_digest "$upstream_full")
                        target_digest=$(get_digest "$target_full")
                        
                        if [[ "$upstream_digest" == "$target_digest" ]]; then
                            echo -e "${GREEN}Synced${NC}"
                        else
                            echo -e "${YELLOW}Outdated${NC}"
                        fi
                    else
                        echo -e "${RED}Missing${NC}"
                    fi
                else
                    echo -e "${RED}Upstream not found${NC}"
                fi
            fi
        done <<< "$tags"
        
        echo
    done < <(jq -r '.images[] | @base64' "$CONFIG_FILE")
}

# Function to sync a specific image
sync_image() {
    local image_name="$1"
    local tag="${2:-all}"
    
    print_info "Syncing image: $image_name (tag: $tag)"
    
    # Find the image configuration
    image_config=$(jq -r --arg name "$image_name" '.images[] | select(.name == $name) | @base64' "$CONFIG_FILE")
    
    if [[ -z "$image_config" ]]; then
        print_error "Image not found in configuration: $image_name"
        return 1
    fi
    
    image_data=$(echo "$image_config" | base64 -d)
    upstream=$(echo "$image_data" | jq -r '.upstream')
    target=$(echo "$image_data" | jq -r '.target')
    
    if [[ "$tag" == "all" ]]; then
        tags=$(echo "$image_data" | jq -r '.tags[]')
    else
        tags="$tag"
    fi
    
    local sync_errors=0
    
    while IFS= read -r current_tag; do
        if [[ -n "$current_tag" ]]; then
            upstream_full="${upstream}:${current_tag}"
            target_full="${target}:${current_tag}"
            
            print_info "Syncing $upstream_full -> $target_full"
            
            if check_image "$upstream_full"; then
                if crane copy "$upstream_full" "$target_full" 2>/dev/null; then
                    print_success "Successfully synced $current_tag"
                else
                    print_error "Failed to sync $current_tag"
                    ((sync_errors++))
                fi
            else
                print_error "Upstream image not found: $upstream_full"
                ((sync_errors++))
            fi
        fi
    done <<< "$tags"
    
    # Return 0 for success, 1 for any failures
    if [[ $sync_errors -gt 0 ]]; then
        return 1
    else
        return 0
    fi
}

# Function to sync all configured images
sync_all() {
    print_info "Syncing all configured images..."
    echo
    
    local sync_errors=0
    local sync_success=0
    local total_synced=0
    
    # Read images into array using process substitution to avoid subshell issues
    readarray -t image_configs < <(jq -r '.images[] | @base64' "$CONFIG_FILE")
    
    # Process each image
    for encoded_image in "${image_configs[@]}"; do
        image_config=$(echo "$encoded_image" | base64 -d)
        
        name=$(echo "$image_config" | jq -r '.name')
        upstream=$(echo "$image_config" | jq -r '.upstream')
        target=$(echo "$image_config" | jq -r '.target')
        
        echo -e "${BLUE}Package: $name${NC}"
        
        # Read tags into array to avoid subshell issues
        readarray -t tags < <(echo "$image_config" | jq -r '.tags[]')
        
        # Process each tag for this image
        for tag in "${tags[@]}"; do
            if [[ -n "$tag" ]]; then
                upstream_full="${upstream}:${tag}"
                target_full="${target}:${tag}"
                
                print_info "  Syncing tag: $tag ($upstream_full -> $target_full)"
                ((total_synced++))
                
                if check_image "$upstream_full"; then
                    # Use proper error handling for crane copy
                    if crane copy "$upstream_full" "$target_full" 2>/dev/null; then
                        print_success "    Successfully synced $tag"
                        ((sync_success++))
                    else
                        print_error "    Failed to sync $tag"
                        ((sync_errors++))
                    fi
                else
                    print_error "    Upstream image not found: $upstream_full"
                    ((sync_errors++))
                fi
            fi
        done
        
        print_success "Package $name processing completed"
        echo
    done
    
    echo "==================== SYNC SUMMARY ===================="
    local total_images=$(jq -r '.images | length' "$CONFIG_FILE")
    local total_tags=$(jq -r '[.images[].tags | length] | add' "$CONFIG_FILE")
    print_info "Total images processed: $total_images"
    print_info "Total tags processed: $total_tags"
    print_info "Successfully synced: $sync_success/$total_synced"
    
    if [[ $sync_errors -gt 0 ]]; then
        print_warning "$sync_errors sync operations failed"
        print_success "Sync operation completed with some failures"
        echo "======================================================"
        # Return error count capped at 1 for shell return code
        return 1
    else
        print_success "Sync operation completed successfully"
        echo "======================================================"
        return 0
    fi
}

# Function to perform a dry run of sync (shows what would be synced without actually doing it)
dry_run() {
    print_info "Dry run: showing what would be synced (no actual sync performed)..."
    echo
    
    while read -r encoded_image; do
        image_config=$(echo "$encoded_image" | base64 -d)
        
        name=$(echo "$image_config" | jq -r '.name')
        upstream=$(echo "$image_config" | jq -r '.upstream')
        target=$(echo "$image_config" | jq -r '.target')
        
        echo -e "${BLUE}Would process package: $name${NC}"
        
        echo "$image_config" | jq -r '.tags[]' | while read -r tag; do
            if [[ -n "$tag" ]]; then
                upstream_full="${upstream}:${tag}"
                target_full="${target}:${tag}"
                
                echo "  Would sync: $upstream_full -> $target_full"
            fi
        done
        
        echo
    done < <(jq -r '.images[] | @base64' "$CONFIG_FILE")
    
    echo "==================== DRY RUN SUMMARY ===================="
    local total_images=$(jq -r '.images | length' "$CONFIG_FILE")
    local total_tags=$(jq -r '[.images[].tags | length] | add' "$CONFIG_FILE")
    echo "Info: Total packages that would be processed: $total_images"
    echo "Info: Total tags that would be processed: $total_tags"
    echo "Warning: This was a dry run - no actual sync was performed"
    echo "========================================================"
}

# Function to discover new tags
discover_tags() {
    local image_name="$1"
    local max_tags="${2:-10}"
    
    print_info "Discovering new tags for: $image_name (max: $max_tags)"
    
    # Find the image configuration
    image_config=$(jq -r --arg name "$image_name" '.images[] | select(.name == $name) | @base64' "$CONFIG_FILE")
    
    if [[ -z "$image_config" ]]; then
        print_error "Image not found in configuration: $image_name"
        exit 1
    fi
    
    image_data=$(echo "$image_config" | base64 -d)
    upstream=$(echo "$image_data" | jq -r '.upstream')
    known_tags=$(echo "$image_data" | jq -r '.tags[]')
    
    print_info "Getting tags from upstream: $upstream"
    
    # Get all tags from upstream
    upstream_tags=$(crane ls "$upstream" 2>/dev/null | head -n "$max_tags" || echo "")
    
    if [[ -n "$upstream_tags" ]]; then
        echo "Available upstream tags:"
        echo "$upstream_tags" | sed 's/^/  /'
        
        echo
        echo "New tags (not in configuration):"
        while IFS= read -r tag; do
            if [[ -n "$tag" ]]; then
                if ! echo "$known_tags" | grep -q "^${tag}$"; then
                    echo "  New tag found: $tag"
                fi
            fi
        done <<< "$upstream_tags"
    else
        print_warning "Could not list tags for $upstream"
    fi
}

# Function to update a tag in the config file
update_config_tag() {
    local image_name="$1"
    local old_tag="$2"
    local new_tag="$3"
    
    print_info "Updating $image_name: $old_tag → $new_tag in config"
    
    # Check if config file is writable
    if [[ ! -w "$CONFIG_FILE" ]]; then
        print_warning "Config file is not writable: $CONFIG_FILE"
        print_warning "Skipping config update. Please run with appropriate permissions or update manually."
        return 1
    fi
    
    # Update the tag in the JSON config
    local temp_file="${CONFIG_FILE}.tmp.$$"
    if jq --arg name "$image_name" --arg old_tag "$old_tag" --arg new_tag "$new_tag" '
        .images |= map(
            if .name == $name then
                .tags |= map(if . == $old_tag then $new_tag else . end)
            else . end
        )
    ' "$CONFIG_FILE" > "$temp_file" && mv "$temp_file" "$CONFIG_FILE"; then
        true  # success
    else
        print_error "Failed to update config file"
        rm -f "$temp_file"
        return 1
    fi
    
    print_success "Updated config: $image_name $old_tag → $new_tag"
}

# Function to update image references in sample files
update_sample_files() {
    local image_name="$1"
    local old_tag="$2"
    local new_tag="$3"
    
    # Extract the target image path from config
    local target_image=$(jq -r --arg name "$image_name" '.images[] | select(.name == $name) | .target' "$CONFIG_FILE")
    
    if [[ -z "$target_image" ]]; then
        print_error "Could not find target image for $image_name"
        return 1
    fi
    
    local old_image_ref="${target_image}:${old_tag}"
    local new_image_ref="${target_image}:${new_tag}"
    
    print_info "Updating sample files: $old_image_ref → $new_image_ref"
    
    # Check if samples directory exists
    if [[ ! -d "samples/" ]]; then
        print_warning "samples/ directory not found, skipping sample file updates"
        return 0
    fi
    
    local update_count=0
    local error_count=0
    
    # Find YAML files and store in array to avoid subshell issues
    readarray -t yaml_files < <(find samples/ -name "*.yaml" -type f 2>/dev/null)
    
    # Update each file
    for file in "${yaml_files[@]}"; do
        if [[ -n "$file" && -f "$file" ]]; then
            if grep -q "$old_image_ref" "$file" 2>/dev/null; then
                print_info "  Updating $file"
                if [[ -w "$file" ]]; then
                    if sed -i "s|$old_image_ref|$new_image_ref|g" "$file" 2>/dev/null; then
                        ((update_count++))
                    else
                        print_warning "  Failed to update $file (sed error)"
                        ((error_count++))
                    fi
                else
                    print_warning "  Skipping $file (not writable)"
                    ((error_count++))
                fi
            fi
        fi
    done
    
    if [[ $update_count -gt 0 ]]; then
        print_success "Updated sample files: $old_tag → $new_tag ($update_count files updated)"
    elif [[ $error_count -gt 0 ]]; then
        print_warning "Some sample files could not be updated ($error_count files had issues)"
    else
        print_info "No sample files needed updating for $old_tag → $new_tag"
    fi
    
    # Return success even if some files couldn't be updated - this is not a critical failure
    return 0
}

# Function to extract image tags from Istio upstream samples
extract_istio_tags() {
    local temp_dir=$(mktemp -d)
    local istio_sample_urls=(
        "https://raw.githubusercontent.com/istio/istio/master/samples/helloworld/helloworld.yaml"
        "https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml"
        "https://raw.githubusercontent.com/istio/istio/master/samples/tcp-echo/tcp-echo-ipv4.yaml"
    )
    
    print_info "Fetching Istio upstream samples to extract official tags..."
    
    declare -A istio_tags
    
    for url in "${istio_sample_urls[@]}"; do
        local filename=$(basename "$url")
        local filepath="$temp_dir/$filename"
        
        print_info "  Fetching $filename"
        if curl -sSfL "$url" -o "$filepath"; then
            # Extract image references and their tags
            while IFS= read -r line; do
                if [[ "$line" =~ image:[[:space:]]*docker\.io/istio/([^:]+):([^[:space:]]+) ]]; then
                    local image_name="${BASH_REMATCH[1]}"
                    local tag="${BASH_REMATCH[2]}"
                    istio_tags["$image_name"]="$tag"
                    print_info "    Found: docker.io/istio/$image_name:$tag"
                elif [[ "$line" =~ image:[[:space:]]*([^:]+/go-httpbin):([^[:space:]]+) ]]; then
                    local image_base="${BASH_REMATCH[1]}"
                    local tag="${BASH_REMATCH[2]}"
                    istio_tags["httpbin"]="$tag"
                    print_info "    Found: $image_base:$tag"
                fi
            done < "$filepath"
        else
            print_warning "  Failed to fetch $filename"
        fi
    done
    
    rm -rf "$temp_dir"
    
    # Store results in global array
    for key in "${!istio_tags[@]}"; do
        echo "$key=${istio_tags[$key]}"
    done
}

# Function to map our image names to upstream names
map_our_name_to_istio() {
    local our_name="$1"
    
    case "$our_name" in
        "examples-helloworld-v1")
            echo "examples-helloworld-v1"
            ;;
        "examples-helloworld-v2")
            echo "examples-helloworld-v2"
            ;;
        "examples-httpbin")
            echo "httpbin"
            ;;
        "examples-tcp-echo-server")
            echo "tcp-echo-server"
            ;;
        *)
            echo "$our_name"
            ;;
    esac
}

# Function to check for updates from Istio upstream and apply them
auto_update() {
    local update_count=0
    local updates_found=()
    
    print_info "Syncing tags with upstream Istio samples..."
    echo
    
    # Extract tags from Istio upstream samples
    local istio_tags_output=$(extract_istio_tags)
    
    if [[ -z "$istio_tags_output" ]]; then
        print_error "Could not extract tags from Istio upstream samples"
        return 1
    fi
    
    # Parse istio tags into associative array
    declare -A istio_official_tags
    while IFS='=' read -r key value; do
        if [[ -n "$key" && -n "$value" ]]; then
            istio_official_tags["$key"]="$value"
        fi
    done <<< "$istio_tags_output"
    
    echo
    print_info "Comparing with our current configuration..."
    
    # First pass: Find all updates required and store them in an array.
    while read -r encoded_image; do
        image_config=$(echo "$encoded_image" | base64 -d)
        
        name=$(echo "$image_config" | jq -r '.name')
        
        echo -e "${BLUE}Checking: $name${NC}"
        
        # Map our name to Istio's naming convention
        istio_name=$(map_our_name_to_istio "$name")
        
        if [[ -n "${istio_official_tags[$istio_name]}" ]]; then
            local istio_tag="${istio_official_tags[$istio_name]}"
            print_info "  Istio uses: $istio_tag"
            
            readarray -t tag_array < <(echo "$image_config" | jq -r '.tags[]')
            
            for current_tag in "${tag_array[@]}"; do
                if [[ -n "$current_tag" && "$current_tag" != "latest" ]]; then
                    if [[ "$current_tag" != "$istio_tag" ]]; then
                        print_warning "  Update needed: $current_tag → $istio_tag"
                        updates_found+=("$name" "$current_tag" "$istio_tag")
                    else
                        print_info "  Tag $current_tag matches Istio"
                    fi
                fi
            done
        else
            print_warning "  No corresponding Istio sample found for $name"
        fi
        echo
    done < <(jq -r '.images[] | @base64' "$CONFIG_FILE")
    
    # Second pass: Apply all the updates found in the first pass
    if [[ ${#updates_found[@]} -gt 0 ]]; then
        print_info "Applying all found updates..."
        
        for ((i=0; i<${#updates_found[@]}; i+=3)); do
            local name="${updates_found[i]}"
            local old_tag="${updates_found[i+1]}"
            local new_tag="${updates_found[i+2]}"
            
            if update_config_tag "$name" "$old_tag" "$new_tag"; then
                if update_sample_files "$name" "$old_tag" "$new_tag"; then
                    ((update_count++))
                else
                    print_warning "  Sample file update had issues (non-critical)"
                fi
            else
                print_error "  Failed to update config for $name"
                return 1
            fi
        done
        
        print_success "$update_count updates completed! Tags now match Istio upstream."
        echo "Configuration file changes saved."
    else
        print_info "All images already match Istio upstream"
    fi
    
    return 0
}

# Function to run full auto-update workflow
auto_sync_with_update() {
    print_info "Starting auto-update and sync workflow..."
    echo
    
    # Step 1: Check for updates and apply them
    if ! auto_update; then
        print_error "The auto-update process failed."
        return 1
    fi
    
    # Step 2: Sync all images (including newly updated ones)
    echo
    print_info "Syncing all images (including any updates)..."
    
    if ! sync_all; then
        print_warning "Auto-update and sync workflow completed with some sync failures!"
        print_info "Check the sync summary above for details on any failed operations."
        return 1
    fi
    
    print_success "Auto-update and sync workflow completed successfully!"
    return 0
}

# Function to show help
show_help() {
    echo "Image Synchronization Utility Script"
    echo
    echo "Usage: $0 <command> [arguments]"
    echo
    echo "Commands:"
    echo "  check-deps           Check if required dependencies are installed"
    echo "  validate             Validate configuration file"
    echo "  list                 List configured images"
    echo "  status               Check status of all configured images"
    echo "  sync-all             Sync all configured images automatically"
    echo "  auto-update          Sync tags with upstream Istio samples and update config + samples"
    echo "  auto-sync            Full workflow: auto-update + sync"
    echo "  test-extract         Test extraction of tags from Istio upstream (no updates)"
    echo "  dry-run              Show what would be synced (fast, no network calls)"
    echo "  sync <image> [tag]   Sync specific image (all tags or specific tag)"
    echo "  discover <image> [max] Discover new tags for an image"
    echo "  help                 Show this help message"
    echo
    echo "Environment variables:"
    echo "  CONFIG_FILE          Path to configuration file (default: .github/image-sync-config.json)"
    echo "  REGISTRY_UPSTREAM    Upstream registry (default: docker.io/istio)"
    echo "  REGISTRY_TARGET      Target registry (default: quay.io/sail-dev)"
    echo
    echo "Examples:"
    echo "  $0 status"
    echo "  $0 dry-run"
    echo "  $0 test-extract       # Test fetching Istio upstream tags"
    echo "  $0 auto-update        # Sync with Istio upstream tags"
    echo "  $0 auto-sync          # Auto-update + sync everything"
    echo "  $0 sync-all"
    echo "  $0 sync examples-helloworld-v1"
    echo "  $0 sync examples-helloworld-v1 1.0"
    echo "  $0 discover examples-httpbin 20"
}

# Main script logic
main() {
    # Check for dependencies first as it's a common point of failure
    check_dependencies
    
    local command="$1"
    local command_result=0
    
    case "${command:-help}" in
        "check-deps")
            # Already checked above
            ;;
        "validate")
            validate_config
            ;;
        "list")
            validate_config
            list_images
            ;;
        "status")
            validate_config
            check_status
            ;;
        "sync-all")
            validate_config
            if ! sync_all; then
                command_result=1
            fi
            ;;
        "auto-update")
            validate_config
            if ! auto_update; then
                command_result=1
            fi
            ;;
        "auto-sync")
            validate_config
            if ! auto_sync_with_update; then
                command_result=1
            fi
            ;;
        "test-extract")
            echo "Testing extraction of tags from Istio upstream samples..."
            extract_istio_tags
            ;;
        "dry-run")
            validate_config
            dry_run
            ;;
        "sync")
            if [[ -z "$2" ]]; then
                print_error "Image name is required"
                echo "Usage: $0 sync <image> [tag]"
                command_result=1
            else
                validate_config
                if ! sync_image "$2" "$3"; then
                    command_result=1
                fi
            fi
            ;;
        "discover")
            if [[ -z "$2" ]]; then
                print_error "Image name is required"
                echo "Usage: $0 discover <image> [max_tags]"
                command_result=1
            else
                validate_config
                discover_tags "$2" "$3"
            fi
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            print_error "Unknown command: $1"
            echo
            show_help
            command_result=1
            ;;
    esac
    
    # Exit with the result of the command
    exit $command_result
}

# Run main function with all arguments
main "$@"