## What and why

<!-- One or two sentences. -->

## Evidence

- [ ] `gofmt -l` on the files I changed prints nothing, `go vet` is clean, `go build ./... && go test ./...` pass
- [ ] Gameplay change: I added or updated a scenario in `scripts/verify.d/` and ran it alone with `scripts/verify.sh` (name it here: )
- [ ] New claims about the original game carry an evidence level in comments (`VERIFIED`, `inferred`, `UNVERIFIED`) and cite the Ghidra name and address
- [ ] Docs updated if needed (`docs/TESTING.md` for new `OD2_*` variables or scenarios, `docs/STATUS_MATRIX.md`, `docs/KNOWN_GAPS.md`)

## Hygiene

- [ ] No game files, extracted tables, saves, CD keys, screenshots of game data or decompiled Blizzard code (`scripts/check_repo_hygiene.sh` passes)
- [ ] I only changed what this task concerns (no drive-by reformatting)

See [CONTRIBUTING.md](../CONTRIBUTING.md).
