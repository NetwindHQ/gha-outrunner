# Release Pipeline

This document describes how outrunner releases are built, signed, and distributed.

## Overview

Pushing a version tag (`v*`) triggers the release workflow which:

1. Builds binaries for Linux (amd64, arm64) and macOS (arm64)
2. Packages deb and rpm with signed GPG signatures
3. Signs the checksums file
4. Creates a GitHub Release with all artifacts
5. Updates the Homebrew formula in `NetwindHQ/homebrew-tap`

## Signing

All release artifacts are signed with a GPG ed25519 key. The master key is kept offline; only the signing subkey is used in CI.

**Key locations:**

| Secret | Where | Purpose |
|--------|-------|---------|
| `GPG_SIGNING_KEY` | GitHub repo secrets | Signing subkey (ci-signing-key.asc) — signs deb, rpm, and checksums |
| `GPG_FINGERPRINT` | GitHub repo secrets | Signing subkey fingerprint — used by GoReleaser for checksums signing |
| `GPG_PRIVATE_KEY` | Cloudflare Workers secrets (pkg worker) | Same signing subkey — used by reprox to sign repo metadata |

The master key and a backup of the public key are stored offline on an air-gapped USB drive. The revocation certificate can be regenerated from the master key.

**What gets signed:**

- **deb packages**: embedded GPG signature via debsign (verified by apt)
- **rpm packages**: embedded GPG signature via rpmsign (verified by dnf)
- **checksums.txt**: detached GPG signature (checksums.txt.sig)
- **apt repo metadata**: InRelease, Release.gpg (signed by reprox using the same key)
- **rpm repo metadata**: repomd.xml.asc (signed by reprox using the same key)

## Distribution Channels

### GitHub Releases

All artifacts (tar.gz, deb, rpm, checksums) are uploaded to GitHub Releases by GoReleaser.

### apt (Debian/Ubuntu)

Served by [reprox](https://github.com/leoherzog/reprox) running as a Cloudflare Worker at `pkg.netwind.pl`. Reprox reads GitHub Releases and generates compliant apt repo metadata on the fly. Packages are redirected to GitHub's CDN.

- Repo URL: `https://pkg.netwind.pl/NetwindHQ/gha-outrunner`
- Public key: `https://pkg.netwind.pl/NetwindHQ/gha-outrunner/public.key`

The deb postinstall script automatically adds the apt source and imports the GPG key, so future updates come through `apt upgrade`. Users can skip this with `OUTRUNNER_NO_REPO=1`.

### rpm (Fedora/RHEL/CentOS)

Same reprox instance generates rpm repodata. Users add the repo manually before installing:

```bash
sudo dnf config-manager addrepo --from-repofile=https://pkg.netwind.pl/NetwindHQ/gha-outrunner/outrunner.repo
sudo dnf install outrunner
```

dnf auto-imports the GPG key on first install.

### Homebrew (macOS/Linux)

The release workflow runs `scripts/update-homebrew.sh` which updates the formula in `NetwindHQ/homebrew-tap` via the GitHub API. It updates the version and SHA256 checksums for all three archive variants.

Requires the `TAP_GITHUB_TOKEN` secret with write access to the homebrew-tap repo.

## How to Release

### Release candidate

```bash
git tag v1.0.1-rc1
git push origin v1.0.1-rc1
```

RC tags trigger the full pipeline including Homebrew updates. Test the artifacts before tagging the final release. Delete RC releases and tags after the final release.

### Final release

```bash
git tag v1.0.1
git push origin v1.0.1
```

### Clean up old RCs

```bash
# Delete RC releases and tags
for tag in $(gh release list --json tagName --jq '.[].tagName' | grep rc); do
  gh release delete "$tag" --yes
  git push origin ":refs/tags/$tag"
  git tag -d "$tag"
done
```

Note: reprox caches release metadata for 24 hours. Bust the cache after cleanup:

```
https://pkg.netwind.pl/NetwindHQ/gha-outrunner/dists/stable/InRelease?cache=false
https://pkg.netwind.pl/NetwindHQ/gha-outrunner/repodata/primary.xml.gz?cache=false
```

## Infrastructure

### reprox (pkg.netwind.pl)

- Runs as Cloudflare Worker `pkg` in the Netwind account
- Source: `NetwindHQ/pkg` (fork of [leoherzog/reprox](https://github.com/leoherzog/reprox))
- Restricted to `NetwindHQ` repos via `ALLOWED_OWNERS` env var
- Paid Workers plan for higher subrequest limits (needed for repos with many releases)
- Secrets: `GPG_PRIVATE_KEY`, `GITHUB_TOKEN`

### Homebrew tap

- Repo: `NetwindHQ/homebrew-tap`
- Formula: `Formula/outrunner.rb`
- Updated automatically by the release workflow via `scripts/update-homebrew.sh`

## Subkey Rotation

When the signing subkey expires (2-year validity):

1. Boot the air-gapped machine with the master key backup
2. Import the master key: `gpg --import master-key.asc`
3. Add a new signing subkey: `gpg --quick-add-key <MASTER_FINGERPRINT> ed25519 sign 2y`
4. Export the new subkey: `gpg --armor --export-secret-subkeys > ci-signing-key.asc`
5. Export the updated public key: `gpg --armor --export > public-key.asc`
6. Update secrets: `GPG_SIGNING_KEY` in GitHub, `GPG_PRIVATE_KEY` in Cloudflare
7. Update `GPG_FINGERPRINT` in GitHub if the subkey fingerprint changed
8. Back up the updated master key
