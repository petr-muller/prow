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

## Next Steps

1. ✅ Initial validation complete - Issue is LEGITIMATE
2. ⏳ Run research subcommand to examine implementation details
3. ⏳ Assess effort level and complexity
4. ⏳ Propose issue augmentation with technical details
5. ⏳ Brief maintainer on findings
6. ⏳ Finalize triage and post results
