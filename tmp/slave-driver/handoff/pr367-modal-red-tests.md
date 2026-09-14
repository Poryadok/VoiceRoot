# T-367 one-shot secret modal — RED handoff

Scope remains test-only: `src/developer-portal/src/test/AppBotRegistration.test.tsx`.

Acceptance contract:

- Opening focuses the first dialog action; forward and reverse Tab stay within the dialog; `Close and clear` clears both secrets and returns focus to its opener.
- A modal removes the entire portal `main` from the accessibility tree and interaction order with `inert` plus `aria-hidden="true"`; the dialog must be its sibling rather than a descendant. Representative background controls (`Rotate webhook secret`, `Sign out`) are inaccessible while it is open, and reverse Tab remains in the dialog. `Sign out` is available only after `Close and clear` or `Escape` clears both secrets, removes isolation, and restores the opener.
- Clipboard API rejection and absence retain the secret and expose the failure through an accessible status.

RED evidence (2026-09-14): `npm test -- --run src/test/AppBotRegistration.test.tsx` reports the retained partial-copy focus failure, the background `Sign out` interaction failure, and the clipboard status failure. The whole-background test keeps `Rotate webhook secret` and `Sign out` as DOM assertions (`hidden: true` once the background is hidden), traverses reverse Tab with `userEvent.tab({ shift: true })`, and uses `userEvent.click` to prove that neither background action changes state or closes the dialog. The close-then-logout test proves that `Close and clear` removes both secrets and isolation, restores its opener, and only then makes normal logout available. The Escape test independently keeps the equivalent clear-and-restore path.

`@testing-library/user-event` `^14.6.7` is now a scoped Developer Portal devDependency (manifest and lockfile updated). Keyboard traversal tests use `userEvent.tab()` / `userEvent.tab({ shift: true })`; no production files were changed.
