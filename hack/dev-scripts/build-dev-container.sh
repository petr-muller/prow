#!/bin/bash
# Build script for the Prow development container
# Uses Podman to build a development-focused container image

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
CONTAINER_NAME=${CONTAINER_NAME:-"prow-hook-dev"}
CONTAINER_TAG=${CONTAINER_TAG:-"latest"}
CONTAINERFILE=${CONTAINERFILE:-"hack/Containerfile.dev"}
BUILD_CONTEXT=${BUILD_CONTEXT:-"."}

echo -e "${PURPLE}🏗️  Building Prow Development Container${NC}"
echo "========================================"

# Check if Podman is available
check_podman() {
    echo -e "${BLUE}🔍 Checking for Podman...${NC}"
    
    if ! command -v podman &> /dev/null; then
        echo -e "${RED}❌ Podman not found${NC}"
        echo -e "${YELLOW}💡 Please install Podman:${NC}"
        echo "   - Fedora/RHEL: sudo dnf install podman"
        echo "   - Ubuntu: sudo apt install podman"
        echo "   - macOS: brew install podman"
        echo "   - Or see: https://podman.io/getting-started/installation"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Podman found: $(podman --version)${NC}"
}

# Check if Containerfile exists
check_containerfile() {
    echo -e "${BLUE}📋 Checking Containerfile...${NC}"
    
    if [[ ! -f "$CONTAINERFILE" ]]; then
        echo -e "${RED}❌ Containerfile not found: $CONTAINERFILE${NC}"
        echo -e "${YELLOW}💡 Make sure you're running from the Prow repository root${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Containerfile found: $CONTAINERFILE${NC}"
}

# Build the container
build_container() {
    echo -e "${BLUE}🏗️  Building container image...${NC}"
    echo -e "${CYAN}   Image: ${CONTAINER_NAME}:${CONTAINER_TAG}${NC}"
    echo -e "${CYAN}   Context: ${BUILD_CONTEXT}${NC}"
    echo -e "${CYAN}   Containerfile: ${CONTAINERFILE}${NC}"
    echo
    
    # Build command with progress and detailed output
    local build_args=(
        "build"
        "--file" "$CONTAINERFILE"
        "--tag" "${CONTAINER_NAME}:${CONTAINER_TAG}"
        "--target" "development"
        "--progress" "plain"
        "$BUILD_CONTEXT"
    )
    
    echo -e "${YELLOW}📦 Running: podman ${build_args[*]}${NC}"
    echo
    
    if podman "${build_args[@]}"; then
        echo
        echo -e "${GREEN}✅ Container built successfully!${NC}"
    else
        echo
        echo -e "${RED}❌ Container build failed${NC}"
        exit 1
    fi
}

# Show build results
show_results() {
    echo
    echo -e "${PURPLE}🎉 Build Complete!${NC}"
    echo "===================="
    echo
    
    # Show image details
    echo -e "${CYAN}📊 Image Details:${NC}"
    podman images "${CONTAINER_NAME}:${CONTAINER_TAG}" --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.Created}}"
    echo
    
    echo -e "${CYAN}🚀 Quick Start Commands:${NC}"
    echo
    echo -e "${YELLOW}# Run development environment:${NC}"
    echo "podman run -it --rm \\"
    echo "  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \\"
    echo "  -v \$(pwd):/workspace:Z \\"
    echo "  -v \$HOME/.gitconfig:/home/prowdev/.gitconfig:ro \\"
    echo "  ${CONTAINER_NAME}:${CONTAINER_TAG}"
    echo
    echo -e "${YELLOW}# Run with GitHub token (recommended):${NC}"
    echo "podman run -it --rm \\"
    echo "  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \\"
    echo "  -v \$(pwd):/workspace:Z \\"
    echo "  -v \$HOME/.gitconfig:/home/prowdev/.gitconfig:ro \\"
    echo "  -v /path/to/github-token:/etc/github/token:ro \\"
    echo "  ${CONTAINER_NAME}:${CONTAINER_TAG}"
    echo
    echo -e "${YELLOW}# Interactive development shell:${NC}"
    echo "podman run -it --rm \\"
    echo "  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \\"
    echo "  -v \$(pwd):/workspace:Z \\"
    echo "  --entrypoint /bin/bash \\"
    echo "  ${CONTAINER_NAME}:${CONTAINER_TAG}"
    echo
    echo -e "${CYAN}📋 Port Mapping:${NC}"
    echo "  8888 - GitHub API proxy (ghproxy-dev)"
    echo "  8889 - Hook component" 
    echo "  8890 - Metrics endpoint"
    echo "  8891 - Health checks"
    echo
    echo -e "${CYAN}📁 Volume Mounts:${NC}"
    echo "  /workspace       - Your Prow source code (with live reload)"
    echo "  /etc/github/token - GitHub API token (optional but recommended)"
    echo "  /etc/webhook/hmac - Webhook secret (auto-generated if not provided)"
    echo
    echo -e "${CYAN}🧪 Testing:${NC}"
    echo "Once the container is running, test the shrug plugin:"
    echo "  ./hack/dev-scripts/test-shrug-plugin.sh"
    echo
}

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up build artifacts...${NC}"
    # Podman automatically cleans up intermediate images
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

# Set up signal handlers
trap cleanup EXIT

# Main execution
main() {
    echo -e "${BLUE}🚀 Starting container build process...${NC}"
    echo
    
    check_podman
    check_containerfile
    echo
    build_container
    show_results
    
    echo -e "${GREEN}🎉 Development container is ready for use!${NC}"
}

# Show usage if help requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Usage: $0 [options]"
    echo
    echo "Build a development container for Prow Hook component development."
    echo
    echo "Environment Variables:"
    echo "  CONTAINER_NAME     Container name (default: prow-hook-dev)"
    echo "  CONTAINER_TAG      Container tag (default: latest)"  
    echo "  CONTAINERFILE      Path to Containerfile (default: hack/Containerfile.dev)"
    echo "  BUILD_CONTEXT      Build context (default: .)"
    echo
    echo "Examples:"
    echo "  # Basic build:"
    echo "  $0"
    echo
    echo "  # Custom name and tag:"
    echo "  CONTAINER_NAME=my-prow-dev CONTAINER_TAG=v1.0 $0"
    echo
    exit 0
fi

# Run main function
main "$@"