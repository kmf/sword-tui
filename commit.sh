#!/bin/bash

# Semantic versioning commit helper
# Usage: ./commit.sh <type> <message>
# Types: feat (minor++), fix (patch++), breaking (major++)

set -e

TYPE=$1
MESSAGE=$2

if [ -z "$TYPE" ] || [ -z "$MESSAGE" ]; then
    echo "Usage: ./commit.sh <type> <message>"
    echo "Types:"
    echo "  feat     - New feature (increments minor version)"
    echo "  fix      - Bug fix (increments patch version)"
    echo "  breaking - Breaking change (increments major version)"
    echo "  docs     - Documentation only (no version increment)"
    echo "  refactor - Code refactoring (no version increment)"
    echo "  chore    - Maintenance (no version increment)"
    exit 1
fi

# Read current version
CURRENT_VERSION=$(grep 'const Version' internal/version/version.go | cut -d'"' -f2 | sed 's/v//')
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Calculate new version based on commit type
case "$TYPE" in
    feat)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    fix)
        PATCH=$((PATCH + 1))
        ;;
    breaking)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    docs|refactor|chore|perf|test|style)
        # No version increment for these types
        ;;
    *)
        echo "Unknown type: $TYPE"
        echo "Valid types: feat, fix, breaking, docs, refactor, chore"
        exit 1
        ;;
esac

NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"

# Update version file if version changed
if [ "$TYPE" = "feat" ] || [ "$TYPE" = "fix" ] || [ "$TYPE" = "breaking" ]; then
    sed -i "s/const Version = \"v[0-9.]*\"/const Version = \"$NEW_VERSION\"/" internal/version/version.go
    echo "Version updated: v$CURRENT_VERSION -> $NEW_VERSION"
    git add internal/version/version.go
fi

# Create commit message
if [ "$TYPE" = "breaking" ]; then
    COMMIT_MSG="feat: $MESSAGE

BREAKING CHANGE: $MESSAGE

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
else
    COMMIT_MSG="$TYPE: $MESSAGE

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
fi

# Stage all changes and commit
git add -A
git commit -m "$COMMIT_MSG"

echo "Committed: $TYPE - $MESSAGE"
if [ "$TYPE" = "feat" ] || [ "$TYPE" = "fix" ] || [ "$TYPE" = "breaking" ]; then
    echo "New version: $NEW_VERSION"
fi
