<!-- Describe what this PR changes and why. -->

## Summary

## Checklist

- [ ] Edited YANG (source of truth), not generated output by hand
- [ ] Ran `make gen` and committed the regenerated artifacts (`make check-gen` is clean)
- [ ] Bumped the `revision` date of any YANG module whose content changed
- [ ] `go build ./... && go vet ./... && go test ./...` pass
- [ ] `buf lint` passes; no unintended `buf breaking` violations vs. `main`
- [ ] Ran the relevant `make check-*` / conformance gates for the area touched
- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:` / `fix:` / `feat!:`) — the changelog is generated from them, don't hand-edit `CHANGELOG.md`
