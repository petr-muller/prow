# Triage for Issue #376

**Status**: In Progress
**Created**: 2026-01-24

## Issue Information

- **Issue Number**: #376
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/376

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Bug / Missing Feature

**Issue Summary**:
The issue reporter (loispostula) cannot run Prow Deck at a subpath (e.g., `https://infra.example.com/prow`) instead of at the root path. While the main Deck page loads when accessed via a subpath, static assets and API endpoints (like `prowjobs.js`) fail because they're requested from the root domain rather than the configured subpath. The reporter has reviewed the code and cannot find a way to configure a path prefix.

**Repository Scope Check**:
- Component mentioned: Prow Deck
- Exists in this repo: Yes - Deck is a core Prow component at `cmd/deck/`
- Relevant code paths: `cmd/deck/`, deck templates, static asset serving

**Information Completeness**:
- Sufficient detail provided: Yes
- Issue includes:
  - Clear description of the problem
  - Concrete example with Kubernetes ingress configuration
  - Expected vs actual behavior
  - Note that they've searched the code for a solution
- Missing information: None - the issue is well-described

**Current Status**:
- Issue is already labeled as `kind/bug` and `area/deck`
- A contributor (tsj-30) has assigned themselves and proposed a solution approach
- Maintainer (petr-muller) has approved the approach and suggested breaking it into smaller PRs
- Active work is in progress as of November 2025

**Analysis**:
This is a legitimate architectural limitation in Prow Deck. The component assumes it runs at the root path (/) and doesn't support running under a subpath. This affects users who need to deploy Deck behind a reverse proxy or ingress controller on a subpath, which is a common deployment pattern in Kubernetes environments where multiple services share a single domain.

The issue represents a real gap in Deck's deployment flexibility. The proposed solution involves:
1. Adding BasePath awareness to templates
2. Extending URL routing logic (Simplify function)
3. Updating static resource and API endpoint references

This is not a misconfiguration or user error - it's a missing feature/capability in Deck itself.

### Recommendation

**Suggested Action**: Keep open and continue triage

**Rationale**:
- This is a legitimate bug/missing feature in Prow Deck
- The issue is well-documented with clear reproduction details
- It affects a valid use case (running behind a reverse proxy on a subpath)
- Work is already in progress by an active contributor
- Proper labels are already applied (kind/bug, area/deck)

**Next Steps**:
1. ✅ Proceed with research subcommand to understand the code architecture and implementation details
2. Assess effort level once solution approach is fully understood
3. Possibly augment the issue with additional technical context from code exploration

---

### Code Research

#### Current Implementation

**Primary Components**:
- **HTTP Handler Setup**: `cmd/deck/main.go:312-507` - Main HTTP multiplexer with hardcoded route registration
- **Template System**: `cmd/deck/template/base.html` - Base template with hardcoded asset and navigation paths
- **Static Asset Serving**: `cmd/deck/main.go:314` - `/static/` handler with no prefix support
- **Frontend TypeScript**: `cmd/deck/static/common/rerun.ts:26-28` - URL construction assuming root path
- **CSRF Middleware**: `cmd/deck/main.go:506` - Configured with hardcoded `Path("/")`

**Architecture Overview**:
Deck uses Go's `http.NewServeMux()` to register all HTTP handlers with absolute paths starting from the root (`/`). Templates are rendered using Go's `html/template` package with template functions that provide branding and configuration, but no base path support. Static assets (CSS, JS bundles, images) are served from `/static/` with long cache headers and version query parameters. The frontend TypeScript code constructs API endpoint URLs by combining `location.protocol`, `location.host`, and hardcoded absolute paths.

**Key Code Paths**:

1. **Route Registration**: `cmd/deck/main.go:312-326`
   ```go
   mux := http.NewServeMux()
   mux.Handle("/static/", http.StripPrefix("/static", staticHandlerFromDir(o.staticFilesLocation)))
   mux.Handle("/pr", gziphandler.GzipHandler(handleSimpleTemplate(o, cfg, "pr.html", nil)))
   mux.Handle("/tide", gziphandler.GzipHandler(handleSimpleTemplate(o, cfg, "tide.html", nil)))
   // ... all routes use absolute paths
   ```

2. **Template Asset References**: `cmd/deck/template/base.html:30-34`
   ```html
   <link rel="stylesheet" type="text/css" href="/static/style.css?v={{deckVersion}}">
   <link rel="stylesheet" type="text/css" href="/static/extensions/style.css?v={{deckVersion}}">
   <script type="text/javascript" src="/static/extensions/script.js?v={{deckVersion}}"></script>
   ```

3. **Navigation Links**: `cmd/deck/template/base.html:47-66`
   ```html
   <a href="/">Prow Status</a>
   <a href="/pr">PR Status</a>
   <a href="/command-help">Command Help</a>
   <a href="/tide">Tide Status</a>
   ```

4. **Frontend URL Construction**: `cmd/deck/static/common/rerun.ts:26-28`
   ```typescript
   const getJobURL = (mode: string): string => {
       return `${location.protocol}//${location.host}/rerun?mode=${mode}&prowjob=${prowjob}`;
   };
   ```

5. **CSRF Configuration**: `cmd/deck/main.go:506`
   ```go
   CSRF := csrf.Protect(csrfToken, csrf.Path("/"), csrf.Secure(!o.allowInsecure))
   ```

**Data Flow**:
1. Browser requests page (e.g., `https://example.com/prow/` with ingress rewrite)
2. Ingress strips `/prow/` prefix and forwards request to Deck at `/`
3. Deck serves HTML template with hardcoded paths like `/static/style.css`
4. Browser requests `https://example.com/static/style.css` (missing `/prow/` prefix)
5. Request fails because it doesn't go through the ingress rewrite rule
6. Similarly, JavaScript makes API calls to `/prowjobs.js` which also fail

#### Related Code

**Dependencies**:
- `github.com/gorilla/csrf` - CSRF protection middleware (configured for root path only)
- `github.com/NYTimes/gziphandler` - Response compression
- `html/template` - Go template rendering
- `net/http` - Standard library HTTP server and mux

**Template Files** (all with hardcoded paths):
- `cmd/deck/template/base.html` - Base template with navigation and asset loading
- `cmd/deck/template/index.html` - Main Prow status page
- `cmd/deck/template/pr.html` - PR status page
- `cmd/deck/template/tide.html` - Tide dashboard
- `cmd/deck/template/tide-history.html` - Tide history page
- `cmd/deck/template/plugins.html` - Plugins page
- `cmd/deck/template/command-help.html` - Command documentation

**Frontend TypeScript Files** (URL construction):
- `cmd/deck/static/common/rerun.ts:27` - Rerun job URL
- `cmd/deck/static/common/abort.ts` - Abort job URL
- `cmd/deck/static/pr/pr.ts:113` - PR data API URL
- `cmd/deck/static/prow/prow.ts:524-778` - Multiple URL references for logs, prowjobs, GitHub links, Spyglass

**Configuration Options**:
- `--static-files-location` - Filesystem path to static assets (not URL path)
- `--template-files-location` - Filesystem path to templates
- `--spyglass-files-location` - Filesystem path to Spyglass assets
- **No option for URL base path/prefix**

#### Test Coverage

**Existing Tests**:
- Unit tests exist for individual handlers (e.g., `cmd/deck/badge_test.go`, `cmd/deck/abort_test.go`)
- Tests focus on functionality, not URL path variations
- No tests for subpath deployment scenarios

**Test Gaps**:
- No test coverage for running Deck behind a reverse proxy with path rewriting
- No tests verifying that asset URLs work with a base path prefix
- No tests for template rendering with base path awareness
- Missing integration tests for subpath deployment scenarios

#### Documentation Review

**Code Comments**:
- Main.go contains basic handler setup comments but no mention of deployment path constraints
- No comments warning that Deck must be deployed at root path
- Template files lack comments about path construction

**Configuration Documentation**:
- Command-line flags documented via `--help`
- No documentation of the root path deployment requirement
- No guidance for users wanting to deploy on a subpath

**Known Limitations**:
- Not explicitly documented that Deck requires root path deployment
- Users discover this limitation when attempting subpath deployment (as in issue #376)

#### Root Cause Analysis

**Primary Cause**:
Deck was designed with the assumption that it would always be deployed at the web server root path (`/`). All URL construction throughout the codebase uses absolute paths starting with `/` rather than relative paths or configurable base paths. There is no architectural provision for a URL prefix.

**Contributing Factors**:

1. **HTTP Handler Registration**: All `mux.Handle()` calls use hardcoded absolute paths (`"/static/"`, `"/pr"`, etc.) with no mechanism to prepend a base path.

2. **Template Hardcoding**: HTML templates directly embed absolute paths in `href` and `src` attributes without using a template function or variable for the base path.

3. **Frontend Path Construction**: TypeScript code constructs URLs by concatenating `location.protocol`, `location.host`, and hardcoded paths, with no base path variable.

4. **CSRF Middleware**: The CSRF protection is configured with `csrf.Path("/")`, which would need to be updated to match any base path.

5. **No Configuration Option**: There is no command-line flag, environment variable, or config file option to specify a base path prefix.

6. **Ingress Rewrite Limitations**: While ingress controllers can rewrite paths when proxying to Deck, they only affect the initial request. Subsequent asset and API requests from the browser bypass the ingress rewrite because they use absolute paths.

**Reproduction Conditions**:
- Deploy Deck behind an ingress controller or reverse proxy
- Configure ingress to serve Deck at a subpath (e.g., `/prow/`) with path rewriting
- The main HTML page loads because ingress forwards the rewritten request
- Static assets fail to load: browser requests `/static/style.css` instead of `/prow/static/style.css`
- API calls fail: browser requests `/prowjobs.js` instead of `/prow/prowjobs.js`
- Navigation links fail: clicking "PR Status" goes to `/pr` instead of `/prow/pr`

#### Proposed Solutions

#### Approach 1: Add Configurable Base Path Support

**Description**: Introduce a `--base-path` configuration option that prepends a URL prefix to all routes, template references, and frontend URLs. This involves:

1. Add a `basePath` configuration option (CLI flag, environment variable)
2. Modify HTTP handler registration to prepend base path to all routes
3. Add a `basePath` template function accessible in all templates
4. Update all templates to use `{{basePath}}/static/...` instead of `/static/...`
5. Pass base path to frontend via a JavaScript variable
6. Update TypeScript code to use the base path when constructing URLs
7. Update CSRF middleware to use the configured base path

**Pros**:
- Clean architectural solution that works with any base path
- Backward compatible (empty base path = current behavior)
- Allows running multiple Deck instances on different subpaths of the same domain
- Configuration-driven, no hardcoded assumptions
- Aligns with standard web application patterns for subpath deployment

**Cons**:
- Requires changes across multiple layers (Go handlers, templates, TypeScript)
- Need to ensure all URL references are updated (risk of missing some)
- Requires careful testing of all pages and features
- May need to handle edge cases (trailing slashes, empty paths, nested paths)

**Affected Components**:
- `cmd/deck/main.go`: Handler registration, CSRF setup, base path flag
- `cmd/deck/templates.go`: Add `basePath` template function
- All template files (`cmd/deck/template/*.html`): Update asset and navigation links
- All TypeScript files (`cmd/deck/static/**/*.ts`): Update URL construction
- Static HTML if any (e.g., index files, error pages)

**Complexity**: Medium-High

**Backwards Compatibility**:
- Fully backward compatible if base path defaults to empty string
- Existing deployments continue to work without configuration changes
- New deployments can opt-in to subpath support via `--base-path` flag

**Testing Requirements**:
- Unit tests: Verify handler path registration with various base paths
- Template tests: Ensure URLs render correctly with base path
- Integration tests: Deploy Deck with base path and verify all features work
- Edge case tests: Empty path, trailing slashes, nested paths, URL encoding

#### Approach 2: Relative Path URLs

**Description**: Convert all absolute paths to relative paths where possible, allowing the browser to resolve paths relative to the current page location. This works better with reverse proxy path rewriting.

**Pros**:
- Less configuration needed
- Works automatically with some reverse proxy setups
- Simpler than full base path support

**Cons**:
- Relative paths can be complex and error-prone (../../static/file.css)
- Doesn't work for API endpoints called from JavaScript (still need absolute or well-formed paths)
- Breaks with deep URLs (e.g., /configured-jobs/namespace/job requires ../../../static/)
- Still requires updates to all templates and frontend code
- Navigation links still need special handling
- Less flexible than configurable base path

**Affected Components**:
- All template files
- All TypeScript files
- May require path depth calculation logic

**Complexity**: Medium

**Backwards Compatibility**:
- Could break existing deployments if relative path resolution differs
- Risk of subtle bugs with deep URL paths

#### Approach 3: JavaScript-Based Path Detection

**Description**: Use JavaScript on page load to detect the current path prefix and rewrite all URLs dynamically. Keep Go handlers at absolute paths but use ingress path rewriting.

**Pros**:
- Minimal Go code changes
- Templates remain mostly unchanged
- Configuration happens in ingress/proxy, not in Deck

**Cons**:
- Fragile and error-prone approach
- Requires JavaScript to run before assets load (chicken-and-egg problem)
- Hard to implement correctly for stylesheets loaded in `<head>`
- Doesn't solve the handler registration issue
- Poor user experience if JavaScript fails
- Not a clean architectural solution

**Complexity**: Medium

**Backwards Compatibility**: Compatible but hacky

#### Recommendation

**Preferred Approach**: **Approach 1 (Add Configurable Base Path Support)**

**Rationale**:
This is the most robust and architecturally sound solution. While it requires changes across multiple layers, it provides:
- Clean, configuration-driven deployment flexibility
- Full backward compatibility
- Support for any base path (not just specific patterns)
- Proper handling of all URL types (assets, navigation, API endpoints)
- Alignment with standard web application practices

The contributor (tsj-30) has already proposed an approach similar to this, which validates that this is the right direction.

**Key Implementation Considerations**:

1. **Base Path Normalization**: Ensure base path starts with `/` and doesn't end with `/` (e.g., `/prow` not `prow/` or `/prow/`)

2. **Template Function**: Create a simple template function like `{{basePath "/static/style.css"}}` that prepends the base path

3. **Frontend Variable**: Inject base path into HTML as a JavaScript variable:
   ```html
   <script>const BASE_PATH = "{{basePath ""}}";</script>
   ```

4. **URL Helper Functions**: Create TypeScript helper functions for URL construction:
   ```typescript
   function apiURL(path: string): string {
       return `${window.BASE_PATH}${path}`;
   }
   ```

5. **Handler Wrapper**: Create a helper function to register handlers with base path prefix:
   ```go
   func (m *muxWithBasePath) Handle(path string, handler http.Handler) {
       m.mux.Handle(m.basePath + path, handler)
   }
   ```

6. **CSRF Path**: Update CSRF middleware to use base path: `csrf.Path(basePath + "/")`

7. **Trailing Slash Consistency**: Ensure consistent handling of trailing slashes in routes

8. **Documentation**: Update deployment docs to explain base path configuration

**Migration/Rollout Strategy**:
- Feature can be added incrementally through small PRs as suggested by maintainer
- First PR: Add base path configuration and handler wrapper
- Second PR: Update templates with base path function
- Third PR: Update frontend TypeScript URL construction
- Fourth PR: Update CSRF and remaining edge cases
- Each PR can be tested independently
- Default behavior (empty base path) maintains full backward compatibility
- Existing deployments require no changes unless adopting subpath deployment

## Next Steps

1. Run research subcommand to explore Deck's routing, template rendering, and static asset serving
2. Run assess-effort subcommand to evaluate implementation complexity
3. Run augment subcommand to enhance issue with technical details
4. Run brief subcommand for final review
5. Run wrapup subcommand to finalize triage
