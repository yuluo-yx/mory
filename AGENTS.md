# Mory Codex Guide

## Scope

These instructions apply to the entire repository. More specific `AGENTS.md` files may extend them for a subdirectory but must not weaken the quality requirements below.

## Product principles

- Preserve Mory as a cross-platform Markdown editor for macOS and Windows.
- Keep the shared web editor behavior consistent across the native macOS host, Windows WebView2 host, and Electron compatibility host.
- Preserve Simplified Chinese and English interface switching for every user-facing feature.
- Prefer established, maintained libraries and platform APIs over custom replacements.
- Keep local workspaces as the default. Remote storage providers remain optional plugins.
- Treat live Markdown rendering, themes, Mermaid, code highlighting, workspace files, backlinks, knowledge graphs, and export as core behavior.

## Language rules

- Write source code, code comments, test names, assertion messages, logs, build metadata, workflow names, and developer documentation in English.
- Chinese text is allowed only for user-facing Simplified Chinese localization and explicit CJK behavior fixtures.
- Encode CJK test fixtures with Unicode escapes when the test file must remain English-only.
- Update the English localization whenever a Chinese user-facing string is added or changed.
- Run `npm run check:language` before delivery.

## Implementation rules

- Inspect the complete call path before changing behavior. Cross-platform features often require coordinated web, macOS, Windows, Electron, and packaging changes.
- Keep editable sources separate from generated artifacts. Edit `Sources/Mory/Web/app.js` and its modules, then regenerate `app.bundle.js` with `npm run build:web`.
- Never edit vendored runtimes or generated bundles by hand.
- Preserve user data and unrelated working-tree changes. Do not use destructive Git commands.
- Use small, explicit functions and platform-neutral data contracts. Avoid placeholder implementations and `NotImplemented` branches.
- Keep credentials out of renderer state, logs, fixtures, commits, and screenshots.
- Do not copy proprietary Typora source code or bundled assets. Reproduce observable behavior with an independent implementation.

## Test-follow rule

- Every functional change must include or update an automated regression test in the same change.
- Tests must verify observable behavior rather than implementation details whenever practical.
- Add pure unit tests for parsers, formatters, and data transformations.
- Add host tests for macOS, Windows, or Electron contract changes.
- Add interaction coverage for editor controls and user workflows.
- Bug fixes must include a test that fails without the fix.
- Keep tests deterministic, isolated, and independent of execution order.
- Maintain at least 90% line coverage for the JavaScript unit-test scope. Do not reduce existing coverage.

## Required verification

Run the checks relevant to the changed surface, with these commands as the default baseline:

```sh
npm run check
npm test
env GOCACHE="$PWD/.cache/go-build" go test ./...
git diff --check
```

Also run the matching interaction or native smoke tests for affected UI and platform behavior. Report any test that cannot run and the exact reason.

## Git and delivery

- Do not commit, tag, push, publish a release, or change remote state unless the user explicitly requests it.
- Use English Conventional Commit messages.
- Before committing, review the staged diff and verify generated files are current.
- Before pushing, confirm the current branch and remote target.
- Summarize changed files, tests, known limits, and rollback scope in the delivery response.

## Repository hygiene

- Keep `docs/` for local development notes only; it remains ignored by Git.
- Keep the canonical application icon in `assets/`. Generate platform-specific icon artifacts during builds.
- Do not commit local caches, temporary exports, credentials, development SDKs, or generated release packages.
