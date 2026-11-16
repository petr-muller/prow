Draft comment for https://github.com/kubernetes-sigs/prow/issues/944

Posting is pending explicit user authorization.

This increment removes the static template theme setting and helpers, and moves
default logo selection entirely into CSS. The index uses its existing
`html.dark-mode` state; Spyglass uses its existing page selector. Configured
branding logos remain independent of theme selection.

The rollout policy preserves current behavior: index supports system preference
and a persistent toggle; Spyglass remains forced-dark; PR status, Tide, Command
Help, Plugins, history, and configured-jobs pages remain light. The policy is
also documented in Deck's Appearance documentation.

Keep this issue open for broader dynamic theme support. That rollout needs
page-specific styling and validation; making Spyglass switchable additionally
needs review of its lens and iframe styling. Neither is included in this logo
consolidation. Existing colors and branding color semantics are preserved.
