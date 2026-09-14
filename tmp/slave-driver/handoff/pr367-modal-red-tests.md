# T-367 one-shot secret modal — RED handoff

Scope remains test-only: `src/developer-portal/src/test/AppBotRegistration.test.tsx`.

Acceptance contract:

- Opening focuses the first dialog action; forward and reverse Tab stay within the dialog; `Close and clear` clears both secrets and returns focus to its opener.
- A modal removes the entire portal `main` from the accessibility tree and interaction order with `inert` plus `aria-hidden="true"`; the dialog must be its sibling rather than a descendant. Representative background controls (`Rotate webhook secret`, `Sign out`) are inaccessible while it is open, and reverse Tab remains in the dialog. Logout remains an explicit allowed exit and clears both secrets.
- Clipboard API rejection and absence retain the secret and expose the failure through an accessible status.

RED evidence (2026-09-14): `npm test -- --run src/test/AppBotRegistration.test.tsx` reports failures for initial focus/focus wrapping, whole-background modal isolation, and accessible status. The whole-background test intentionally fails before checking representative controls because the current `main` has neither `inert` nor `aria-hidden`; it also currently contains the dialog. Logout clearing already passes.

The portal does not currently declare `@testing-library/user-event`; the existing portal test suite has no user-event helper, so its keyboard assertions use the established Testing Library keyboard dispatch. Add the package and convert these assertions to `userEvent.tab()` only if the next owner is authorized to change the package manifest and lockfile; this test-only correction deliberately does not expand its write scope.
