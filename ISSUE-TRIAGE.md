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

---

### Effort Assessment

**Effort Level**: 2 - Moderate (help-needed)

**Assessment Date**: 2026-02-06

#### Summary

This is a moderate-effort feature addition. While the infrastructure already exists and the pattern is well-established, implementing it across both Deck and Hook requires understanding TLS concepts, Prow's option patterns, and testing both components. Well-suited for contributors with some Go and Prow experience.

#### Factor Analysis

##### Scope of Changes
- **Assessment**: Small
- **Details**:
  - 2 files to modify: cmd/deck/main.go, cmd/hook/main.go
  - Estimated 60-100 total lines of code (30-50 per component)
  - Changes are localized to main.go files in each component
  - No changes to shared packages or libraries
  - Optional: Documentation updates for new flags
- **Level Indication**: 1-2

##### Complexity
- **Assessment**: Moderate
- **Details**:
  - Core logic is straightforward: add flags and conditional server initialization
  - Must understand TLS certificate/key file configuration
  - Need to replicate changes consistently across two components
  - Pattern already exists in cmd/admission/main.go to follow
  - No algorithmic challenges or concurrency issues
  - Edge case: validation that both cert and key are provided together
- **Level Indication**: 1-2

##### Required Expertise
- **Assessment**: Moderate
- **Details**:
  - **Go knowledge**: Basic to intermediate (flags, structs, conditionals)
  - **TLS concepts**: Understanding of certificates, keys, HTTPS
  - **Prow patterns**: Familiarity with options structs and flag registration (learnable from code)
  - **Testing**: Basic unit test writing following existing patterns
  - **Not required**: Deep Prow architecture knowledge, distributed systems expertise
  - Can learn by studying cmd/admission/main.go as reference
- **Level Indication**: 2

##### Clarity and Certainty
- **Assessment**: Well-defined
- **Details**:
  - Problem is clearly articulated by issue author
  - Solution approach is unambiguous (already tested by author)
  - Exact pattern to follow exists (cmd/admission/main.go)
  - Required infrastructure (`interrupts.ListenAndServeTLS()`) already exists
  - No competing approaches or design debates needed
  - Author has already validated the approach works
- **Level Indication**: 1-2

##### Testing Requirements
- **Assessment**: Moderate
- **Details**:
  - **Unit tests needed**:
    - Flag parsing with TLS flags present
    - Flag parsing without TLS flags (default HTTP mode)
    - Validation that both cert and key are required together
    - Validation error when only one TLS flag is provided
  - **Testing patterns**: Can follow existing option validation tests
  - **Integration testing**: Could use self-signed certs, but not strictly necessary for unit coverage
  - **Manual testing**: Would require certificate setup for verification
  - No complex test infrastructure needed
- **Level Indication**: 2

##### Backwards Compatibility
- **Assessment**: Fully compatible
- **Details**:
  - **Default behavior unchanged**: Without flags, components run in HTTP mode (current behavior)
  - **Opt-in feature**: Only users who add `--tls-cert-file` and `--tls-key-file` get TLS
  - **No configuration changes**: Existing deployments continue to work
  - **No API changes**: HTTP handlers remain the same
  - **Health checks unaffected**: Run on separate port, remain HTTP
  - **Zero risk to existing Kubernetes project deployment**
  - Gradual rollout possible (enable on one component at a time)
- **Level Indication**: 1-2

##### Architectural Alignment
- **Assessment**: Perfect fit
- **Details**:
  - **Follows established patterns**: cmd/admission and cmd/webhook-server already use this exact pattern
  - **Uses existing infrastructure**: `interrupts.ListenAndServeTLS()` proven in production
  - **Consistent with Prow philosophy**: Optional features via flags
  - **No new abstractions needed**: Everything already exists
  - **Natural extension**: Adding configurability to existing capability
  - **Aligns with Go standards**: Standard lib `crypto/tls` and `http.Server.TLSConfig`
  - Fits Prow's design of "provide hooks, let users configure"
- **Level Indication**: 1-2

##### External Dependencies
- **Assessment**: None
- **Details**:
  - **No new dependencies**: Uses only existing packages
  - **No external API requirements**: Works with any TLS certificate
  - **Certificate management**: User responsibility (cert-manager, manual, etc.)
  - **Well-documented**: TLS and Go HTTP server documentation extensive
  - **Standard practice**: HTTPS servers are well-understood in Go ecosystem
- **Level Indication**: 1-3

#### Overall Assessment

**Level 2 (Moderate)** is appropriate because:

✅ **Factors favoring Level 1-2:**
- Small scope (2 files, <100 LOC)
- Clear solution with existing pattern to follow
- Fully backwards compatible
- Perfect architectural fit
- No external dependencies

⚠️ **Factors elevating to Level 2 (not Level 1):**
- Requires moderate Go and TLS knowledge
- Must be implemented consistently across two components
- Need to understand Prow's option/flag patterns
- Testing requires understanding of both components
- Not trivial enough for a complete newcomer to Prow

**Not Level 3 because:**
- Infrastructure already exists (not building something new)
- Well-defined with clear reference implementation
- Limited scope and impact
- No concurrency, race conditions, or complex edge cases

#### Recommended Labels

Based on this assessment:

- [x] **`help-wanted`**: Good scope for contributor with some experience
  - *Rationale*: Well-defined, moderate scope, clear pattern to follow

- [x] **`kind/feature`**: Already applied, confirms this is a feature request
  - *Rationale*: Adding new optional capability

- [x] **`area/deck`**: Affects Deck component
  - *Rationale*: One of two components being modified

- [x] **`area/hook`**: Affects Hook component
  - *Rationale*: One of two components being modified

- [ ] **`good-first-issue`**: Not recommended
  - *Rationale*: Requires moderate expertise, touches multiple components, needs TLS understanding

- [ ] **`priority/important-longterm`**: Could be considered
  - *Rationale*: Helps specific security compliance scenarios, but niche use case

#### Guidance for Contributors

**For Level 2 (Moderate):**

**Prerequisites**:
- Familiarity with Go (flags, structs, conditional logic, testing)
- Understanding of TLS/HTTPS concepts (certificates, keys, what they do)
- Ability to read and follow existing code patterns
- Experience writing unit tests in Go

**Recommended Study**:
1. **Primary reference**: cmd/admission/main.go (lines 38-39, 51-86)
   - Shows exact pattern for `--tls-cert-file` and `--tls-key-file` flags
   - Demonstrates validation that both are required
   - Shows how to call `interrupts.ListenAndServeTLS()`

2. **Secondary reference**: cmd/webhook-server/main.go (lines 188-196)
   - Shows TLS configuration with `tls.Config`
   - Demonstrates `interrupts.ListenAndServeTLS()` usage

3. **Infrastructure**: pkg/interrupts/interrupts.go (lines 179-187)
   - Understand the `ListenAndServeTLS()` function signature
   - Note the graceful shutdown handling

4. **Optional features pattern**: cmd/deck/main.go (lines 664-684)
   - Shows how Deck implements optional features with flags
   - Example of conditional feature enablement

**Implementation Checklist**:
- [ ] Add `tlsCertFile` and `tlsKeyFile` fields to options struct
- [ ] Register flags in `gatherOptions()` function
- [ ] Add validation in `Validate()` method (both or neither required)
- [ ] Modify server initialization with conditional logic
- [ ] Add TLSConfig to http.Server when TLS enabled
- [ ] Call appropriate ListenAndServe function based on flags
- [ ] Write unit tests for flag parsing and validation
- [ ] Test manually with self-signed certificates
- [ ] Update documentation/help text for new flags
- [ ] Repeat for both Deck and Hook components

**Testing Approach**:
- Generate self-signed certificates for local testing:
  ```bash
  openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
  ```
- Test with flags: `--tls-cert-file=cert.pem --tls-key-file=key.pem`
- Verify server starts with HTTPS
- Verify health check port still HTTP
- Test without flags to ensure HTTP mode works (backwards compat)

**Questions to Consider**:
- Should there be a separate port for TLS, or use the same port?
  - **Answer**: Same port (simpler, matches user's use case)
- What should happen if only one TLS flag is provided?
  - **Answer**: Validation error (both required together)
- Should health checks be on HTTPS too?
  - **Answer**: No, keep on separate HTTP port for simplicity

**Similar PRs to Review** (if any exist):
- This would be the first to add this feature to Deck/Hook
- Review how admission was implemented for TLS

#### Caveats and Considerations

**Important Notes**:

1. **Issue author already implementing**: The issue author (NiJuFirenzia) has already indicated they're working on this and have tested the approach. Consider:
   - Reaching out to coordinate if you want to help
   - Letting them proceed if they're making progress
   - Offering review/testing assistance

2. **Maintainer bandwidth concerns**: Maintainer @BenTheElder noted limited bandwidth. A clean, well-tested PR following existing patterns will be easier to review and merge.

3. **Certificate management is user responsibility**:
   - This feature doesn't include certificate rotation/renewal
   - Users must handle certificate lifecycle (via cert-manager, manual process, etc.)
   - Document this clearly in flag help text and docs

4. **Niche use case**: First request for this feature in Prow's history
   - Validates that optional implementation is correct choice
   - Low risk since it won't affect existing users
   - Should remain simple and not add complexity for non-users

5. **Alternative exists**: Service mesh (Istio, Linkerd) with mTLS
   - Some orgs may prefer this architectural approach
   - Consider documenting service mesh approach as alternative
   - This PR provides choice for users who can't/won't use service mesh

6. **Testing with real certificates**:
   - Self-signed certs sufficient for testing
   - Consider testing with cert-manager in a real cluster
   - Document common certificate setups

**Success Criteria**:
- ✅ Both Deck and Hook support optional TLS flags
- ✅ Backwards compatible (no flags = HTTP mode)
- ✅ Validation prevents partial configuration
- ✅ Unit tests cover flag parsing and validation
- ✅ Manual testing confirms HTTPS works with certificates
- ✅ Health checks remain accessible via HTTP
- ✅ Documentation explains usage and certificate requirements
- ✅ Code follows existing Prow patterns (admission/webhook-server)

---

### Proposed Issue Augmentation

**Augmentation Date**: 2026-02-06

#### Title Change

- **Current**: "Allow Option for Ingress to Reach pods through SSL"
- **Proposed**: "Add optional TLS backend support for Deck and Hook pods"
- **Rationale**: More technically precise (TLS vs SSL), mentions both affected components explicitly, clearer that it's an optional feature

#### Proposed GitHub Comment

```
/retitle Add optional TLS backend support for Deck and Hook pods

## Infrastructure Already Exists

Good news: the core infrastructure for this feature already exists in Prow. The `interrupts.ListenAndServeTLS()` function (pkg/interrupts/interrupts.go:179-187) is already implemented and proven in production use by the admission and webhook-server components. What's missing is simply the configuration flags to enable it in Deck and Hook.

## Implementation Pattern

The pattern to follow is in **cmd/admission/main.go** (lines 38-39, 51-86), which implements exactly this feature:
- Add `--tls-cert-file` and `--tls-key-file` flags to the options struct
- Validate that both flags are provided together (or neither)
- Conditionally call `interrupts.ListenAndServeTLS()` when flags are present, or `interrupts.ListenAndServe()` (current HTTP mode) when absent
- This keeps the feature fully backwards compatible - deployments without the flags continue with HTTP backends as they do today

The **cmd/webhook-server/main.go** (lines 188-196) provides a secondary reference showing how to configure the TLS server with appropriate `tls.Config` settings.

## Estimated Scope

This is a relatively straightforward addition:
- **Files to modify**: cmd/deck/main.go and cmd/hook/main.go (2 files)
- **Lines of code**: ~30-50 per component (~60-100 total)
- **Pattern**: Directly follows existing Prow components
- **Breaking changes**: None (opt-in via flags)
- **Testing**: Unit tests for flag validation, manual testing with certificates (self-signed or cert-manager)

Note that health check endpoints (default port 8081) would remain on HTTP regardless of the main server's TLS configuration, ensuring liveness/readiness probes work without complexity.

/area deck
/area hook
/kind feature
/help-wanted
```

#### Rationale

**What's being added**:

1. **Infrastructure confirmation**: The issue author mentioned testing with `ListenAndServeTLS()` but didn't know how complete the infrastructure is. Confirming it exists and is production-proven reduces uncertainty.

2. **Specific implementation guidance**: While the author said they're working on it, pointing to the exact pattern in cmd/admission/main.go (with line numbers) provides a concrete reference that matches Prow's conventions.

3. **Scope estimate**: The original issue lacks any detail about implementation complexity. Adding scope estimates (files, LOC, breaking changes) helps set expectations and demonstrates this is a manageable change.

4. **Backwards compatibility assurance**: Important for maintainer review - explicitly stating this is opt-in and non-breaking addresses the bandwidth concerns raised by @BenTheElder.

5. **Health check note**: A technical detail that wasn't obvious from the issue but matters for Kubernetes deployments.

**Why these labels**:

- `/area deck`: One of the two components affected by this feature
- `/area hook`: The other component affected by this feature
- `/kind feature`: Confirms this is a feature request (already applied, but reinforcing)
- `/help-wanted`: Based on Level 2 effort assessment - moderate complexity, well-defined, suitable for contributors with some Prow experience

**Why retitle**:

- "SSL" is outdated terminology; "TLS" is more accurate
- "Ingress to Reach pods through SSL" is awkwardly phrased
- New title makes it immediately clear: optional feature, TLS backends, two components
- More searchable for others with similar needs

**What's NOT included**:

- No priority label: Author is already working on it, not seeking urgency
- No deep technical dive: Issue author already understands the problem and has tested an approach
- No alternative solutions discussion: Author already committed to this approach, and it's architecturally sound
- No detailed certificate management guidance: Keep comment focused; detailed docs can come with the PR

**Special Consideration**:

The author (@NiJuFirenzia) has already self-assigned this issue and indicated they're working on it. The augmentation is structured to be **helpful to their implementation** rather than inviting others to take over. It provides:
- Confirmation their approach is correct
- Specific code references to follow
- Scope validation
- Assurance it will align with Prow patterns

The `/help-wanted` label reflects that contributions are welcome (perhaps for review, testing, or helping with both components) but acknowledges active development.

## Next Steps

1. ✅ Initial validation complete - Issue is LEGITIMATE
2. ✅ Code research complete - Infrastructure exists, clear pattern to follow
3. ✅ Effort assessment complete - Level 2 (Moderate/help-wanted)
4. ✅ Issue augmentation proposed - Retitle + context + labels
5. ⏳ Brief maintainer on findings
6. ⏳ Finalize triage and post results
