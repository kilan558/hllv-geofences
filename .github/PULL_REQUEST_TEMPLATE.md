## Summary

Describe the operator-visible change and why it is needed.

## Verification

List the exact commands run and their results. Do not claim checks that were not
executed.

## Checklist

- [ ] The change preserves the standalone Vietnam-only Warfare scope.
- [ ] Behavior changes include a regression test that failed before the fix.
- [ ] `go test ./... -count=1`, `go vet ./...`, `npm test`, and `npm run check` pass.
- [ ] Docker and configuration generation were tested when affected.
- [ ] No `.env`, credentials, real addresses, Discord IDs, player identifiers,
      generated policies, runtime state, or unredacted logs are included.
- [ ] New or updated dependencies have compatible licenses and
      `THIRD_PARTY_NOTICES.md` is current.
- [ ] Existing copyright, license, and attribution notices remain intact.
- [ ] Documentation and public runtime text are in English.
