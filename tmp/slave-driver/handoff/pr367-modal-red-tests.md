# T-367 one-shot secret modal — RED handoff

Scope is test-only: `src/developer-portal/src/test/AppBotRegistration.test.tsx`.

Acceptance coverage added:

- Opening the secret dialog focuses the first dialog control; Tab and Shift+Tab cycle within it; Escape clears both secrets, closes the dialog, and restores the invoking `Register bot` button.
- A background `Sign out` click has no effect while the modal is open.
- A rejected Clipboard API call retains the secret and renders its failure status.

Current UI lacks focus lifecycle, keyboard trapping/Escape handling, and background inertness, so the focused test run is expected to be red. The clipboard-failure test documents already-present behavior. Implementation must not weaken these assertions.
