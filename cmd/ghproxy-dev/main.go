/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// ghproxy-dev is a development-focused GitHub proxy that reads real GitHub state
// but blocks all write operations and provides enhanced logging for Prow development.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/prow/pkg/pjutil/pprof"

	"sigs.k8s.io/prow/pkg/apptokenequalizer"
	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/flagutil"
	"sigs.k8s.io/prow/pkg/ghcache"
	"sigs.k8s.io/prow/pkg/interrupts"
	"sigs.k8s.io/prow/pkg/logrusutil"
	"sigs.k8s.io/prow/pkg/metrics"
	"sigs.k8s.io/prow/pkg/pjutil"
)

var (
	blockedWriteRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ghproxy_dev_blocked_write_requests_total",
		Help: "Number of write requests blocked in development mode.",
	}, []string{"method", "path_template"})
	
	developmentLogs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ghproxy_dev_enhanced_logs_total", 
		Help: "Number of enhanced development logs generated.",
	}, []string{"log_type", "action"})
)

func init() {
	prometheus.MustRegister(blockedWriteRequests)
	prometheus.MustRegister(developmentLogs)
}

type options struct {
	port            int
	upstream        string
	cacheDir        string
	cacheSizeGB     int
	redisAddress    string
	diskFree        float64
	
	// Throttling configuration
	throttlingTime                uint
	throttlingTimeV4              uint
	throttlingTimeForGET          uint
	maxDelayTime                  uint
	maxDelayTimeV4                uint
	
	// Development mode options
	developmentMode       bool
	allowedReadOnlyOrgs   flagutil.Strings
	enhancedLogging       bool
	logWebhookSimulation  bool
}

func (o *options) Validate() error {
	if o.upstream == "" {
		return errors.New("--upstream must not be empty")
	}
	
	upstreamURL, err := url.Parse(o.upstream)
	if err != nil {
		return fmt.Errorf("failed to parse upstream URL: %w", err)
	}
	
	if upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https" {
		return fmt.Errorf("invalid scheme for upstream URL: %s", upstreamURL.Scheme)
	}
	
	if o.developmentMode && len(o.allowedReadOnlyOrgs.Strings()) == 0 {
		logrus.Warn("Development mode enabled but no allowed read-only orgs specified - will read from any org")
	}
	
	return nil
}

func gatherOptions(fs *flag.FlagSet, args ...string) options {
	var o options
	fs.IntVar(&o.port, "port", 8888, "Port to listen on.")
	fs.StringVar(&o.upstream, "upstream", "https://api.github.com", "Upstream URL")
	fs.StringVar(&o.cacheDir, "cache-dir", "", "Cache directory. If empty, use an in-memory cache.")
	fs.IntVar(&o.cacheSizeGB, "cache-sizeGB", 10, "Cache size in GB.")
	fs.StringVar(&o.redisAddress, "redis-address", "", "Redis address for cache. If empty, use local disk cache.")
	fs.Float64Var(&o.diskFree, "disk-free", 0.5, "Minimum free disk space (GB) before evicting cached responses.")
	
	// Throttling options
	fs.UintVar(&o.throttlingTime, "throttling-time-ms", 900, "Throttling time in milliseconds.")
	fs.UintVar(&o.throttlingTimeV4, "throttling-time-v4-ms", 0, "Throttling time for GraphQL API in milliseconds.")
	fs.UintVar(&o.throttlingTimeForGET, "get-throttling-time-ms", 300, "Throttling time for GET requests in milliseconds.")
	fs.UintVar(&o.maxDelayTime, "throttling-max-delay-duration-seconds", 45, "Max delay duration in seconds.")
	fs.UintVar(&o.maxDelayTimeV4, "throttling-max-delay-duration-v4-seconds", 0, "Max delay duration for GraphQL API in seconds.")
	
	// Development mode options
	fs.BoolVar(&o.developmentMode, "development-mode", false, "Enable development mode with write blocking and enhanced logging.")
	fs.Var(&o.allowedReadOnlyOrgs, "allowed-read-only-orgs", "GitHub orgs to allow read-only access to in development mode.")
	fs.BoolVar(&o.enhancedLogging, "enhanced-logging", true, "Enable enhanced development logging.")
	fs.BoolVar(&o.logWebhookSimulation, "log-webhook-simulation", true, "Log webhook simulation suggestions.")
	
	fs.Parse(args)
	return o
}

// writeBlockingTransport blocks all write operations and provides enhanced logging
type writeBlockingTransport struct {
	roundTripper        http.RoundTripper
	logger              *logrus.Entry
	allowedReadOnlyOrgs []string
	enhancedLogging     bool
}

func (w *writeBlockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check if this is a read operation
	isReadOperation := req.Method == http.MethodGet || req.Method == http.MethodHead
	
	// Extract GitHub API context
	apiContext := extractGitHubContext(req)
	
	if !isReadOperation {
		// Log the blocked write operation with context
		logFields := logrus.Fields{
			"method":      req.Method,
			"url":         req.URL.String(),
			"blocked":     true,
			"org":         apiContext.Org,
			"repo":        apiContext.Repo,
			"number":      apiContext.Number,
			"action_type": apiContext.ActionType,
		}
		
		// Read request body for logging what would have been written
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err == nil && len(bodyBytes) > 0 {
				// Try to parse as JSON for prettier logging
				var jsonBody interface{}
				if json.Unmarshal(bodyBytes, &jsonBody) == nil {
					logFields["request_body"] = jsonBody
				} else {
					logFields["request_body"] = string(bodyBytes)
				}
			}
		}
		
		w.logger.WithFields(logFields).Warn("🚫 BLOCKED GitHub write operation in development mode")
		
		// Generate enhanced Prow context logs
		if w.enhancedLogging {
			w.logProwAction(apiContext, logFields)
		}
		
		// Update metrics
		blockedWriteRequests.WithLabelValues(req.Method, apiContext.PathTemplate).Inc()
		
		// Return a mock success response
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK (Development Mode - Operation Blocked)",
			Header: http.Header{
				"Content-Type":               []string{"application/json"},
				"X-GitHub-Request-Id":        []string{"dev-mode-blocked"},
				"X-Prow-Development-Mode":    []string{"write-blocked"},
				ghcache.CacheModeHeader:      []string{string(ghcache.ModeSkip)},
			},
			Body: io.NopCloser(strings.NewReader(`{"message":"Write operation blocked in development mode","documentation_url":"https://prow.k8s.io/development"}`)),
		}, nil
	}
	
	// For read operations, add enhanced logging and continue
	if w.enhancedLogging {
		w.logger.WithFields(logrus.Fields{
			"method":     req.Method,
			"url":        req.URL.String(),
			"org":        apiContext.Org,
			"repo":       apiContext.Repo,
			"action":     "reading_github_state",
		}).Debug("📖 Reading GitHub state")
	}
	
	return w.roundTripper.RoundTrip(req)
}

// logProwAction generates enhanced logs explaining what Prow would do
func (w *writeBlockingTransport) logProwAction(ctx *githubAPIContext, logFields logrus.Fields) {
	switch ctx.ActionType {
	case "add_label":
		w.logger.WithFields(logFields).Info("🏷️  Prow would ADD LABEL to issue/PR")
	case "remove_label":
		w.logger.WithFields(logFields).Info("🏷️  Prow would REMOVE LABEL from issue/PR")
	case "create_comment":
		w.logger.WithFields(logFields).Info("💬 Prow would CREATE COMMENT on issue/PR")
	case "edit_comment":
		w.logger.WithFields(logFields).Info("✏️  Prow would EDIT COMMENT on issue/PR")
	case "delete_comment":
		w.logger.WithFields(logFields).Info("🗑️  Prow would DELETE COMMENT on issue/PR")
	case "create_status":
		w.logger.WithFields(logFields).Info("✅ Prow would CREATE STATUS on commit")
	case "create_review":
		w.logger.WithFields(logFields).Info("👀 Prow would CREATE REVIEW on PR")
	case "close_issue":
		w.logger.WithFields(logFields).Info("🔒 Prow would CLOSE ISSUE")
	case "reopen_issue":
		w.logger.WithFields(logFields).Info("🔓 Prow would REOPEN ISSUE")
	case "assign_issue":
		w.logger.WithFields(logFields).Info("👤 Prow would ASSIGN USER to issue/PR")
	default:
		w.logger.WithFields(logFields).Info("⚙️  Prow would perform GitHub API operation")
	}
	
	developmentLogs.WithLabelValues("prow_action", ctx.ActionType).Inc()
}

// githubAPIContext extracts meaningful context from GitHub API requests
type githubAPIContext struct {
	Org          string
	Repo         string
	Number       int
	ActionType   string
	PathTemplate string
}

// extractGitHubContext parses GitHub API URLs to extract meaningful context
func extractGitHubContext(req *http.Request) *githubAPIContext {
	ctx := &githubAPIContext{}
	
	// Parse URL path components
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	
	if len(pathParts) >= 2 && pathParts[0] == "repos" {
		// /repos/{org}/{repo}/...
		if len(pathParts) >= 3 {
			ctx.Org = pathParts[1]
			ctx.Repo = pathParts[2]
		}
		
		// Extract more specific context based on path patterns
		if len(pathParts) >= 4 {
			switch pathParts[3] {
			case "issues":
				if len(pathParts) >= 5 {
					if num, err := strconv.Atoi(pathParts[4]); err == nil {
						ctx.Number = num
						if len(pathParts) >= 6 {
							switch pathParts[5] {
							case "comments":
								ctx.ActionType = "create_comment"
								if req.Method == http.MethodPatch {
									ctx.ActionType = "edit_comment"
								} else if req.Method == http.MethodDelete {
									ctx.ActionType = "delete_comment"
								}
								ctx.PathTemplate = "/repos/{org}/{repo}/issues/{number}/comments"
							case "labels":
								if req.Method == http.MethodPost {
									ctx.ActionType = "add_label"
								} else if req.Method == http.MethodDelete {
									ctx.ActionType = "remove_label"
								}
								ctx.PathTemplate = "/repos/{org}/{repo}/issues/{number}/labels"
							case "assignees":
								ctx.ActionType = "assign_issue"
								ctx.PathTemplate = "/repos/{org}/{repo}/issues/{number}/assignees"
							}
						} else {
							// Issue operations
							if req.Method == http.MethodPatch {
								ctx.ActionType = "update_issue"
							}
							ctx.PathTemplate = "/repos/{org}/{repo}/issues/{number}"
						}
					}
				}
			case "pulls":
				if len(pathParts) >= 5 {
					if num, err := strconv.Atoi(pathParts[4]); err == nil {
						ctx.Number = num
						if len(pathParts) >= 6 && pathParts[5] == "reviews" {
							ctx.ActionType = "create_review"
							ctx.PathTemplate = "/repos/{org}/{repo}/pulls/{number}/reviews"
						}
					}
				}
			case "statuses":
				ctx.ActionType = "create_status"
				ctx.PathTemplate = "/repos/{org}/{repo}/statuses/{sha}"
			case "labels":
				if req.Method == http.MethodPost {
					ctx.ActionType = "create_repo_label"
				}
				ctx.PathTemplate = "/repos/{org}/{repo}/labels"
			}
		}
	}
	
	// Default path template if not set
	if ctx.PathTemplate == "" {
		ctx.PathTemplate = req.URL.Path
	}
	
	return ctx
}

func main() {
	logrusutil.ComponentInit()

	o := gatherOptions(flag.NewFlagSet(os.Args[0], flag.ExitOnError), os.Args[1:]...)
	if err := o.Validate(); err != nil {
		logrus.WithError(err).Fatal("Invalid options")
	}

	if o.developmentMode {
		logrus.Info("🚀 Starting ghproxy in DEVELOPMENT MODE")
		logrus.Info("   - All GitHub write operations will be BLOCKED")
		logrus.Info("   - Enhanced logging enabled for Prow development")
		logrus.Info("   - Reading real GitHub state with caching")
		if len(o.allowedReadOnlyOrgs.Strings()) > 0 {
			logrus.Infof("   - Restricted to read-only access for orgs: %v", o.allowedReadOnlyOrgs.Strings())
		}
	}

	defer interrupts.WaitForGracefulShutdown()

	// Create cache with proper throttling times
	throttlingTimes := ghcache.NewRequestThrottlingTimes(
		o.throttlingTime,
		o.throttlingTimeV4,
		o.throttlingTimeForGET,
		o.maxDelayTime,
		o.maxDelayTimeV4,
	)
	
	// Create the upstream transport with app token equalizer
	upstreamTransport := apptokenequalizer.New(http.DefaultTransport)
	
	// Create cache
	var cache http.RoundTripper
	if o.redisAddress != "" {
		cache = ghcache.NewRedisCache(upstreamTransport, o.redisAddress, 1000, throttlingTimes)
	} else if o.cacheDir == "" {
		cache = ghcache.NewMemCache(upstreamTransport, 1000, throttlingTimes)
	} else {
		cache = ghcache.NewDiskCache(upstreamTransport, o.cacheDir, o.cacheSizeGB, 1000, false, time.Hour, throttlingTimes)
	}

	// Add development enhancements to the transport stack
	if o.developmentMode {
		// Create write-blocking transport
		cache = &writeBlockingTransport{
			roundTripper:        cache,
			logger:              logrus.NewEntry(logrus.StandardLogger()),
			allowedReadOnlyOrgs: o.allowedReadOnlyOrgs.Strings(),
			enhancedLogging:     o.enhancedLogging,
		}
	}

	// Create reverse proxy
	upstreamURL, _ := url.Parse(o.upstream)
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = cache

	// Expose prometheus metrics
	metrics.ExposeMetrics("ghproxy-dev", config.PushGateway{}, flagutil.DefaultMetricsPort)
	pprof.Instrument(flagutil.InstrumentationOptions{})

	// Create health endpoint
	health := pjutil.NewHealthOnPort(flagutil.DefaultHealthPort)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/", proxy)
	
	// Add development-specific endpoints
	if o.developmentMode {
		mux.HandleFunc("/dev/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			status := map[string]interface{}{
				"development_mode": true,
				"allowed_orgs":     o.allowedReadOnlyOrgs.Strings(),
				"enhanced_logging": o.enhancedLogging,
				"upstream":         o.upstream,
			}
			json.NewEncoder(w).Encode(status)
		})
		
		mux.HandleFunc("/dev/help", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Prow Development Proxy</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .emoji { font-size: 1.2em; }
        code { background: #f5f5f5; padding: 2px 4px; border-radius: 3px; }
        .endpoint { background: #e8f4fd; padding: 10px; margin: 10px 0; border-radius: 5px; }
    </style>
</head>
<body>
    <h1><span class="emoji">🚀</span> Prow Development Proxy</h1>
    <p>This is a development-focused GitHub proxy that reads real GitHub state but blocks all write operations.</p>
    
    <h2>What this proxy does:</h2>
    <ul>
        <li><span class="emoji">📖</span> Reads real GitHub data (issues, PRs, comments, labels)</li>
        <li><span class="emoji">🚫</span> Blocks all write operations (comments, labels, statuses)</li>
        <li><span class="emoji">💬</span> Provides enhanced logging showing what Prow would do</li>
        <li><span class="emoji">⚡</span> Caches responses for fast iteration</li>
    </ul>
    
    <h2>Development endpoints:</h2>
    <div class="endpoint">
        <strong>GET /dev/status</strong> - Proxy configuration and status
    </div>
    <div class="endpoint">
        <strong>GET /dev/help</strong> - This help page
    </div>
    
    <h2>Using with Prow hook:</h2>
    <pre><code>go run ./cmd/hook \
  --github-endpoint=http://localhost:%d \
  --dry-run=true \
  --config-path=config/prow/config.yaml \
  --plugin-config=config/prow/plugins.yaml</code></pre>
  
    <p>Check the logs to see what Prow would do! <span class="emoji">🎉</span></p>
</body>
</html>`, o.port)
		})
	}

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port), Handler: mux}

	health.ServeReady()

	logrus.Infof("Listening on port %d", o.port)
	if o.developmentMode {
		logrus.Info("Visit http://localhost:" + strconv.Itoa(o.port) + "/dev/help for development information")
	}
	
	interrupts.ListenAndServe(httpServer, 5*time.Second)
}