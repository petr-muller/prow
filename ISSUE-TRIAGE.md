# Triage for Issue #328

**Status**: In Progress
**Created**: 2026-02-06

## Issue Information

- **Issue Number**: #328
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/328

## Findings

### Initial Validation

**Assessment**: LEGITIMATE

**Issue Category**: Feature Request

**Issue Title**: "Allow Option for Ingress to Reach pods through SSL"

**Issue Summary**:
The reporter requests support for HTTPS backend protocol between ingress and Prow pods (Deck and Hook). Currently, Prow only supports HTTP backends with SSL/TLS termination at the ingress layer. The request is to add an option to use HTTPS all the way through to the pods without terminating SSL at the ingress.

**Analysis**:

This is a legitimate feature request for the following reasons:

1. **Repository Scope Check**:
   - **Components mentioned**: Deck and Hook
   - **Exist in this repo**: Yes
     - `cmd/deck/main.go` - confirmed with interrupts.ListenAndServe at lines 513, 734
     - `cmd/hook/main.go` - confirmed with interrupts.ListenAndServe at line 277
   - **Relevant code paths**: Both components currently use `interrupts.ListenAndServe()` for HTTP servers

2. **Valid Use Case**:
   - Reporter has organizational security requirement: "all ingresses must use an https backend protocol"
   - This is a real compliance/security policy requirement, not a misconfiguration
   - Author has already tested solution using `interrupts.ListenAndServeTLS()` method

3. **Information Completeness**:
   - **Sufficient detail provided**: Yes
   - **Use case**: Clear organizational security requirement
   - **Technical approach**: Specific mention of ListenAndServeTLS() method
   - **Commitment**: Author willing to contribute PR with tests
   - **Prior research**: Author has already tested the modification

4. **Not a Misconfiguration**:
   - This is a genuine feature gap in Prow's capabilities
   - Current architecture only supports HTTP backends (TLS terminates at ingress)
   - Request is for optional HTTPS backend support (not replacing existing behavior)

5. **Community Context**:
   - Already labeled `/kind feature` by maintainer @BenTheElder
   - Maintainer noted this is the first such request (Nov 2024)
   - Maintainer suggested alternative (mTLS via service mesh), but acknowledged limited bandwidth
   - Issue went through stale/rotten cycle and was auto-closed (Apr 2025)
   - Author reopened (Oct 2025) and committed to implementing it
   - You (@petr-muller) removed lifecycle/rotten label (Oct 2025)
   - Author self-assigned and removed stale label again (Jan 2026)

**Important Considerations**:

1. **Maintainer Bandwidth**: BenTheElder explicitly noted "prow has very limited maintainer bandwidth at the moment" and that existing functionality is essential for running Kubernetes project itself

2. **Niche Use Case**: This is the first request for this feature in Prow's history, indicating it's not a common need

3. **Alternative Solutions Exist**: Service mesh solutions (Istio, Linkerd) with mTLS can achieve similar security goals without modifying Prow

4. **Contributor Commitment**: Author has demonstrated serious intent by:
   - Testing the approach locally
   - Reopening after auto-close
   - Self-assigning
   - Committing to write tests
   - Repeatedly fighting stale-bot

### Recommendation

**KEEP OPEN** - This is a legitimate feature request for an optional enhancement to Prow's ingress capabilities.

**Rationale**:
- Valid use case backed by organizational security requirements
- Components exist in this repository
- Author is willing and capable of contributing the implementation
- Feature can be optional (won't affect existing deployments)
- No fundamental architectural conflicts identified

**Risk Assessment**:
- **Low risk** if implemented as optional feature with proper testing
- **Medium effort** - requires changes to Deck and Hook server initialization
- **Niche benefit** - helps deployments with specific security policies

**Next Steps for Triage**:
1. Continue to **research** subcommand to examine implementation details
2. Assess technical complexity and backwards compatibility
3. Determine appropriate effort level and labels

---

### Code Research

**Research Date**: 2026-02-06

#### Current Implementation

**Primary Components**:
- **Deck**: cmd/deck/main.go:504-513 - Web UI server, currently HTTP-only on port 8080
- **Hook**: cmd/hook/main.go:273-277 - Webhook receiver, currently HTTP-only on configurable port (default 8888)
- **Interrupts Package**: pkg/interrupts/interrupts.go - Provides server lifecycle management

**Architecture Overview**:
Both Deck and Hook use the `interrupts` package to manage HTTP server lifecycle with graceful shutdown. Currently, both components call `interrupts.ListenAndServe()` which starts an HTTP server. The infrastructure for TLS support already exists but is not exposed via configuration flags.

**Key Code Paths**:
1. Deck server initialization: cmd/deck/main.go:507-513
   ```go
   server = &http.Server{Addr: ":8080", Handler: traceHandler(mux)}
   interrupts.ListenAndServe(server, 5*time.Second)
   ```

2. Hook server initialization: cmd/hook/main.go:273-277
   ```go
   httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port), Handler: hookMux}
   interrupts.ListenAndServe(httpServer, o.gracePeriod)
   ```

3. **Critical Finding**: `interrupts.ListenAndServeTLS()` already exists at pkg/interrupts/interrupts.go:179-187
   ```go
   func ListenAndServeTLS(server *http.Server, certFile, keyFile string, gracePeriod time.Duration)
   ```

**Data Flow**:
- Current: Ingress → (HTTPS) → Ingress TLS termination → (HTTP) → Deck/Hook pods
- Requested: Ingress → (HTTPS) → Ingress passthrough → (HTTPS) → Deck/Hook pods

#### Related Code

**TLS Configuration Patterns in Prow**:

1. **admission component** (cmd/admission/main.go:51-86):
   - Uses `--tls-cert-file` and `--tls-private-key-file` flags
   - Validates both flags are provided together
   - Calls `interrupts.ListenAndServeTLS()`
   - **Best reference pattern** for this feature

2. **webhook-server component** (cmd/webhook-server/main.go:188-196):
   - Retrieves certificates from secrets
   - Configures TLS with `tls.Config{ClientAuth: tls.NoClientCert}`
   - Calls `interrupts.ListenAndServeTLS()`

3. **Deck optional features** (cmd/deck/main.go:664-684):
   - Shows pattern for conditional feature enablement
   - Example: `redirectHTTPTo` flag that conditionally wraps handlers
   - Demonstrates how to add optional features without breaking existing deployments

**Dependencies**:
- Standard library `crypto/tls` - TLS configuration
- `pkg/interrupts` - Server lifecycle management (already has TLS support)
- Flag parsing infrastructure - Already exists in both components

**Health Check Consideration**:
- Health endpoints run on separate ports (default 8081) via pkg/pjutil/health.go:44-52
- Health checks won't be affected by main server TLS configuration
- Separate servers ensure liveness/readiness probes can remain HTTP

#### Test Coverage

**Existing Tests**:
- cmd/deck/main_test.go - Tests deck initialization
- cmd/hook/main_test.go - Tests hook initialization
- Coverage assessment: **Partial** - Server initialization is tested, but TLS scenarios are not

**Test Gaps**:
- No existing tests for TLS server configuration
- Need tests for:
  - TLS mode with valid certificates
  - TLS mode with missing/invalid certificates
  - Backwards compatibility (no flags = HTTP mode)
  - Flag validation (both cert and key required together)

**Test Patterns to Follow**:
- cmd/admission/main.go provides example of TLS flag validation testing
- Integration tests would need to generate self-signed certificates

#### Root Cause Analysis

**Current Limitation**:
Deck and Hook lack configuration flags for TLS certificates. While the underlying infrastructure (`interrupts.ListenAndServeTLS()`) exists and is proven in other components (admission, webhook-server), there's no way for users to enable it.

**Why This Limitation Exists**:
- Historical design: Prow assumes TLS termination at ingress/load balancer layer
- Standard Kubernetes pattern: Let ingress controllers handle TLS
- Simpler operations: No certificate management in application pods
- Works for Kubernetes project's needs (the primary user)

**What's Missing**:
1. Command-line flags: `--tls-cert-file` and `--tls-key-file`
2. Options struct fields to store certificate paths
3. Conditional logic to choose between ListenAndServe vs ListenAndServeTLS
4. Validation logic to ensure both cert and key are provided together
5. Documentation for the feature

#### Proposed Solutions

##### Approach 1: Add Optional TLS Flags (Recommended)

**Description**:
Add `--tls-cert-file` and `--tls-key-file` flags to Deck and Hook. When both flags are provided, use `interrupts.ListenAndServeTLS()` instead of `interrupts.ListenAndServe()`. When flags are empty (default), maintain current HTTP behavior.

**Implementation Points**:
- Deck: Modify cmd/deck/main.go options struct (line ~117) and server init (lines 504-513)
- Hook: Modify cmd/hook/main.go options struct (line ~59) and server init (lines 273-277)
- Pattern: Follow cmd/admission/main.go:51-86 for flag structure
- Validation: Both flags required together, or neither

**Pros**:
- ✅ Minimal code changes (infrastructure already exists)
- ✅ Backwards compatible (empty flags = current behavior)
- ✅ Consistent with existing Prow components (admission, webhook-server)
- ✅ Simple mental model: flags present = TLS, flags absent = HTTP
- ✅ No new dependencies required
- ✅ Health checks unaffected (separate port)

**Cons**:
- ⚠️ Users must manage certificates (generation, renewal, rotation)
- ⚠️ Additional configuration complexity for users who need it
- ⚠️ No certificate auto-rotation (user must restart pods with new certs)

**Affected Components**:
- cmd/deck/main.go: Add flags, conditional server initialization
- cmd/hook/main.go: Add flags, conditional server initialization
- Documentation: Add usage examples for TLS mode

**Complexity**: Low

**Backwards Compatibility**: 100% - No changes to default behavior

**Testing Requirements**:
- Unit tests for flag validation
- Unit tests for option parsing with/without TLS flags
- Integration tests with self-signed certificates (optional)
- Documentation of testing with cert-manager or manual certs

##### Approach 2: Automatic Certificate Discovery from Kubernetes Secrets

**Description**:
Automatically detect and load certificates from well-known Kubernetes secret mount paths (e.g., `/etc/tls/tls.crt` and `/etc/tls/tls.key`). If files exist, enable TLS automatically.

**Pros**:
- ✅ Works seamlessly with cert-manager
- ✅ No command-line flags needed
- ✅ Common pattern in Kubernetes ecosystem

**Cons**:
- ⚠️ Less explicit configuration (magic behavior)
- ⚠️ Harder to debug when certificates are unexpectedly loaded
- ⚠️ Requires agreed-upon mount path convention
- ⚠️ Doesn't match existing Prow patterns (other components use explicit flags)

**Complexity**: Low-Medium

**Backwards Compatibility**: Good if paths are chosen carefully

##### Approach 3: Dedicated TLS Port (Dual Listeners)

**Description**:
Add `--tls-port` flag alongside existing port. Run both HTTP and HTTPS servers simultaneously on different ports.

**Pros**:
- ✅ Allows mixed environments (some ingresses HTTP, others HTTPS)
- ✅ Easier migration path

**Cons**:
- ⚠️ More complex: managing two servers
- ⚠️ Port management complexity
- ⚠️ Unclear which port to use for which purpose
- ⚠️ Doesn't match the requester's use case (they want HTTPS backend on standard port)

**Complexity**: Medium

**Backwards Compatibility**: Good

#### Recommendation

**Preferred Approach**: **Approach 1 - Add Optional TLS Flags**

**Rationale**:
1. **Infrastructure exists**: `interrupts.ListenAndServeTLS()` is production-proven in admission and webhook-server components
2. **Minimal changes**: Only need flags and conditional initialization logic
3. **Consistent pattern**: Matches admission component's design exactly
4. **Low risk**: Optional feature with 100% backwards compatibility
5. **Clear semantics**: Explicit flags make behavior obvious
6. **Maintenance burden**: Minimal - no new infrastructure to maintain

**Key Implementation Considerations**:

1. **Flag Validation**:
   - Both `--tls-cert-file` and `--tls-key-file` must be provided together
   - Validate in options.Validate() method
   - Error clearly if only one is provided

2. **Conditional Server Initialization**:
   ```go
   if o.tlsCertFile != "" && o.tlsKeyFile != "" {
       // TLS mode
       server.TLSConfig = &tls.Config{ClientAuth: tls.NoClientCert}
       interrupts.ListenAndServeTLS(server, o.tlsCertFile, o.tlsKeyFile, gracePeriod)
   } else {
       // HTTP mode (current behavior)
       interrupts.ListenAndServe(server, gracePeriod)
   }
   ```

3. **Documentation Needs**:
   - Document the new flags in command help
   - Provide examples with cert-manager
   - Provide examples with manual certificates
   - Note that health checks remain on HTTP (separate port)

4. **Certificate Management Guidance**:
   - Document that users are responsible for:
     - Certificate generation/provisioning
     - Certificate renewal/rotation
     - Restarting pods after certificate updates
   - Suggest using cert-manager for automation
   - Provide example Kubernetes manifests

5. **Port Considerations**:
   - Same port for HTTP or HTTPS (8080 for Deck, 8888 for Hook)
   - Health port (8081) remains HTTP regardless
   - Metrics port remains HTTP regardless

6. **Testing Strategy**:
   - Unit tests: Flag validation (both/neither required)
   - Unit tests: Option parsing with TLS flags
   - Manual testing: Self-signed certificates
   - Manual testing: cert-manager integration
   - Verify health checks still work when main server is HTTPS

**Migration/Rollout Strategy**:
- Feature is opt-in via flags
- Existing deployments unaffected (no flags = HTTP mode)
- Users with TLS requirements can add flags and certificate volumes
- Can be rolled out gradually (one component at a time if desired)

**Estimated Scope**:
- Lines of code: ~30-50 per component (Deck and Hook)
- Files modified: 2 (cmd/deck/main.go, cmd/hook/main.go)
- New dependencies: 0
- Breaking changes: 0

## Next Steps

1. ✅ Initial validation complete - Issue is LEGITIMATE
2. ⏳ Run research subcommand to examine implementation details
3. ⏳ Assess effort level and complexity
4. ⏳ Propose issue augmentation with technical details
5. ⏳ Brief maintainer on findings
6. ⏳ Finalize triage and post results
