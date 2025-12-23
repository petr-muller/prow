# Subcommand: maintenance::issues::triage::wrapup

## Purpose

Wrap up the triage process by pushing branches to origin and offering to post the augmentation comment to the GitHub issue. This is the final step after completing all triage subcommands (initial, research, assess-effort, augment, brief).

## Parameters

- `issue_number` (required): The GitHub issue number being triaged

## Instructions

You are wrapping up triage for GitHub issue #{issue_number}.

### Step 1: Verify Completion

Ensure all previous triage steps have been completed:
- ✓ Initial validation
- ✓ Code research
- ✓ Effort assessment
- ✓ Augmentation proposed
- ✓ Briefing completed (if applicable)

If any are missing, inform the user and suggest completing them first.

### Step 2: Push Branches to Origin

Push both the maintenance helpers branch and the issue-specific triage branch:

```bash
# Push claude-maintenance-helpers
git checkout claude-maintenance-helpers
git push origin claude-maintenance-helpers

# Push issue triage branch with tracking
git checkout issue-triage-{issue_number}
git push -u origin issue-triage-{issue_number}
```

Confirm to the user:
```
✓ Pushed claude-maintenance-helpers to origin
✓ Pushed issue-triage-{issue_number} to origin with tracking
```

### Step 3: Retrieve and Construct the Comment

1. **Read the proposed augmentation comment** from the "Proposed GitHub Comment" section in ISSUE-TRIAGE.md

2. **Check if comment should be posted**:
   - Read the "Should This Comment Be Posted?" section
   - If recommendation is "No" or "Skip", inform user and ask if they want to proceed anyway

3. **Construct the full comment** by combining:
   - The augmentation comment content
   - The boilerplate footer with links

**Template**:
```markdown
[Content from "Proposed GitHub Comment" section - exactly as written, including any /retitle, paragraphs, and label commands]

<details>
<summary>Triage information</summary>

This comment was made by experimental [Claude triage helper](https://github.com/petr-muller/prow/blob/claude-maintenance-helpers/.claude/skills/maintenance-issue-triage/SKILL.md). I reviewed the content and I hope it is useful and not AI slop. If you have feedback please reach out to me.

Full triage: https://github.com/petr-muller/prow/blob/issue-triage-{issue_number}/ISSUE-TRIAGE.md
</details>
```

### Step 4: Present Comment for Review

Show the complete comment to the user:

```
I can post the following comment to issue #{issue_number}:

---
[Display the complete formatted comment exactly as it will appear]
---

Would you like me to post this comment to the issue?
```

### Step 5: Wait for Confirmation

Wait for user response:
- **"yes" / "post it" / "go ahead"** → Proceed to post
- **"no" / "don't post" / "skip"** → Acknowledge and skip posting
- **"edit" / "change it"** → Ask what they'd like to change, but note this is just for posting, not updating the triage doc
- **Question/concern** → Answer and re-ask for confirmation

### Step 6: Post the Comment (if confirmed)

If user confirms, post using `gh` CLI:

```bash
gh issue comment {issue_number} --body "[full comment text with proper escaping]"
```

**Important**: Properly escape the comment text for the shell:
- Use a heredoc for complex formatting
- Ensure newlines are preserved
- Don't break on special characters

Example:
```bash
gh issue comment {issue_number} --body "$(cat <<'EOF'
[full comment text here]
EOF
)"
```

### Step 7: Confirm Success

After posting:

1. **Get the comment URL** from `gh` output or construct it:
   `https://github.com/kubernetes-sigs/prow/issues/{issue_number}#issuecomment-XXXXX`

2. **Confirm to user**:
   ```
   ✓ Comment posted successfully!

   View it at: [comment URL]
   ```

3. **Update ISSUE-TRIAGE.md** with posting confirmation:
   ```markdown
   ## Comment Posted

   Posted augmentation comment on: [date/time]
   Comment URL: [URL]
   ```

4. **Commit the update**:
   ```bash
   git add ISSUE-TRIAGE.md
   git commit -m "Posted augmentation comment to issue #{issue_number}"
   git push origin issue-triage-{issue_number}
   ```

### Step 8: Final Summary

Provide a final summary to the user:

```
🎉 Triage complete for issue #{issue_number}!

Summary:
- Initial validation: [LEGITIMATE/NEEDS_INFO/CLOSE/REDIRECT]
- Root cause: [one sentence]
- Effort level: [1/2/3/4]
- Comment posted: [Yes/No]

Branches:
- claude-maintenance-helpers: Synced with origin
- issue-triage-{issue_number}: Synced with origin

Full triage document:
https://github.com/petr-muller/prow/blob/issue-triage-{issue_number}/ISSUE-TRIAGE.md

[If comment posted:]
GitHub comment:
[comment URL]
```

---

## Special Cases

### Comment Not Recommended

If the augmentation proposed a "No" for posting:

```
Note: The augmentation analysis recommends NOT posting this comment because:
[reason from triage doc]

Would you still like to post it anyway? (yes/no)
```

### No Augmentation Proposed

If no augmentation comment was created during triage:

```
No augmentation comment was proposed during triage. This might happen if:
- Issue should be closed/redirected
- Issue is already complete and well-documented
- Augmentation subcommand wasn't run

Would you like to:
1. Run the augmentation subcommand first
2. Skip posting and just wrap up
3. Write a custom comment now
```

### Posting Fails

If `gh issue comment` fails:

1. **Show the error** to the user
2. **Suggest troubleshooting**:
   - Check GitHub authentication: `gh auth status`
   - Verify issue exists and is accessible
   - Check for rate limiting
3. **Save the comment** to a file for manual posting:
   ```bash
   cat > /tmp/issue-{issue_number}-comment.md <<'EOF'
   [comment content]
   EOF
   ```
4. **Inform user**:
   ```
   ✗ Failed to post comment automatically.

   The comment has been saved to: /tmp/issue-{issue_number}-comment.md

   You can post it manually by:
   1. Copying the content from that file
   2. Going to: https://github.com/kubernetes-sigs/prow/issues/{issue_number}
   3. Pasting it as a new comment
   ```

---

## Example Walkthrough

**User triggers wrapup**:
```
User: Let's wrap this up
```

**Agent verifies and pushes**:
```
Agent: Wrapping up triage for issue #337...

✓ All triage steps completed
✓ Pushed claude-maintenance-helpers to origin
✓ Pushed issue-triage-337 to origin with tracking

I can post the following comment to issue #337:

---
## Root Cause

This is a race condition in Tide's context checking logic. When a GitHub Action is re-triggered, GitHub temporarily **removes** the old CheckRun from its API before creating the new one. During this brief window (typically a few seconds), the required check is completely missing from GitHub's status API. If Tide's sync loop runs during this window, it sees "no unsuccessful contexts" and incorrectly proceeds with the merge.

## Technical Details

The issue occurs in the context evaluation flow (`pkg/tide/tide.go:865-889`). Tide doesn't currently track which contexts were previously seen for a commit. When a required context disappears, there's no way to distinguish between "context never existed" (might be legitimate) versus "context disappeared" (suspicious, likely a re-trigger). This allows the race window where GitHub has removed the old CheckRun but hasn't created the new one yet.

PR #563 addresses this by maintaining state of previously-seen contexts per PR/commit. When a context that was previously observed disappears, it's treated as PENDING rather than missing, preventing premature merges during the re-trigger window.

/priority important-soon

<details>
<summary>Triage information</summary>

This comment was made by experimental [Claude triage helper](https://github.com/petr-muller/prow/blob/claude-maintenance-helpers/.claude/skills/maintenance-issue-triage/SKILL.md). I reviewed the content and I hope it is useful and not AI slop. If you have feedback please reach out to me.

Full triage: https://github.com/petr-muller/prow/blob/issue-triage-337/ISSUE-TRIAGE.md
</details>
---

Would you like me to post this comment to the issue?
```

**User confirms**:
```
User: yes
```

**Agent posts and confirms**:
```
Agent: ✓ Comment posted successfully!

View it at: https://github.com/kubernetes-sigs/prow/issues/337#issuecomment-XXXXX

🎉 Triage complete for issue #337!

Summary:
- Initial validation: LEGITIMATE
- Root cause: GitHub removes CheckRun during re-trigger, creating race window
- Effort level: 3 - Large (requires expertise)
- Comment posted: Yes

Branches:
- claude-maintenance-helpers: Synced with origin
- issue-triage-337: Synced with origin

Full triage document:
https://github.com/petr-muller/prow/blob/issue-triage-337/ISSUE-TRIAGE.md

GitHub comment:
https://github.com/kubernetes-sigs/prow/issues/337#issuecomment-XXXXX
```

---

## Important Notes

- **Always verify** the comment looks correct before posting - this will be public
- **Respect the user's decision** if they say no - don't push to post
- **Handle errors gracefully** - provide fallback options if posting fails
- **Keep the boilerplate** exactly as specified - it provides context and attribution
- **Test the gh command** before using it to ensure proper escaping
