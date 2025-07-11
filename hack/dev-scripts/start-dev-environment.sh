#!/bin/bash
# Development environment startup script for Prow Hook development
# This script sets up a complete development environment with:
# - GitHub proxy with write-blocking
# - Hook component with minimal plugins
# - Hot reloading capabilities
# - Enhanced development logging

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
GHPROXY_PORT=${GHPROXY_PORT:-8888}
HOOK_PORT=${HOOK_PORT:-8889}
METRICS_PORT=${METRICS_PORT:-8890}
HEALTH_PORT=${HEALTH_PORT:-8891}

# GitHub configuration
GITHUB_TOKEN_FILE=${GITHUB_TOKEN_FILE:-"/tmp/github-token"}
WEBHOOK_SECRET_FILE=${WEBHOOK_SECRET_FILE:-"/tmp/webhook-secret"}

# Development configuration
DEV_CONFIG_DIR="/tmp/prow-dev"
DEV_CACHE_DIR="/tmp/ghproxy-dev"
DEV_LOGS_DIR="/tmp/logs"

echo -e "${PURPLE}🚀 Starting Prow Development Environment${NC}"
echo "=================================================="

# Create necessary directories
mkdir -p "${DEV_CONFIG_DIR}" "${DEV_CACHE_DIR}" "${DEV_LOGS_DIR}"

# Check for required tokens/secrets
check_secrets() {
    echo -e "${BLUE}🔐 Checking secrets and tokens...${NC}"
    
    if [[ ! -f "${GITHUB_TOKEN_FILE}" ]]; then
        echo -e "${YELLOW}⚠️  GitHub token not found at ${GITHUB_TOKEN_FILE}${NC}"
        echo "   Creating a development placeholder token file"
        # Create a token that looks more realistic to avoid GitHub 400 errors
        echo "ghp_development_placeholder_token_1234567890abcdef" > "${GITHUB_TOKEN_FILE}"
        echo -e "${YELLOW}   ⚠️  You'll need to mount a real GitHub token for full functionality${NC}"
        echo -e "${YELLOW}   ℹ️  Using placeholder token - GitHub API calls will fail but hook will start${NC}"
    fi
    
    if [[ ! -f "${WEBHOOK_SECRET_FILE}" ]]; then
        echo -e "${YELLOW}⚠️  Webhook secret not found at ${WEBHOOK_SECRET_FILE}${NC}"
        echo "   Creating a development webhook secret"
        echo "development-webhook-secret" > "${WEBHOOK_SECRET_FILE}"
    fi
}

# Generate minimal development configuration
generate_dev_config() {
    echo -e "${BLUE}📝 Generating development configuration...${NC}"
    
    # Create minimal Prow config focused on shrug plugin
    cat > "${DEV_CONFIG_DIR}/config.yaml" << 'EOF'
# Minimal Prow configuration for development
# Focuses on the shrug plugin for simple testing

plank:
  job_url_template: 'http://localhost:8889/view/log/{{.Spec.Job}}/{{.Status.BuildID}}'
  default_decoration_configs:
    "*":
      gcs_configuration:
        bucket: "dev-prow-logs"  # Not used in development
        path_strategy: "explicit"
      s3_credentials_secret: ""  # Not used in development

deck:
  spyglass:
    lenses: []
  rerun_auth_configs: {}

prowjob_namespace: default
pod_namespace: default

# Minimal tide configuration (not used in hook development)
tide:
  queries: []
EOF

    # Create plugin configuration with just the shrug plugin
    cat > "${DEV_CONFIG_DIR}/plugins.yaml" << 'EOF'
# Development plugin configuration
# Enables only the shrug plugin for simple testing

plugins:
  # Global plugins (apply to all repos)
  "": []
  
  # Example configuration for testing
  # Replace with your target org/repo for development
  "kubernetes/test-infra":
    - shrug
  
  # Development testing repo
  "octocat/Hello-World":
    - shrug

# Plugin-specific configuration
shrug: {}

# External plugin configuration (empty for development)
external_plugins: {}

# Configuration for plugin behaviors
approve: []
lgtm: []
owners: {}
repo_milestone: {}
config_updater:
  maps: {}
plugins_config: {}

# Simplified configuration for development
label: {}
size: {}
welcome: []
heart: {}
EOF

    echo -e "${GREEN}✅ Configuration files generated in ${DEV_CONFIG_DIR}${NC}"
}

# Start GitHub development proxy
start_ghproxy() {
    echo -e "${BLUE}🌐 Starting GitHub development proxy...${NC}"
    
    local log_file="${DEV_LOGS_DIR}/ghproxy-dev.log"
    
    ghproxy-dev \
        --port="${GHPROXY_PORT}" \
        --cache-dir="${DEV_CACHE_DIR}" \
        --cache-sizeGB=1 \
        --development-mode=true \
        --enhanced-logging=true \
        --get-throttling-time-ms=100 \
        --throttling-time-ms=500 \
        --allowed-read-only-orgs="kubernetes,octocat" \
        > "${log_file}" 2>&1 &
    
    local ghproxy_pid=$!
    echo "${ghproxy_pid}" > "${DEV_LOGS_DIR}/ghproxy.pid"
    
    echo -e "${GREEN}✅ GitHub development proxy started (PID: ${ghproxy_pid})${NC}"
    echo -e "   📊 Proxy running on http://localhost:${GHPROXY_PORT}"
    echo -e "   📋 Development help: http://localhost:${GHPROXY_PORT}/dev/help"
    echo -e "   📈 Status endpoint: http://localhost:${GHPROXY_PORT}/dev/status"
    echo -e "   📄 Logs: ${log_file}"
}

# Start hook component
start_hook() {
    echo -e "${BLUE}🪝 Starting Prow Hook component...${NC}"
    
    local log_file="${DEV_LOGS_DIR}/hook.log"
    
    # Wait for ghproxy to be ready
    echo -e "${YELLOW}⏳ Waiting for GitHub proxy to be ready...${NC}"
    for i in {1..30}; do
        if curl -s "http://localhost:${GHPROXY_PORT}/dev/status" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ GitHub proxy is ready${NC}"
            break
        fi
        sleep 1
        echo -n "."
    done
    echo
    
    hook \
        --port="${HOOK_PORT}" \
        --config-path="${DEV_CONFIG_DIR}/config.yaml" \
        --plugin-config="${DEV_CONFIG_DIR}/plugins.yaml" \
        --github-endpoint="http://localhost:${GHPROXY_PORT}" \
        --github-token-path="${GITHUB_TOKEN_FILE}" \
        --hmac-secret-file="${WEBHOOK_SECRET_FILE}" \
        --dry-run=true \
        --health-port="${HEALTH_PORT}" \
        --metrics-port="${METRICS_PORT}" \
        > "${log_file}" 2>&1 &
    
    local hook_pid=$!
    echo "${hook_pid}" > "${DEV_LOGS_DIR}/hook.pid"
    
    echo -e "${GREEN}✅ Hook component started (PID: ${hook_pid})${NC}"
    echo -e "   🪝 Hook endpoint: http://localhost:${HOOK_PORT}/hook"
    echo -e "   ❓ Plugin help: http://localhost:${HOOK_PORT}/plugin-help"
    echo -e "   🏥 Health check: http://localhost:${HEALTH_PORT}/healthz"
    echo -e "   📊 Metrics: http://localhost:${METRICS_PORT}/metrics"
    echo -e "   📄 Logs: ${log_file}"
}

# Display development information
show_dev_info() {
    echo
    echo -e "${PURPLE}🎉 Prow Development Environment Ready!${NC}"
    echo "=============================================="
    echo
    echo -e "${CYAN}🔧 Development URLs:${NC}"
    echo -e "   GitHub Proxy:     http://localhost:${GHPROXY_PORT}"
    echo -e "   Hook Component:   http://localhost:${HOOK_PORT}"
    echo -e "   Plugin Help:      http://localhost:${HOOK_PORT}/plugin-help"
    echo -e "   Metrics:          http://localhost:${METRICS_PORT}/metrics"
    echo -e "   Health:           http://localhost:${HEALTH_PORT}/healthz"
    echo
    echo -e "${CYAN}📋 Development Guide:${NC}"
    echo -e "   1. All GitHub writes are ${RED}BLOCKED${NC} - safe for development"
    echo -e "   2. Enhanced logs show what Prow ${GREEN}would do${NC}"
    echo -e "   3. Test with: ${YELLOW}curl -X POST localhost:${HOOK_PORT}/hook${NC}"
    echo -e "   4. Logs are in: ${BLUE}${DEV_LOGS_DIR}/${NC}"
    echo
    echo -e "${CYAN}🧪 Testing the Shrug Plugin:${NC}"
    echo -e "   Send a webhook with a comment containing '/shrug'"
    echo -e "   Watch the logs to see what Prow would do!"
    echo
    echo -e "${CYAN}📄 Log Files:${NC}"
    echo -e "   GitHub Proxy: ${DEV_LOGS_DIR}/ghproxy-dev.log"
    echo -e "   Hook:         ${DEV_LOGS_DIR}/hook.log"
    echo
    echo -e "${YELLOW}💡 Tip: Use 'tail -f ${DEV_LOGS_DIR}/*.log' to follow all logs${NC}"
    echo
}

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up development environment...${NC}"
    
    # Kill background processes
    if [[ -f "${DEV_LOGS_DIR}/ghproxy.pid" ]]; then
        local ghproxy_pid=$(cat "${DEV_LOGS_DIR}/ghproxy.pid")
        if kill -0 "${ghproxy_pid}" 2>/dev/null; then
            echo -e "   Stopping GitHub proxy (PID: ${ghproxy_pid})"
            kill "${ghproxy_pid}"
        fi
        rm -f "${DEV_LOGS_DIR}/ghproxy.pid"
    fi
    
    if [[ -f "${DEV_LOGS_DIR}/hook.pid" ]]; then
        local hook_pid=$(cat "${DEV_LOGS_DIR}/hook.pid")
        if kill -0 "${hook_pid}" 2>/dev/null; then
            echo -e "   Stopping hook component (PID: ${hook_pid})"
            kill "${hook_pid}"
        fi
        rm -f "${DEV_LOGS_DIR}/hook.pid"
    fi
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

# Set up signal handlers
trap cleanup EXIT INT TERM

# Main execution
main() {
    check_secrets
    generate_dev_config
    start_ghproxy
    sleep 2  # Give ghproxy a moment to start
    start_hook
    sleep 2  # Give hook a moment to start
    show_dev_info
    
    # Keep the script running and show logs
    echo -e "${CYAN}📺 Following development logs (Ctrl+C to stop):${NC}"
    echo "=================================================="
    tail -f "${DEV_LOGS_DIR}"/*.log
}

# Run main function
main "$@"