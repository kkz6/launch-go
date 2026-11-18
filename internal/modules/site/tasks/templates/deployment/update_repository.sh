# Credentials are delivered separately in a private per-execution directory.
# Empty credential.helper resets inherited helpers, preventing ambient credentials
# from granting access to an unselected private dependency.
export GIT_TERMINAL_PROMPT=0
export GIT_ASKPASS=false
unset GIT_CONFIG_PARAMETERS
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=5
export GIT_CONFIG_KEY_0=credential.helper
export GIT_CONFIG_VALUE_0=""
export GIT_CONFIG_KEY_1=credential.useHttpPath
export GIT_CONFIG_VALUE_1=true
export GIT_CONFIG_KEY_2=credential.helper
export GIT_CONFIG_VALUE_2=""
export GIT_CONFIG_KEY_3=http.followRedirects
export GIT_CONFIG_VALUE_3=false
export GIT_CONFIG_KEY_4=http.emptyAuth
export GIT_CONFIG_VALUE_4=true
{{ if .HasAppAuth }}
    : "${LAUNCH_SECRET_DIR:?Missing deployment credentials}"
    test -r "$LAUNCH_SECRET_DIR/git-credentials" || exit 1
    export GIT_CONFIG_VALUE_2="store --file=$LAUNCH_SECRET_DIR/git-credentials"
    export GIT_ALLOW_PROTOCOL=https
    export GIT_SSH_COMMAND=false
    # GitHub SSH submodule URLs use the same explicit HTTPS repository grants.
    export GIT_CONFIG_COUNT=7
    export GIT_CONFIG_KEY_5=url.https://github.com/.insteadOf
    export GIT_CONFIG_VALUE_5=git@github.com:
    export GIT_CONFIG_KEY_6=url.https://github.com/.insteadOf
    export GIT_CONFIG_VALUE_6=ssh://git@github.com/
{{ else if .HasDeployKey }}
    : "${LAUNCH_SECRET_DIR:?Missing deployment credentials}"
    export GIT_SSH_COMMAND="ssh -i $LAUNCH_SECRET_DIR/git-key -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
{{ else }}
    echo "No repository authentication is available. Reconnect and select this repository."
    exit 1
{{ end }}

# Reused checkouts must fetch the currently selected canonical repository.
# Remove legacy local authentication and rewrites before consulting origin;
# URL-specific headers otherwise bypass the scoped credential helper.
if [ -f "{{ .RepositoryDirectory }}/HEAD" ] || [ -f "{{ .RepositoryDirectory }}/.git/HEAD" ]; then
    cd "{{ .RepositoryDirectory }}"
    {{ if .HasAppAuth }}
    while IFS= read -r key; do
        git config --local --unset-all "$key" || true
    done < <(git config --local --name-only --get-regexp '^(credential\.|http\.|url\.|include\.|includeif\.|core\.(sshcommand|askpass)$)' || true)
    {{ end }}
    {{ if .ZeroDowntimeDeployment }}
    CURRENT_REMOTE_URL=$(git config --local --get remote.origin.url || echo '')
    if [ "$CURRENT_REMOTE_URL" != '{{ .RepositoryURL }}' ]; then
        cd "{{ .SitePath }}"
        rm -rf "{{ .RepositoryDirectory }}"
    else
        git config --local --replace-all remote.origin.url '{{ .RepositoryURL }}'
    fi
    {{ else }}
    git config --local --replace-all remote.origin.url '{{ .RepositoryURL }}'
    {{ end }}
fi

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
    if GIT_ALLOW_PROTOCOL=file git clone -l {{ .RepositoryDirectory }} . && git checkout --force {{ .RepositoryBranch }}; then
        echo "✅ Code exported to release directory"
    else
        echo "❌ Failed to export code to release directory"
        exit 1
    fi
{{ end }}

# Cleanup is handled by the trap function
cd {{ .SitePath }}
