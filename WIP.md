# WIP: Prow Single Component Development Environment

**Status**: 🟡 Partially Complete - Core infrastructure working, needs GitHub token integration fix

## Overview

This work implements a development environment for Prow contributors that allows working on individual components (starting with the Hook component and shrug plugin) without requiring a full Prow deployment. The goal is to lower the barrier to contribution by providing:

- **Safe development**: Reads real GitHub state but blocks all writes
- **Enhanced logging**: Shows exactly what Prow would do
- **Fast iteration**: Containerized environment with hot reloading
- **Minimal setup**: One command to get started

## What's Working ✅

### 1. Enhanced GitHub Proxy (`cmd/ghproxy-dev/main.go`)
- **Write-blocking transport**: Intercepts and blocks all non-GET/HEAD requests
- **Enhanced logging**: Provides detailed logs showing what Prow would do
- **Real GitHub integration**: Reads live GitHub state with caching
- **Development endpoints**: `/dev/status` and `/dev/help` for debugging
- **Prometheus metrics**: Tracks blocked operations

### 2. Containerized Development Environment
- **Podman-based**: Uses `hack/Containerfile.dev` with Go 1.24
- **Development tools**: Includes air, golangci-lint, debugger, etc.
- **Volume mounts**: Live code reloading via `/workspace` mount
- **Port mapping**: 8888 (proxy), 8889 (hook), 8890 (metrics), 8891 (health)

### 3. Development Scripts (`hack/dev-scripts/`)
- **`build-dev-container.sh`**: Builds the development container
- **`start-dev-environment.sh`**: Starts both proxy and hook components
- **`test-shrug-plugin.sh`**: Tests the shrug plugin with webhook simulation

### 4. Documentation
- **Contributor guide**: `site/content/en/docs/contribution-guidelines/single-component-development.md`
- **Technical README**: `hack/README-dev-environment.md`
- **Integration**: Fits into existing Hugo documentation site

## Current Issue ❌

**GitHub Token Authentication**: The hook component requires a valid GitHub token even in dry-run mode because it attempts to authenticate and get the bot name during startup. Currently fails with:

```
Error getting Git client: fetching bot name from GitHub: status code 400
```

**Root Cause**: Hook component calls GitHub `/user` API during initialization, even with `--dry-run=true`.

## Files Created/Modified

### New Files
```
cmd/ghproxy-dev/main.go                     # Enhanced GitHub proxy with write-blocking
hack/Containerfile.dev                     # Development container definition
hack/dev-scripts/build-dev-container.sh    # Container build script
hack/dev-scripts/start-dev-environment.sh  # Environment startup script
hack/dev-scripts/test-shrug-plugin.sh      # Plugin testing script
hack/README-dev-environment.md             # Technical documentation
site/content/en/docs/contribution-guidelines/single-component-development.md  # Contributor guide
```

### Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Test Script   │───▶│  Hook Component  │───▶│  GitHub Proxy   │
│  (webhooks)     │    │  (port 8889)     │    │  (port 8888)    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │     Plugins      │    │  GitHub API     │
                       │   (shrug, etc)   │    │ (reads only)    │
                       └──────────────────┘    └─────────────────┘
```

## Next Steps for Continuation

### Immediate (High Priority)
1. **Fix GitHub token issue**:
   - Option A: Modify hook startup to skip GitHub authentication in development mode
   - Option B: Provide better documentation for real token setup
   - Option C: Mock the GitHub client initialization

2. **Test end-to-end workflow**:
   ```bash
   # With real GitHub token
   echo "your_github_token" > /tmp/github-token
   podman run -d --rm -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
     -v $(pwd):/workspace:Z -v /tmp/github-token:/etc/github/token:ro \
     --entrypoint /workspace/hack/dev-scripts/start-dev-environment.sh \
     --name prow-dev prow-hook-dev:latest
   
   # Test the shrug plugin
   ./hack/dev-scripts/test-shrug-plugin.sh
   ```

### Medium Priority
3. **Enhance development experience**:
   - Add more plugins to default configuration
   - Improve webhook simulation with more realistic payloads
   - Add development-specific log formatting

4. **Extend to other components**:
   - Apply same pattern to Deck, Tide, Crier
   - Create component-specific development configurations

### Low Priority
5. **Documentation improvements**:
   - Add video/screenshots to contributor guide
   - Create troubleshooting section
   - Document plugin development workflow

## Key Insights from Development

### ghproxy/ghcache Integration
- ✅ **Perfect foundation**: Existing ghproxy provides excellent caching and request handling
- ✅ **Modular design**: Easy to add write-blocking and enhanced logging layers
- ✅ **Production-tested**: Reliable infrastructure for development use

### Container Strategy
- ✅ **Podman-first**: Works well with rootless containers and security
- ✅ **Multi-stage build**: Separates build dependencies from runtime
- ✅ **Go 1.24 required**: Critical for supporting the `tool` directive in go.mod

### Plugin Development Focus
- ✅ **Shrug plugin ideal**: Perfect "hello world" example (111 lines, minimal deps)
- ✅ **Real GitHub data**: Provides realistic development environment
- ✅ **Safety first**: Write-blocking ensures no accidental modifications

## Testing Status

### Working Commands
```bash
# Build container
./hack/dev-scripts/build-dev-container.sh

# Check GitHub proxy (works)
curl http://localhost:8888/dev/status
curl http://localhost:8888/dev/help

# Start environment (proxy works, hook fails on auth)
podman run -d --rm -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  --entrypoint /workspace/hack/dev-scripts/start-dev-environment.sh \
  --name prow-dev prow-hook-dev:latest
```

### Expected Working Commands (after token fix)
```bash
# Test hook component
curl http://localhost:8889/plugin-help

# Test shrug plugin
./hack/dev-scripts/test-shrug-plugin.sh

# View enhanced logs showing blocked operations
podman logs prow-dev
```

## Technical Implementation Details

### Write-Blocking Transport
- Intercepts HTTP requests in the RoundTripper chain
- Blocks non-GET/HEAD methods before reaching GitHub
- Logs detailed context about what would have been written
- Returns mock success responses to keep plugins working

### Enhanced Logging
- Plugin-aware context extraction from GitHub API URLs
- Prometheus metrics for blocked operations
- Structured logging with operation types (add_label, create_comment, etc.)
- Development-friendly log formatting

### Configuration Management
- Minimal Prow config focused on shrug plugin
- Auto-generated secrets for development
- Simplified plugin configuration with single repo target

## Resources and References

- **Existing tools**: Uses phony tool for webhook simulation
- **GitHub API patterns**: Analyzed all plugin interactions for mocking requirements
- **Prow architecture**: Built on existing ghproxy/ghcache infrastructure
- **Container best practices**: Rootless, multi-stage, proper caching

---

**Next Claude Code session should start by**:
1. Reading this WIP.md thoroughly
2. Testing the current container build: `./hack/dev-scripts/build-dev-container.sh`
3. Investigating GitHub token authentication bypass options in hook component
4. Getting the full end-to-end demo working with either real tokens or auth bypass