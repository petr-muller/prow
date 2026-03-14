# Triage for Issue #477

**Status**: In Progress
**Created**: 2026-03-14

## Issue Information

- **Issue Number**: #477
- **Issue URL**: https://github.com/kubernetes-sigs/prow/issues/477
- **Title**: branchprotector: excluded branches retain existing protection instead of being removed
- **State**: CLOSED (auto-closed by lifecycle bot, not resolved)
- **Author**: kaovilai

## Initial Validation

**Assessment**: LEGITIMATE

### Analysis

The issue describes a real bug in `cmd/branchprotector/protect.go`. When branches are added to the `exclude` list in branchprotector configuration, existing GitHub branch protection is not removed from those branches. The tool only stops applying new protection rules.

**Issue Category**: Bug

**Repository Scope Check**:
- Component mentioned: branchprotector (`cmd/branchprotector`)
- Exists in this repo: Yes
- Relevant code paths: `cmd/branchprotector/protect.go` lines 341-343 (exclusion filter), lines 182-188 (removal logic)

**Information Completeness**:
- Sufficient detail provided: Yes
- The issue includes: clear description, impact with error logs, root cause analysis with exact line numbers, workaround, and even a suggested code change in the first comment

### Code Verification

The bug is confirmed by reading the code:

1. In `UpdateRepo()` (line 341-343), excluded branches hit `continue` and are never added to the `branches` map:
   ```go
   } else if !ok && branchExclusions != nil && branchExclusions.MatchString(b.Name) {
       logrus.Infof("%s/%s=%s: excluded", orgName, repoName, b.Name)
       continue  // Branch skipped entirely
   }
   ```

2. The removal mechanism in `configureBranches()` (line 184-188) triggers when `Request` is `nil`, but excluded branches never reach `UpdateBranch()` at all, so no `requirements` with `Request: nil` is ever sent for them.

3. The issue was auto-closed by the k8s lifecycle bot (stale -> rotten -> closed), NOT because it was resolved. The author even tried to keep it alive by removing lifecycle/rotten once.

### Recommendation

Keep open (reopen) and continue triage. This is a valid, well-documented bug with a clear root cause and suggested fix. The auto-closure by the lifecycle bot should not be treated as resolution.

**Suggested Action**:
- Reopen the issue
- Continue triage with research, effort assessment, and augmentation

## Findings

(Additional findings from triage subcommands will be added below)

## Next Steps

- [ ] Research: Deep-dive into code paths and solution approaches
- [ ] Assess effort: Determine complexity level
- [ ] Augment: Improve issue with triage findings
- [ ] Brief: Walk maintainer through findings
- [ ] Wrapup: Push and post comment
