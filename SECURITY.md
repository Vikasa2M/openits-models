# Security Policy

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report suspected vulnerabilities privately through GitHub's
[**Report a vulnerability**](https://github.com/Vikasa2M/openits-models/security/advisories/new)
flow (Security → Advisories → *Report a vulnerability*). This opens a private
advisory visible only to the maintainers.

Please include, as far as you can:

- the affected file(s) or component (a YANG module, generated proto, a
  `tools/` or `scripts/` program, or a CI workflow);
- a description of the issue and its impact;
- steps to reproduce, or a proof of concept.

We aim to acknowledge a report within a few business days and will keep you
updated as we investigate and prepare a fix. We'll credit reporters who wish
to be named once a fix is released.

## Scope

This repository is a **data-model layer** — YANG modules and the artifacts
generated from them (protobuf, Go, JSON Schema, AsyncAPI), plus in-repo
generation/validation tooling. Security-relevant reports we care about include:

- **Supply-chain / CI issues** — e.g. an unpinned or compromisable action,
  a workflow that could leak the repository token or secrets, or a way for a
  pull request to execute privileged code. Our workflows pin every action to a
  full commit SHA, run untrusted PR code only under a read-only token with
  `persist-credentials: false`, and never use `pull_request_target`; reports
  that defeat those controls are in scope.
- **Tooling vulnerabilities** — a crash, path traversal, or code-execution
  issue in a `tools/` or `scripts/` program when fed hostile input.
- **Model-integrity issues** — a way to bypass the validation/conformance
  gates such that non-conformant data validates as conformant.

Vulnerabilities in upstream dependencies (goyang, ygot, protobuf, libyang,
etc.) should be reported to those projects; tell us too if this repo's usage
makes an upstream issue exploitable here.

## Supported versions

The project is pre-1.0 (`v0.x`). Security fixes are made on `main` and released
in the next tag. See [`docs/versioning.md`](docs/versioning.md).
