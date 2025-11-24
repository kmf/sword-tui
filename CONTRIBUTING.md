# Contributing to sword-tui

## Commit Guidelines

This project uses [Conventional Commits](https://www.conventionalcommits.org/) and [Semantic Versioning](https://semver.org/).

### Using the Commit Helper

We provide a helper script that automatically handles versioning:

```bash
./commit.sh <type> <message>
```

**Commit Types:**
- `feat` - New feature (increments minor version: 1.0.0 → 1.1.0)
- `fix` - Bug fix (increments patch version: 1.0.0 → 1.0.1)
- `breaking` - Breaking change (increments major version: 1.0.0 → 2.0.0)
- `docs` - Documentation only (no version increment)
- `refactor` - Code refactoring (no version increment)
- `chore` - Maintenance tasks (no version increment)
- `perf` - Performance improvements (no version increment)
- `test` - Adding tests (no version increment)
- `style` - Code style changes (no version increment)

**Examples:**

```bash
# Add a new feature - version 1.0.0 → 1.1.0
./commit.sh feat "add dark mode toggle"

# Fix a bug - version 1.1.0 → 1.1.1
./commit.sh fix "resolve memory leak in cache manager"

# Breaking change - version 1.1.1 → 2.0.0
./commit.sh breaking "change API endpoint structure"

# Documentation - no version change
./commit.sh docs "update README with new features"
```

### Manual Commits

If you commit manually, follow this format:

```
<type>: <description>

[optional body]

[optional footer]

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Important:** When committing manually with `feat`, `fix`, or breaking changes, remember to update the version in `internal/version/version.go` according to semantic versioning rules.

## Build System

Use the build script to create versioned builds:

```bash
./build.sh
```

This automatically:
- Increments the build number
- Injects the build number via ldflags
- Compiles the binary

The version is displayed in the app footer as: `v1.1.0 (build 18)`
