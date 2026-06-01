# coone-ailab-cli

`coone-ailab-cli` is a lightweight Python bootstrap package for Heimdal CLI.

After installation, running `heimdal` downloads the matching Heimdal binary from
GitHub Releases (if needed), stores it in a local cache, and forwards all
arguments to the binary.

## Install

```bash
pip install coone-ailab-cli
```

## Recommended

```bash
pipx install coone-ailab-cli
```

## Environment Variables

- `HEIMDAL_REPO`: GitHub repository in `owner/name` format.  
  Default: `coone-ai/heimdal`
- `HEIMDAL_VERSION`: Release tag to pin (for example `v0.0.1`).  
  Default: latest release
- `HEIMDAL_INSTALL_CACHE_DIR`: Cache directory for downloaded binaries

## Publish

Quick release (version bump + commit + tag + push):

```bash
./scripts/release.sh 0.0.2
```

Local only (no push):

```bash
./scripts/release.sh 0.0.2 --no-push
```

Manual publish:

```bash
./scripts/publish.sh
```

CI publishing is also configured at:

- `.github/workflows/release.yml`

Tag format:

```bash
git tag v0.0.1
git push origin v0.0.1
```
