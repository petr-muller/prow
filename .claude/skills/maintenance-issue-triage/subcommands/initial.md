# Subcommand: maintenance::issues::triage::initial

## Purpose

Perform initial validation of a GitHub issue to determine if it constitutes a legitimate record that should be kept in the repository. This helps distinguish between:

**Legitimate issues** (keep open):
- Bugs in code that lives in this repository
- Feature requests for components in this repository
- Enhancement proposals for existing functionality

**Issues to close** (not legitimate):
- Misconfigurations or user errors
- Requests to investigate CI job failures (unless it's a Prow component bug)
- Issues that belong in other repositories
- Generic support requests that should go elsewhere
- Duplicate issues
- Issues lacking sufficient information to be actionable

## Parameters

- `issue_number` (required): The GitHub issue number to validate

## Instructions

You are performing initial validation for GitHub issue #{issue_number}.

### Step 1: Gather Issue Context

Fetch the full issue details if not already available:

```bash
gh issue view {issue_number} --json title,body,state,labels,comments,author,createdAt,url
```

### Step 2: Analyze Issue Content

Examine the following aspects:

1. **Issue Title and Description**
   - Does it describe a specific problem or feature request?
   - Is it related to code/functionality in this repository?
   - Does it provide enough context to be actionable?

2. **Determine Issue Category**
   - Is this reporting a bug in Prow components?
   - Is this requesting a new feature for Prow?
   - Is this a misconfiguration or user error?
   - Is this asking for help with CI/CD setup?
   - Does this belong in a different repository?

3. **Check for Required Information**
   - For bugs: reproduction steps, expected vs actual behavior, environment details
   - For features: use case, proposed solution, alternatives considered
   - Is there enough information to act on this issue?

4. **Review Repository Scope**
   - Check what components/code exist in this repository
   - Verify the issue relates to code maintained here
   - Identify if it's actually a downstream/upstream issue

### Step 3: Make Legitimacy Assessment

Based on your analysis, determine:

- **LEGITIMATE**: Issue describes a valid bug or feature request for this repository
- **NEEDS_INFO**: Issue might be legitimate but lacks sufficient detail
- **CLOSE**: Issue should be closed (explain why: misconfiguration, wrong repo, duplicate, etc.)
- **REDIRECT**: Issue belongs in another repository (specify which one)

### Step 4: Update Triage Document

Update `ISSUE-TRIAGE.md` with your findings:

```markdown
## Initial Validation

**Assessment**: [LEGITIMATE/NEEDS_INFO/CLOSE/REDIRECT]

### Analysis

[Your detailed analysis here]

**Issue Category**: [Bug/Feature Request/Misconfiguration/Support Request/Other]

**Repository Scope Check**:
- Component mentioned: [component name]
- Exists in this repo: [Yes/No]
- Relevant code paths: [list file paths if applicable]

**Information Completeness**:
- Sufficient detail provided: [Yes/No]
- Missing information: [list what's missing if applicable]

### Recommendation

[Explain your recommendation with reasoning]

**Suggested Action**:
- [Keep open and continue triage]
- [Request more information from author]
- [Close with reason: ...]
- [Redirect to repository: ...]

**Suggested Comment** (if closing or redirecting):
```
[Draft a helpful comment explaining why this should be closed/redirected]
```
```

### Step 5: Commit Findings

Save your analysis to the triage document:

```bash
git add ISSUE-TRIAGE.md
git commit -m "Initial validation for issue #{issue_number}: [ASSESSMENT]"
```

## Example Analysis Format

### For a Legitimate Bug

```markdown
**Assessment**: LEGITIMATE

The issue describes a race condition in Tide's merge logic when GitHub Actions are re-triggered. This is a bug in pkg/tide/status.go which exists in this repository. The issue provides:
- Clear description of the problem
- Example PR demonstrating the issue
- Reference to suspected code location
- Categorization as kind/bug is appropriate

**Recommendation**: Keep open and continue triage. This is a valid bug report for Tide component.
```

### For a Misconfiguration

```markdown
**Assessment**: CLOSE

This issue describes problems with a specific Prow deployment's configuration, not a bug in Prow itself. The error messages indicate incorrect ProwJob YAML syntax, which is a configuration issue that should be resolved by the deployment owner.

**Suggested Comment**:
"This appears to be a configuration issue rather than a bug in Prow. Please review your ProwJob YAML syntax and ensure it matches the schema documented at [link]. If you believe this is actually a Prow bug after investigating, please reopen with reproduction steps. For configuration help, please use the Kubernetes Slack #prow channel."
```

### For Wrong Repository

```markdown
**Assessment**: REDIRECT

This issue reports a bug in Kubernetes test-infra tooling, not in Prow itself. The component mentioned (kubetest2) lives in kubernetes/test-infra repository.

**Suggested Comment**:
"This issue should be filed in the kubernetes/test-infra repository at https://github.com/kubernetes/test-infra/issues as it relates to kubetest2, not Prow. Please open an issue there and I'll close this one."
```

## Important Notes

- Be thorough but fair in your assessment
- If unsure, lean toward NEEDS_INFO rather than closing immediately
- Always provide constructive feedback in suggested comments
- Consider the issue author's experience level when drafting responses
- Check existing labels - they may indicate previous triage efforts
