package tasks

import (
	"encoding/base64"
	"encoding/json"
)

// Credentials are delivered separately from persisted task scripts. Every job,
// including public pulls, receives its own Docker config and cleanup lifetime.
func deploymentSecretFiles(gitCredentials string, logins []ComposeRegistryLogin) map[string]string {
	auths := map[string]map[string]string{}
	for _, login := range logins {
		if login.Username == "" || login.Password == "" {
			continue
		}
		host := login.RegistryURL
		if host == "" {
			host = "https://index.docker.io/v1/"
		}
		if login.Username == "oauth2" && host == "ghcr.io" {
			auths[host] = map[string]string{"registrytoken": login.Password}
		} else {
			auths[host] = map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(login.Username + ":" + login.Password))}
		}
	}
	data, _ := json.Marshal(map[string]any{"auths": auths})
	files := map[string]string{"docker-config": string(data)}
	if gitCredentials != "" {
		files["git-credentials"] = gitCredentials
	}
	return files
}

const isolatedCredentialsScript = `
set +x
umask 077
export DOCKER_CONFIG="${LAUNCH_SECRET_DIR}/docker"
mkdir -p "${DOCKER_CONFIG}"
cp "${LAUNCH_SECRET_DIR}/docker-config" "${DOCKER_CONFIG}/config.json"
export GIT_TERMINAL_PROMPT=0
export GIT_ASKPASS=false
export GIT_ALLOW_PROTOCOL=https
export GIT_SSH_COMMAND=false
unset GIT_CONFIG_PARAMETERS
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=7
export GIT_CONFIG_KEY_0=credential.helper
export GIT_CONFIG_VALUE_0=""
export GIT_CONFIG_KEY_1=credential.useHttpPath
export GIT_CONFIG_VALUE_1=true
export GIT_CONFIG_KEY_2=http.followRedirects
export GIT_CONFIG_VALUE_2=false
export GIT_CONFIG_KEY_3=url.https://github.com/.insteadOf
export GIT_CONFIG_VALUE_3=git@github.com:
export GIT_CONFIG_KEY_4=url.https://github.com/.insteadOf
export GIT_CONFIG_VALUE_4=ssh://git@github.com/
export GIT_CONFIG_KEY_5=credential.helper
export GIT_CONFIG_VALUE_5="store --file=${LAUNCH_SECRET_DIR}/git-credentials"
export GIT_CONFIG_KEY_6=http.emptyAuth
export GIT_CONFIG_VALUE_6=true
`
