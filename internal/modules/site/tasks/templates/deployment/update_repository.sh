# Check if the repository exists and if the remote URL is correct, if not, delete it
if [ -f "{{ .RepositoryDirectory }}/HEAD" ]; then
    cd {{ .RepositoryDirectory }}
    CURRENT_REMOTE_URL=$(git config --get remote.origin.url || echo '');

    if [ "$CURRENT_REMOTE_URL" != '{{ .RepositoryURL }}' ]; then
        {{ if .ZeroDowntimeDeployment }}
            rm -rf {{ .RepositoryDirectory }}
            cd {{ .SitePath }}
            mkdir -p {{ .RepositoryDirectory }}
        {{ else }}
            git remote set-url origin {{ .RepositoryURL }}
        {{ end }}
    fi
fi

# Set up deployment-specific variables to avoid race conditions
DEPLOYMENT_ID="{{ .DeploymentID }}"
CREDENTIALS_FILE="/tmp/git-credentials-${DEPLOYMENT_ID}"
USE_APP_AUTH=false
USE_SSH_AUTH=false

# Cleanup function to ensure credentials are always removed. Silent by
# design — which auth path was used and how it was torn down is internal
# plumbing, not something a site owner watching the deploy log needs to
# see. Genuine failures below still print.
cleanup_credentials() {
    if [ -f "$CREDENTIALS_FILE" ]; then
        rm -f "$CREDENTIALS_FILE"
    fi
    # Reset git config to system defaults
    git config --global --unset credential.helper 2>/dev/null || true
}

# Set trap to cleanup on script exit
trap cleanup_credentials EXIT

# Try to configure app-based authentication first
{{ if and .HasAppAuth .TempToken .AuthURL }}
    # Set up HTTPS authentication with temporary installation token using deployment-specific file
    git config --global credential.helper "store --file=$CREDENTIALS_FILE"
    echo "{{ .AuthURL }}" > "$CREDENTIALS_FILE"

    # Set git configuration for this operation
    git config --global user.email "deploy@{{ .AppName }}.local"
    git config --global user.name "{{ .AppName }} Deployment"

    USE_APP_AUTH=true
{{ else }}
    # Check if site has SSH deploy keys as fallback
    {{ if .DeployKeyPrivate }}
        # Set up SSH authentication using deploy keys
        DEPLOY_KEY_PATH="{{ .SitePath }}/deploy_key"

        # Create deploy key file if it doesn't exist
        if [ ! -f "$DEPLOY_KEY_PATH" ]; then
            cat <<EOF > "$DEPLOY_KEY_PATH"
{{ .DeployKeyPrivate }}
EOF
            chmod 600 "$DEPLOY_KEY_PATH"
        fi

        # Configure git to use the deploy key
        export GIT_SSH_COMMAND="ssh -i $DEPLOY_KEY_PATH -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

        USE_SSH_AUTH=true
    {{ else }}
        echo "❌ No authentication method available"
        echo "This site requires either:"
        echo "  1. App-based authentication"
        echo "  2. SSH deploy keys (legacy method)"
        echo ""
        echo "Please configure authentication before deploying."
        exit 1
    {{ end }}
{{ end }}

cd {{ .SitePath }}

echo "📦 Cloning/updating repository..."

# Clone the repository if it doesn't exist
{{ if .ZeroDowntimeDeployment }}
    if [ ! -f "{{ .RepositoryDirectory }}/HEAD" ]; then
        if git clone --mirror {{ .RepositoryURL }} {{ .RepositoryDirectory }}; then
            echo "✅ Repository cloned (mirror mode)"
        else
            echo "❌ Failed to clone repository"
            exit 1
        fi
    fi
{{ else }}
    if [ ! -f "{{ .RepositoryDirectory }}/.git/HEAD" ]; then
        if git clone {{ .RepositoryURL }} {{ .RepositoryDirectory }}; then
            echo "✅ Repository cloned"
        else
            echo "❌ Failed to clone repository"
            exit 1
        fi
    fi
{{ end }}

cd {{ .RepositoryDirectory }}

echo "🔄 Fetching latest changes..."

{{ if .ZeroDowntimeDeployment }}
    if git fetch origin {{ .RepositoryBranch }}; then
        echo "✅ Fetched branch: {{ .RepositoryBranch }}"
    else
        echo "❌ Failed to fetch branch: {{ .RepositoryBranch }}"
        exit 1
    fi
{{ else }}
    if git fetch origin && git reset --hard origin/{{ .RepositoryBranch }}; then
        echo "✅ Updated to latest: {{ .RepositoryBranch }}"
    else
        echo "❌ Failed to update to latest: {{ .RepositoryBranch }}"
        exit 1
    fi
{{ end }}

{{ if .ZeroDowntimeDeployment }}
    # Clone the repository into the release directory
    echo "📂 Exporting code to release directory..."
    cd {{ .ReleaseDirectory }}
    if git clone -l {{ .RepositoryDirectory }} . && git checkout --force {{ .RepositoryBranch }}; then
        echo "✅ Code exported to release directory"
    else
        echo "❌ Failed to export code to release directory"
        exit 1
    fi
{{ end }}

# Cleanup is handled by the trap function
cd {{ .SitePath }}
