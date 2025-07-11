---
title: "Single Component Development"
weight: 30
description: >
  Set up isolated development environments for individual Prow components like plugins
---

# Single Component Development Environment

This guide helps new contributors quickly set up minimal development environments for individual Prow components, starting with the Hook component and simple plugins. This approach removes the complexity of full Prow deployments while enabling realistic development and testing.

## Overview

Prow's microservice architecture means you often don't need a full Prow instance to develop and test individual components. This development environment provides:

- **Real GitHub state reading** through a development proxy
- **Write operation blocking** for safe development against live repositories  
- **Enhanced logging** showing what Prow would do
- **Fast iteration** with hot reloading and minimal setup
- **Isolated testing** focusing on specific plugins or components

## Quick Start

### Prerequisites

- [Podman](https://podman.io/getting-started/installation) (container runtime)
- Git
- Basic familiarity with Go (for code changes)

### 1. Build the Development Container

```bash
# Clone the repository
git clone https://github.com/kubernetes-sigs/prow.git
cd prow

# Build the development container
./hack/dev-scripts/build-dev-container.sh
```

### 2. Run the Development Environment

```bash
# Run with auto-generated secrets (basic testing)
podman run -it --rm \
  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  prow-hook-dev:latest
```

### 3. Test the Shrug Plugin

In another terminal:

```bash
# Test the shrug plugin with webhook simulation
./hack/dev-scripts/test-shrug-plugin.sh
```

## Development Workflow

### Understanding the Environment

The development environment runs two main components:

1. **ghproxy-dev** (port 8888) - GitHub API proxy that:
   - Reads real GitHub state (issues, PRs, labels, comments)
   - Blocks all write operations (safe for development)
   - Provides enhanced logging showing what Prow would do
   - Caches responses for fast iteration

2. **hook** (port 8889) - The hook component that:
   - Receives GitHub webhooks
   - Processes them through enabled plugins
   - Uses the development proxy for GitHub API calls

### Plugin Development Cycle

1. **Make code changes** to your plugin in `/workspace`
2. **Send test webhooks** using the provided test scripts
3. **Check logs** to see what Prow would do:
   ```bash
   tail -f /workspace/logs/*.log
   ```
4. **Iterate quickly** - changes are reflected immediately

### Understanding the Logs

The development environment provides enhanced logging to show exactly what Prow would do:

```bash
# Example logs when testing the shrug plugin:
🚫 BLOCKED GitHub write operation in development mode
🏷️  Prow would ADD LABEL to issue/PR 
💬 Prow would CREATE COMMENT on issue/PR
📖 Reading GitHub state
```

## Working with Real GitHub Repositories

### Using a GitHub Token (Recommended)

For realistic testing with real GitHub repositories:

1. **Create a GitHub token** with appropriate permissions
2. **Mount it into the container**:

```bash
podman run -it --rm \
  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  -v /path/to/your/github-token:/etc/github/token:ro \
  prow-hook-dev:latest
```

### Safe Testing

The development proxy ensures safety:
- **All writes are blocked** - no risk of accidentally modifying repositories
- **Reads are cached** - minimal impact on API rate limits
- **Enhanced logging** - clear visibility into what would happen

## Available Development Tools

### Development Endpoints

- **Hook component**: http://localhost:8889
  - `/hook` - Webhook endpoint for testing
  - `/plugin-help` - Plugin documentation
- **GitHub proxy**: http://localhost:8888
  - `/dev/status` - Proxy configuration and status
  - `/dev/help` - Development guide
- **Metrics**: http://localhost:8890/metrics
- **Health**: http://localhost:8891/healthz

### Testing Scripts

```bash
# Test the shrug plugin specifically
./hack/dev-scripts/test-shrug-plugin.sh

# Manual webhook testing
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issue_comment" \
  -d '{"action":"created","comment":{"body":"/shrug"},...}' \
  http://localhost:8889/hook
```

### Development Shell

For advanced development, run an interactive shell:

```bash
podman run -it --rm \
  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  --entrypoint /bin/bash \
  prow-hook-dev:latest

# Inside the container:
# - Go development tools are pre-installed
# - Air for hot reloading: air
# - Direct component execution: go run ./cmd/hook
# - Linting: golangci-lint run
```

## Plugin Development Guide

### Starting with the Shrug Plugin

The [shrug plugin]({{< relref "../components/plugins#shrug" >}}) is the simplest plugin and perfect for learning:

- **111 lines of code** - easy to understand
- **Minimal dependencies** - only GitHub API calls
- **Simple logic** - regex matching and label management
- **Clear behavior** - adds/removes labels based on comments

**Key files:**
- `pkg/plugins/shrug/shrug.go` - Main implementation
- `pkg/plugins/shrug/shrug_test.go` - Tests

### Adding a New Plugin

1. **Create plugin directory**: `pkg/plugins/myplugin/`
2. **Implement the plugin interface**:
   ```go
   func init() {
       plugins.RegisterGenericCommentHandler("myplugin", handleComment, helpProvider)
   }
   ```
3. **Add to plugin imports**: `cmd/hook/plugin-imports/plugin-imports.go`
4. **Test in development environment**
5. **Write tests** following existing patterns

### Plugin Architecture

Prow plugins follow a standard pattern:
- **Registration** in `init()` function
- **Event handlers** for different GitHub events
- **Help providers** for documentation
- **GitHub client interface** for API calls

## Extending to Other Components

This development approach can be extended to other Prow components:

### Other Hook Plugins

Enable additional plugins by modifying `/etc/prow-dev/plugins.yaml`:

```yaml
plugins:
  "your-org/your-repo":
    - shrug
    - hold
    - heart
    - label
```

### Other Components

The same containerized approach can be applied to:
- **Deck** - Prow's web UI
- **Tide** - PR merge automation  
- **Crier** - Event reporting
- **Sinker** - Job cleanup

## Troubleshooting

### Common Issues

**Container build fails:**
- Ensure Podman is installed and running
- Check you're in the Prow repository root
- Verify Containerfile exists: `hack/Containerfile.dev`

**Hook not receiving webhooks:**
- Check hook is running: `curl http://localhost:8889/plugin-help`
- Verify webhook payload format
- Check logs: `tail -f /workspace/logs/hook.log`

**GitHub proxy errors:**
- Verify proxy is running: `curl http://localhost:8888/dev/status`
- Check GitHub token if using real repositories
- Review proxy logs: `tail -f /workspace/logs/ghproxy-dev.log`

### Getting Help

- **Plugin documentation**: http://localhost:8889/plugin-help
- **Development status**: http://localhost:8888/dev/status  
- **Existing contributor docs**: [Build, Test, Update]({{< relref "../build-test-update" >}})
- **Prow community**: [Kubernetes Slack #prow](https://kubernetes.slack.com/channels/prow)

## Next Steps

1. **Try modifying the shrug plugin** - change the regex or label name
2. **Enable other simple plugins** - hold, heart, label
3. **Write a new plugin** - follow the shrug plugin pattern
4. **Contribute back** - share improvements to the development environment

This development environment is designed to grow with you - start simple with the shrug plugin, then gradually work with more complex plugins as you become comfortable with Prow's architecture.