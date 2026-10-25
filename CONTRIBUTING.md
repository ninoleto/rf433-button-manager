# Contributing and forking

Start with [README.md](README.md), [the developer guide](docs/DEVELOPMENT.md) and
[the architecture](docs/ARCHITECTURE.md). Coding agents should also read
[AGENTS.md](AGENTS.md). Use a fork and a topic branch; preserve
the project's Git history. Do not force-push a shared branch or replace someone
else's work. A source package's bundle can restore history without overwriting
local edits using `bash scripts/restore-history.sh`.

Keep changes focused and describe the concrete trigger and resulting behavior.
Run `make check` and `make build` with the documented toolchain. Run the browser
suite when UI behavior changes, and relevant physical tests with an operator when
hardware behavior changes. Report actual commands, results and environment limits.
Do not represent synthetic USB/process tests as physical acceptance.

Include regression coverage for correctness and safety changes: identity ambiguity,
three-press learning, action suppression, busy limits, persistence, URL validation,
receiver ownership or recovery. Avoid tests that merely mirror trivial styling
edits. Update API/configuration/operations docs alongside behavior changes. Changes
to saved formats require compatibility, backup and rollback plans.

Use Go's formatter and existing naming conventions. Keep subprocess arguments
separate, secrets out of logs, rendered external data out of `innerHTML`, and real
RF/webhook endpoints out of automated tests. No new runtime service, package,
firewall, USB reset or unrelated automation change should be hidden in an installer.

Before submitting, inspect `git diff --check`, staged files and commit history.
Never include production databases, tokens, config exports, receiver captures or
private endpoint identifiers. Follow [SECURITY.md](SECURITY.md) for vulnerabilities.
An issue should include version/commit, expected and observed behavior, sanitized
steps and whether evidence is simulated or physical.

The application uses the [MIT license](LICENSE). Retain its copyright and license
notice in copies/forks. Contributions are provided under that same license. Third-party
dependencies retain their licenses; review notices when changing dependencies.
`rtl_433` is separately installed and not relicensed by this project.

To rename a fork's module, deliberately update `go.mod` and all internal import
paths, then run checks and update links/packaging metadata. Keeping the original
module path in a local fork is also valid; change it only when distribution needs
it. Preserve attribution and clearly identify incompatible behavior or unvalidated
hardware claims. See [release procedures](docs/RELEASING.md).
