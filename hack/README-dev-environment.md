# Prow Development Environment

A containerized development environment for Prow contributors that provides:

- ✅ **Safe development** - reads real GitHub state but blocks all writes
- 🚀 **Fast iteration** - hot reloading and minimal setup  
- 📊 **Enhanced logging** - see exactly what Prow would do
- 🎯 **Focused development** - work on individual plugins without full Prow complexity

## Quick Start

```bash
# 1. Build the development container
./hack/dev-scripts/build-dev-container.sh

# 2. Run the development environment
podman run -it --rm \
  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  prow-hook-dev:latest

# 3. Test the shrug plugin (in another terminal)
./hack/dev-scripts/test-shrug-plugin.sh
```

## Architecture

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

## Development Workflow

1. **Write/modify plugin code** in `pkg/plugins/yourplugin/`
2. **Send test webhooks** using provided scripts
3. **Check enhanced logs** to see what Prow would do
4. **Iterate quickly** - changes are reflected immediately

## Key Features

### Safe GitHub Integration

- **Reads real GitHub state** (issues, PRs, labels, comments)
- **Blocks all write operations** (comments, labels, statuses)
- **Enhanced logging** shows what would happen:
  ```
  🚫 BLOCKED GitHub write operation in development mode
  🏷️  Prow would ADD LABEL to kubernetes/test-infra#12345
  💬 Prow would CREATE COMMENT: "¯\_(ツ)_/¯"
  ```

### Development-Focused Tools

- **Hot reloading** with volume mounts
- **Pre-installed Go tools** (air, golangci-lint, debugger)
- **Webhook simulation** scripts
- **Real-time log following**
- **Development endpoints** for status and help

## Files and Structure

```
hack/
├── Containerfile.dev              # Development container definition
├── dev-scripts/
│   ├── build-dev-container.sh     # Build the container
│   ├── start-dev-environment.sh   # Start development services
│   └── test-shrug-plugin.sh       # Test the shrug plugin
└── README-dev-environment.md      # This file

cmd/
└── ghproxy-dev/                   # Development GitHub proxy
    └── main.go                    # Enhanced proxy with write blocking

site/content/en/docs/contribution-guidelines/
└── single-component-development.md # Contributor documentation
```

## Plugin Development

### Starting Point: Shrug Plugin

The shrug plugin (`pkg/plugins/shrug/shrug.go`) is perfect for learning:
- **111 lines** - easy to understand
- **Simple logic** - regex matching and label management  
- **Minimal dependencies** - only GitHub API calls

### Development Cycle

1. Modify plugin code
2. Test with webhook simulation
3. Check logs for expected behavior
4. Iterate

### Example: Adding a New Feature

```bash
# 1. Edit the plugin
vim pkg/plugins/shrug/shrug.go

# 2. Test your changes
./hack/dev-scripts/test-shrug-plugin.sh

# 3. Check logs to verify behavior
tail -f /workspace/logs/*.log
```

## Advanced Usage

### Using Real GitHub Repositories

```bash
# Mount your GitHub token for realistic testing
podman run -it --rm \
  -p 8888:8888 -p 8889:8889 -p 8890:8890 -p 8891:8891 \
  -v $(pwd):/workspace:Z \
  -v ~/.github-token:/etc/github/token:ro \
  prow-hook-dev:latest
```

### Interactive Development

```bash
# Run a development shell
podman run -it --rm \
  -v $(pwd):/workspace:Z \
  --entrypoint /bin/bash \
  prow-hook-dev:latest

# Inside the container:
go run ./cmd/hook --help
air  # Hot reloading
golangci-lint run
```

### Multiple Plugins

Enable more plugins by editing `/etc/prow-dev/plugins.yaml`:

```yaml
plugins:
  "your-org/your-repo":
    - shrug
    - hold
    - heart
    - label
```

## Monitoring and Debugging

### Development Endpoints

- **Hook**: http://localhost:8889/plugin-help
- **Proxy Status**: http://localhost:8888/dev/status
- **Proxy Help**: http://localhost:8888/dev/help
- **Metrics**: http://localhost:8890/metrics
- **Health**: http://localhost:8891/healthz

### Logs and Metrics

```bash
# Follow all logs
tail -f /workspace/logs/*.log

# Check proxy metrics
curl http://localhost:8890/metrics | grep ghproxy_dev

# Check proxy status
curl http://localhost:8888/dev/status | jq
```

## Extending the Environment

### Adding New Components

This pattern can be extended to other Prow components:

1. Create a development version with enhanced logging
2. Add to the Containerfile
3. Update startup scripts
4. Create test scripts

### Improving the Proxy

The development proxy (`cmd/ghproxy-dev/main.go`) can be enhanced with:
- More sophisticated GitHub API mocking
- Plugin-specific logging contexts
- Webhook replay capabilities
- Integration with external tools

## Contributing

This development environment is designed to grow with the community:

1. **Start simple** - use the shrug plugin to learn
2. **Add features** - enhance logging, add tools, improve workflows
3. **Share improvements** - contribute back to help other developers
4. **Extend scope** - adapt for other Prow components

## Troubleshooting

### Common Issues

**Build failures:**
```bash
# Check Podman installation
podman version

# Verify you're in the repository root
ls hack/Containerfile.dev
```

**Connection issues:**
```bash
# Check services are running
curl http://localhost:8889/plugin-help
curl http://localhost:8888/dev/status
```

**GitHub API issues:**
```bash
# Check proxy logs
tail -f /workspace/logs/ghproxy-dev.log

# Verify token (if using real repositories)
curl -H "Authorization: token $(cat ~/.github-token)" https://api.github.com/user
```

### Getting Help

- Check the logs first: `tail -f /workspace/logs/*.log`
- Review development endpoints for status information
- Consult the contributor documentation in `site/content/en/docs/`
- Ask in [Kubernetes Slack #prow](https://kubernetes.slack.com/channels/prow)

## Next Steps

1. **Try the quick start** to familiarize yourself with the environment
2. **Modify the shrug plugin** to understand the development cycle
3. **Enable additional plugins** to explore more functionality
4. **Write a new plugin** using the established patterns
5. **Contribute improvements** to help other developers

The goal is to make Prow development accessible to new contributors while providing the tools and safety needed for productive development.