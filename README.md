# Heimdal CLI

Heimdal CLI lets customers run AI Lab Auto Run Tests directly from terminal.

## Install

### macOS, Linux, WSL

```bash
curl -fsSL https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.sh | bash
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.ps1 | iex
```

### Windows CMD

```cmd
curl -fsSL https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.cmd -o install.cmd && install.cmd && del install.cmd
```

### Python (optional)

```bash
pip install coone-ailab-cli
```

## Quick Start

1. Login

```bash
heimdal login
```

2. Select organization

```bash
heimdal org list
heimdal org use <org-id>
```

3. Select project

```bash
heimdal projects
heimdal use <project-id>
```

4. Initialize config

```bash
heimdal init --integration <integration-id>
```

5. Validate config

```bash
heimdal config validate --file ./heimdal.yaml
```

6. Run a test

```bash
heimdal test auto --test-id AT-01 --scenario A
```

## Common Commands

```bash
heimdal test auto --test-id CS-01 --scenario A
heimdal auto runs --project <project-id>
heimdal auto results <test-run-id>
heimdal auto datasets --project <project-id> --test-id <test-id>
heimdal integrations --project <project-id>
heimdal knowledge-bases --project <project-id>
```

## Notes

- `integration.endpoint` in `heimdal.yaml` is your model endpoint.
- `heimdal` and `coval` commands both work.
- Use your own account token via `heimdal login`.
