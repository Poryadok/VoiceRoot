# T-367 one-shot secret modal — RED handoff

Scope remains test-only: `src/developer-portal/src/test/AppBotRegistration.test.tsx`.

Acceptance contract:

- Opening focuses the first dialog action; forward and reverse Tab stay within the dialog; `Close and clear` clears both secrets and returns focus to its opener.
- A modal removes the entire portal `main` from the accessibility tree and interaction order with `inert` plus `aria-hidden="true"`; the dialog must be its sibling rather than a descendant. Representative background controls (`Rotate webhook secret`, `Sign out`) are inaccessible while it is open, and reverse Tab remains in the dialog. Logout remains an explicit allowed exit and clears both secrets.
- Clipboard API rejection and absence retain the secret and expose the failure through an accessible status.

RED evidence (2026-09-14): `npm test -- --run src/test/AppBotRegistration.test.tsx` reports four failures: real `userEvent.tab()` traversal exposes missing initial focus/focus wrapping; the modal background has neither `inert` nor `aria-hidden`; and the two clipboard failures lack an accessible `status`. The whole-background test now keeps `Rotate webhook secret` and `Sign out` as DOM assertions (`hidden: true` once the background is hidden), traverses reverse Tab with `userEvent.tab({ shift: true })`, and clicks Rotate to require no extra request while the dialog is open. It intentionally stops at the missing `inert` assertion on the current production tree, where the dialog is still inside `main`. Logout clearing passes.

`@testing-library/user-event` `^14.6.7` is now a scoped Developer Portal devDependency (manifest and lockfile updated). Keyboard traversal tests use `userEvent.tab()` / `userEvent.tab({ shift: true })`; no production files were changed.
