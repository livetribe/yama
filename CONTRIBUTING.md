# Contributing to Yama

## Get Yama

Add Yama to a Go module with `go get l7e.io/yama`. The
[README](README.md#setup) gives the full setup steps.

## Report a bug

Report a bug in a GitHub issue:

https://github.com/livetribe/yama/issues/new

Include these items in the report:

- The version of Yama and the version of Go.
- The steps that show the problem.
- The result that you expected and the result that you got.

Do not report a security vulnerability in a public issue. Use the process in
[SECURITY.md](SECURITY.md).

## Ask for a feature

Open a GitHub issue that describes the problem to solve. Open the issue
before you write the code. A change to the public API or to the lifecycle
behavior needs agreement before work starts.

## Send a change

1. Fork the repository and make a branch from `master`.
2. Make the change.
3. Run `make check`. This command runs the tests with the race detector, the
   linter, and the format check.
4. Open a pull request against `master`. Refer to the related issue in the
   description.

The maintainer reviews each pull request. CI must pass before a merge.

## Requirements for a change

A change must obey the project conventions in
[CODING_STANDARDS.md](CODING_STANDARDS.md) and [AGENTS.md](AGENTS.md). These
conventions apply to all contributors. Their baseline is the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

- Add tests for new behavior. Add a test that shows the bug for a bug fix.
- Keep `make check` passing.
- Write code comments in ASD-STE100 Simplified Technical English.
- Update the API surface golden file when you change an exported symbol.
  [AGENTS.md](AGENTS.md#testing) gives the command.
- Write or update an ADR in `docs/adr/` when the change makes a design
  decision.
- Write commit messages that obey the
  [`git-commit-message` skill](.claude/skills/git-commit-message/SKILL.md).

## License

Yama uses the [Apache License 2.0](LICENSE). When you send a contribution, you
release it under this license.
