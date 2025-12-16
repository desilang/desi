# Versioned Documentation with Mike

This guide explains how to manage versioned documentation for The Desi Book.

## Overview

We use [mike](https://github.com/jimporter/mike) for versioning, which:
- Maintains multiple doc versions in gh-pages branch
- Provides version selector dropdown
- Supports aliases (stable, latest, dev)

## Setup

### Install Dependencies

```bash
pip install mkdocs-material mike
```

## Workflow

### Deploying a New Version

When releasing a new version (e.g., v0.1.0):

```bash
cd book

# Deploy version 0.1.0 with alias "stable"
mike deploy --push --update-aliases 0.1.0 stable

# Set 0.1.0 as the default
mike set-default --push stable
```

### Deploying a Patch Release

For bug fixes (e.g., v0.1.1):

```bash
# Just update the changelog and redeploy
mike deploy --push --update-aliases 0.1.1 stable
```

### Deploying a New Minor/Major Version

For v0.2.0 with breaking changes:

```bash
# Deploy new version (keeps 0.1.x available)
mike deploy --push --update-aliases 0.2.0 stable

# Old versions remain accessible at /0.1.0/
```

### Development Docs

For unreleased changes:

```bash
mike deploy --push dev
```

## Version Aliases

| Alias | Points To | Description |
|-------|-----------|-------------|
| `stable` | Latest stable release | Recommended for users |
| `latest` | Newest version | Same as stable usually |
| `dev` | Development | Unreleased features |

## Local Preview

Preview a specific version:

```bash
mike serve
```

This serves all versions with the version selector.

## Directory Structure

After deploying multiple versions, gh-pages looks like:

```
gh-pages branch:
├── 0.1.0/         # v0.1.0 docs
├── 0.1.1/         # v0.1.1 docs  
├── 0.2.0/         # v0.2.0 docs
├── stable/        # Redirect to latest stable
├── dev/           # Development docs
└── versions.json  # Version metadata
```

## GitHub Actions Automation

Add this workflow for automatic deployment on release:

```yaml
# .github/workflows/docs.yml
name: Deploy Docs

on:
  push:
    branches: [main]
    paths: ['book/**']
  release:
    types: [published]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Needed for mike
      
      - uses: actions/setup-python@v4
        with:
          python-version: '3.x'
      
      - run: pip install mkdocs-material mike
      
      - name: Configure Git
        run: |
          git config user.name github-actions
          git config user.email github-actions@github.com
      
      - name: Deploy dev docs (on push)
        if: github.event_name == 'push'
        run: |
          cd book
          mike deploy --push dev
      
      - name: Deploy release docs
        if: github.event_name == 'release'
        run: |
          cd book
          VERSION=${GITHUB_REF#refs/tags/v}
          mike deploy --push --update-aliases $VERSION stable
          mike set-default --push stable
```

## Making Version-Specific Changes

When making changes for a new version:

1. **Update content** in `book/docs/`
2. **Update changelog** in `book/docs/changelog.md`
3. **Deploy** with the new version number

Old versions remain frozen in their deployed state.

## Example: Updating Dict Syntax in v0.2.0

1. Update `tutorials/collections.md` with new syntax
2. Add to changelog:
   ```markdown
   ## [0.2.0] - 2025-XX-XX
   
   ### Changed
   - **Breaking**: Dict literal syntax changed from `{k: v}` to `{k = v}`
   
   ### Migration
   - Replace all `{key: value}` with `{key = value}` in your code
   ```
3. Deploy:
   ```bash
   mike deploy --push --update-aliases 0.2.0 stable
   ```
4. Users can still access v0.1.x docs at `/0.1.0/`
