package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/pathutil"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for SSH key operations
const (
	GenerateRsaKeyPairTaskType     = "server:generate_rsa_keypair"
	GenerateEd25519KeyPairTaskType = "server:generate_ed25519_keypair"
	GeneratePublicKeyTaskType      = "server:generate_public_key"
	AuthorizePublicKeyTaskType     = "server:authorize_public_key"
	DeauthorizePublicKeyTaskType   = "server:deauthorize_public_key"
	GetAuthorizedKeysTaskType      = "server:get_authorized_keys"
	UpdateAuthorizedKeysTaskType   = "server:update_authorized_keys"
)

// GenerateRsaKeyPair creates a task to generate RSA SSH key pair
func GenerateRsaKeyPair(keyPath string, bits int) *taskrunner.BaseTask {
	if bits == 0 {
		bits = 4096
	}

	script := fmt.Sprintf(`ssh-keygen -t rsa -b %d -f %s -N "" -q`, bits, keyPath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Generate RSA Key Pair"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GenerateEd25519KeyPair creates a task to generate Ed25519 SSH key pair
func GenerateEd25519KeyPair(keyPath string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`ssh-keygen -t ed25519 -f %s -N "" -q`, keyPath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Generate Ed25519 Key Pair"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GeneratePublicKey creates a task to derive public key from private key
func GeneratePublicKey(privateKeyPath string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`ssh-keygen -y -f %s`, privateKeyPath)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Generate Public Key"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// AuthorizePublicKey creates a task to add a public key to authorized_keys
func AuthorizePublicKey(publicKey string, user string) *taskrunner.BaseTask {
	sshDir := pathutil.SSHDir(user)
	authorizedKeys := pathutil.AuthorizedKeysPath(user)

	script := fmt.Sprintf(`mkdir -p %s
chmod 700 %s
echo '%s' >> %s
chmod 600 %s
chown -R %s:%s %s`, sshDir, sshDir, publicKey, authorizedKeys, authorizedKeys, user, user, sshDir)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Authorize Public Key"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeauthorizePublicKey creates a task to remove a public key from authorized_keys
func DeauthorizePublicKey(publicKey string, user string) *taskrunner.BaseTask {
	authorizedKeys := pathutil.AuthorizedKeysPath(user)

	// Escape special characters in the public key for sed
	script := fmt.Sprintf(`sed -i '\|%s|d' %s`, publicKey, authorizedKeys)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deauthorize Public Key"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GetAuthorizedKeys creates a task to get all authorized keys for a user
func GetAuthorizedKeys(user string) *taskrunner.BaseTask {
	authorizedKeys := pathutil.AuthorizedKeysPath(user)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get Authorized Keys"),
		taskrunner.WithScript(fmt.Sprintf("cat %s 2>/dev/null || echo ''", authorizedKeys)),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// UpdateAuthorizedKeys creates a task to replace the authorized_keys file with new content
func UpdateAuthorizedKeys(user string, publicKey string) *taskrunner.BaseTask {
	sshDir := pathutil.SSHDir(user)
	authorizedKeys := pathutil.AuthorizedKeysPath(user)

	script := fmt.Sprintf(`mkdir -p %s
chmod 700 %s
cat > %s << 'LAUNCH_EOF'
%s
LAUNCH_EOF
chmod 600 %s
chown -R %s:%s %s`, sshDir, sshDir, authorizedKeys, publicKey, authorizedKeys, user, user, sshDir)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Authorized Keys"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
