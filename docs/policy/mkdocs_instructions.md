# MkDocs Instructions

Internal documentation for maintaining "The Desi Book" at [desilang.org](https://desilang.org).

---

## Overview

The Desi Book is built with:
- **MkDocs** - Static site generator
- **Material for MkDocs** - Theme
- **mike** - Versioning

**Production URL**: https://desilang.org

---

## Prerequisites

### Optional: Create Conda Environment

To avoid installing dependencies in your main Python environment, create a dedicated conda environment:

```bash
# Create environment
conda create -n desi-docs python=3.11 -y

# Activate environment
conda activate desi-docs
```

!!! tip "Remember to activate"
Always activate the environment before working on docs:
```bash
conda activate desi-docs
```

### Install Dependencies

```bash
pip install mkdocs-material mike
```

---

## Local Development

### Preview Docs

```bash
cd book
mkdocs serve
```

Open http://localhost:8000

### Preview with Versioning

```bash
cd book
mike serve
```

---

## Deployment

### First-Time Setup

1. Configure custom domain in GitHub Pages settings
2. Add CNAME file:
   ```bash
   echo "desilang.org" > book/docs/CNAME
   ```

### Deploy a Version

```bash
cd book

# Deploy v0.1.0 and mark as stable
mike deploy --push --update-aliases 0.1.0 stable

# Set as default (redirects desilang.org/ to /stable/)
mike set-default --push stable
```

### Deploy a Patch Release

```bash
# Update changelog, then:
mike deploy --push --update-aliases 0.1.1 stable
```

### Deploy a New Minor/Major Version

```bash
# v0.2.0 with breaking changes
mike deploy --push --update-aliases 0.2.0 stable

# Old version remains at desilang.org/0.1.0/
```

### Deploy Development Docs

```bash
mike deploy --push dev
# Available at desilang.org/dev/
```

---

## URL Structure

| URL | Content |
|-----|---------|
| `desilang.org/` | Redirects to stable |
| `desilang.org/stable/` | Latest stable version |
| `desilang.org/0.1.0/` | v0.1.0 docs (frozen) |
| `desilang.org/0.2.0/` | v0.2.0 docs |
| `desilang.org/dev/` | Development docs |

---

## Directory Structure

```
book/
├── mkdocs.yml           # Main config
├── VERSIONING.md        # This is copied here for reference
├── docs/
│   ├── CNAME            # Custom domain for GitHub Pages
│   ├── index.md         # Home page
│   ├── changelog.md     # Version history
│   ├── getting-started/
│   ├── tutorials/
│   ├── language/
│   ├── reference/
│   ├── examples/
│   └── stylesheets/
│       └── saffron.css  # Custom theme colors
└── overrides/           # Theme customizations
```

---

## Adding Content

### New Page

1. Create markdown file in appropriate directory
2. Add to `nav` section in `mkdocs.yml`

### Update mkdocs.yml Example

```yaml
nav:
  - Home: index.md
  - Getting Started:
    - Installation: getting-started/install.md
    - First Program: getting-started/first-program.md
    - New Page Here: getting-started/new-page.md  # Add this
```

---

## Version Management

### When to Create a New Version

| Change Type | Action |
|-------------|--------|
| Bug fix in docs | Redeploy same version |
| Bug fix release (0.1.1) | Deploy 0.1.1, update stable alias |
| New feature (0.2.0) | Deploy 0.2.0, update stable alias |
| Breaking change (1.0.0) | Deploy 1.0.0, update stable alias |

### Updating Existing Version

To fix typos in a deployed version:

```bash
# Edit the files, then redeploy
mike deploy --push 0.1.0
```

### Listing Versions

```bash
mike list
```

### Deleting a Version

```bash
mike delete --push 0.1.0-beta
```

---

## GitHub Pages Configuration

### Repository Settings

1. Go to Settings → Pages
2. Source: Deploy from branch `gh-pages`
3. Custom domain: `desilang.org`
4. Enforce HTTPS: ✓

### DNS Configuration

Add these DNS records at your registrar:

| Type | Name | Value |
|------|------|-------|
| A | @ | 185.199.108.153 |
| A | @ | 185.199.109.153 |
| A | @ | 185.199.110.153 |
| A | @ | 185.199.111.153 |
| CNAME | www | desilang.github.io |

---

## GitHub Actions Automation

Add `.github/workflows/docs.yml`:

```yaml
name: Deploy Docs

on:
  push:
    branches: [main, revised]
    paths: ['book/**']
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to deploy (e.g., 0.1.0)'
        required: true
      alias:
        description: 'Alias (stable, dev, etc.)'
        required: false
        default: 'stable'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - uses: actions/setup-python@v4
        with:
          python-version: '3.x'
      
      - name: Install dependencies
        run: pip install mkdocs-material mike
      
      - name: Configure Git
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
      
      # Manual deployment
      - name: Deploy manual version
        if: github.event_name == 'workflow_dispatch'
        run: |
          cd book
          mike deploy --push --update-aliases ${{ inputs.version }} ${{ inputs.alias }}
      
      # Dev deployment on push
      - name: Deploy dev docs
        if: github.event_name == 'push'
        run: |
          cd book
          mike deploy --push dev
      
      # Release deployment
      - name: Deploy release docs
        if: github.event_name == 'release'
        run: |
          cd book
          VERSION=${GITHUB_REF#refs/tags/v}
          mike deploy --push --update-aliases $VERSION stable
          mike set-default --push stable
```

---

## Updating the Theme

### Saffron Colors

Edit `book/docs/stylesheets/saffron.css`:

```css
:root {
  --md-primary-fg-color: #FF6B35;        /* Main saffron */
  --md-primary-fg-color--light: #FF8F5E;
  --md-primary-fg-color--dark: #E85A2A;
  --md-accent-fg-color: #FF9800;
}
```

### Adding Custom Templates

Put custom templates in `book/overrides/`:

```
overrides/
├── main.html           # Override base template
├── partials/
│   └── footer.html     # Custom footer
```

---

## Troubleshooting

### "mike: command not found"

```bash
pip install mike
```

### CNAME keeps disappearing

Add CNAME file to `book/docs/`:
```bash
echo "desilang.org" > book/docs/CNAME
```

### Version dropdown not showing

Ensure `extra.version.provider: mike` is in `mkdocs.yml`.

### 404 on custom domain

1. Check DNS propagation: `dig desilang.org`
2. Verify CNAME file is in gh-pages root
3. Wait 24-48 hours for DNS

---

## Quick Reference

```bash
# Preview locally
cd book && mkdocs serve

# Preview with versions
cd book && mike serve

# Deploy v0.X.Y
cd book && mike deploy --push --update-aliases 0.X.Y stable

# Set default
cd book && mike set-default --push stable

# List versions
cd book && mike list

# Delete version
cd book && mike delete --push VERSION
```

---

## Contacts

- **Domain**: desilang.org
- **Registrar**: [Your registrar]
- **Hosting**: GitHub Pages
- **Repository**: github.com/desilang/desi
