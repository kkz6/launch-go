#!/bin/bash
# Setup script for git hooks and linting tools
# Run this after cloning the repository

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Launch Go - Development Setup${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}Go version: $GO_VERSION${NC}"
echo ""

# Install linting tools
echo -e "${YELLOW}Installing linting tools...${NC}"
echo ""

echo -e "  Installing goimports..."
go install golang.org/x/tools/cmd/goimports@latest
echo -e "${GREEN}  goimports installed${NC}"

echo -e "  Installing revive..."
go install github.com/mgechev/revive@latest
echo -e "${GREEN}  revive installed${NC}"

echo -e "  Installing staticcheck..."
go install honnef.co/go/tools/cmd/staticcheck@latest
echo -e "${GREEN}  staticcheck installed${NC}"

echo -e "  Installing govulncheck..."
go install golang.org/x/vuln/cmd/govulncheck@latest
echo -e "${GREEN}  govulncheck installed${NC}"

echo ""

# Configure git to use our hooks directory
echo -e "${YELLOW}Configuring git hooks...${NC}"
git config core.hooksPath .githooks
echo -e "${GREEN}Git hooks configured to use .githooks directory${NC}"
echo ""

# Make hooks executable
echo -e "${YELLOW}Making hooks executable...${NC}"
chmod +x .githooks/*
echo -e "${GREEN}Hooks are now executable${NC}"
echo ""

# Verify installation
echo -e "${YELLOW}Verifying installation...${NC}"
echo ""

GOBIN="$(go env GOPATH)/bin"
TOOLS=("goimports" "revive" "staticcheck" "govulncheck")
ALL_INSTALLED=true

for tool in "${TOOLS[@]}"; do
    TOOL_PATH="$GOBIN/$tool"
    if [ -x "$TOOL_PATH" ]; then
        VERSION=$("$TOOL_PATH" -version 2>&1 | head -n1 || echo "installed")
        echo -e "  ${GREEN}$tool: $VERSION${NC}"
    else
        echo -e "  ${RED}$tool: NOT FOUND at $TOOL_PATH${NC}"
        ALL_INSTALLED=false
    fi
done

echo ""
echo -e "${YELLOW}Tools installed at: $GOBIN${NC}"

echo ""

if [ "$ALL_INSTALLED" = true ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Setup complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Available commands:"
    echo "  make lint          - Run all linters"
    echo "  make lint-revive   - Run revive only"
    echo "  make lint-static   - Run staticcheck only"
    echo "  make lint-fix      - Auto-fix with goimports"
    echo "  make fmt           - Format code with gofmt"
    echo ""
    echo "Git hooks will now run automatically on commit."
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  Setup incomplete!${NC}"
    echo -e "${RED}========================================${NC}"
    echo ""
    echo "Some tools failed to install. Please check your Go installation"
    echo "and ensure \$GOPATH/bin is in your PATH."
    exit 1
fi
