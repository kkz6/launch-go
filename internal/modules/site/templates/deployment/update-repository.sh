{{/* Go Template: deployment/update-repository.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/update-repository.blade.php */}}
# Check if the repository exists and if the remote URL is correct, if not, delete it
if [ -f "{{ .RepositoryDirectory }}/HEAD" ]; then
    cd {{ .RepositoryDirectory }}
    CURRENT_REMOTE_URL=$(git config --get remote.origin.url || echo '');

    if [ "$CURRENT_REMOTE_URL" != '{{ .Site.RepositoryURL }}' ]; then
        {{ if .Site.ZeroDowntimeDeployment }}
        rm -rf {{ .RepositoryDirectory }}
        cd {{ .Site.Path }}
        mkdir -p {{ .RepositoryDirectory }}
        {{ else }}
        git remote set-url origin {{ .Site.RepositoryURL }}
        {{ end }}
    fi
fi

# Set up deployment-specific variables to avoid race conditions
DEPLOYMENT_ID="{{ .Deployment.ID }}"
CREDENTIALS_FILE="/tmp/git-credentials-${DEPLOYMENT_ID}"
USE_APP_AUTH=false
USE_SSH_AUTH=false

# Cleanup function to ensure credentials are always removed
cleanup_credentials() {
    if [ -f "$CREDENTIALS_FILE" ]; then
        rm -f "$CREDENTIALS_FILE"
        echo "Cleaned up deployment credentials"
    fi
    # Reset git config to system defaults
    git config --global --unset credential.helper 2>/dev/null || true
}

# Set trap to cleanup on script exit
trap cleanup_credentials EXIT

# Try to configure app-based authentication first
{{ if and .HasAppAuth .TempToken .AuthURL }}
echo "Setting up app-based authentication for deployment ${DEPLOYMENT_ID}..."

# Set up HTTPS authentication with temporary installation token using deployment-specific file
git config --global credential.helper "store --file=$CREDENTIALS_FILE"
echo "{{ .AuthURL }}" > "$CREDENTIALS_FILE"

# Set git configuration for this operation
git config --global user.email "deploy@{{ .AppName }}.local"
git config --global user.name "{{ .AppName }} Deployment"

USE_APP_AUTH=true
echo "App-based authentication configured successfully"
{{ else }}
# Check if site has SSH deploy keys as fallback
{{ if .Site.DeployKeyPrivate }}
echo "App-based authentication not available, falling back to SSH deployment keys..."

# Set up SSH authentication using deploy keys
DEPLOY_KEY_PATH="{{ .Site.Path }}/deploy_key"

# Create deploy key file if it doesn't exist
if [ ! -f "$DEPLOY_KEY_PATH" ]; then
    cat <<EOF > "$DEPLOY_KEY_PATH"
{{ .Site.DeployKeyPrivate }}
EOF
    chmod 600 "$DEPLOY_KEY_PATH"
fi

# Configure git to use the deploy key
export GIT_SSH_COMMAND="ssh -i $DEPLOY_KEY_PATH -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

USE_SSH_AUTH=true
echo "SSH authentication configured successfully"
{{ else }}
echo "No authentication method available"
echo "This site requires either:"
echo "  1. App-based authentication (run: php artisan sites:migrate-to-app-deployment)"
echo "  2. SSH deploy keys (legacy method)"
echo ""
echo "Please configure authentication before deploying."
exit 1
{{ end }}
{{ end }}

cd {{ .Site.Path }}

echo "Cloning/updating repository..."

# Clone the repository if it doesn't exist
{{ if .Site.ZeroDowntimeDeployment }}
if [ ! -f "{{ .RepositoryDirectory }}/HEAD" ]; then
    if git clone --mirror {{ .Site.RepositoryURL }} {{ .RepositoryDirectory }}; then
        echo "Repository cloned (mirror mode)"
    else
        echo "Failed to clone repository"
        exit 1
    fi
fi
{{ else }}
if [ ! -f "{{ .RepositoryDirectory }}/.git/HEAD" ]; then
    if git clone {{ .Site.RepositoryURL }} {{ .RepositoryDirectory }}; then
        echo "Repository cloned"
    else
        echo "Failed to clone repository"
        exit 1
    fi
fi
{{ end }}

cd {{ .RepositoryDirectory }}

echo "Fetching latest changes..."

{{ if .Site.ZeroDowntimeDeployment }}
if git fetch origin {{ .Site.RepositoryBranch }}; then
    echo "Fetched branch: {{ .Site.RepositoryBranch }}"
else
    echo "Failed to fetch branch: {{ .Site.RepositoryBranch }}"
    exit 1
fi
{{ else }}
if git fetch origin && git reset --hard origin/{{ .Site.RepositoryBranch }}; then
    echo "Updated to latest: {{ .Site.RepositoryBranch }}"
else
    echo "Failed to update to latest: {{ .Site.RepositoryBranch }}"
    exit 1
fi
{{ end }}

# Success message with authentication method used
if [ "$USE_APP_AUTH" = true ]; then
    echo "Repository updated successfully with app-based authentication"
elif [ "$USE_SSH_AUTH" = true ]; then
    echo "Repository updated successfully with SSH deploy keys"
fi

{{ if .Site.ZeroDowntimeDeployment }}
# Clone the repository into the release directory
echo "Exporting code to release directory..."
cd {{ .ReleaseDirectory }}
if git clone -l {{ .RepositoryDirectory }} . && git checkout --force {{ .Site.RepositoryBranch }}; then
    echo "Code exported to release directory"
else
    echo "Failed to export code to release directory"
    exit 1
fi
{{ end }}

# Cleanup is handled by the trap function
cd {{ .Site.Path }}
