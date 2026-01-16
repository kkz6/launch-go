package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
	homeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		homeDir = "/root"
	}

	script := fmt.Sprintf(`mkdir -p %s/.ssh
chmod 700 %s/.ssh
echo '%s' >> %s/.ssh/authorized_keys
chmod 600 %s/.ssh/authorized_keys
chown -R %s:%s %s/.ssh`, homeDir, homeDir, publicKey, homeDir, homeDir, user, user, homeDir)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Authorize Public Key"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeauthorizePublicKey creates a task to remove a public key from authorized_keys
func DeauthorizePublicKey(publicKey string, user string) *taskrunner.BaseTask {
	homeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		homeDir = "/root"
	}

	// Escape special characters in the public key for sed
	script := fmt.Sprintf(`sed -i '\|%s|d' %s/.ssh/authorized_keys`, publicKey, homeDir)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Deauthorize Public Key"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// GetAuthorizedKeys creates a task to get all authorized keys for a user
func GetAuthorizedKeys(user string) *taskrunner.BaseTask {
	homeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		homeDir = "/root"
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get Authorized Keys"),
		taskrunner.WithScript(fmt.Sprintf("cat %s/.ssh/authorized_keys 2>/dev/null || echo ''", homeDir)),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// UpdateAuthorizedKeys creates a task to replace the authorized_keys file with new content
func UpdateAuthorizedKeys(user string, publicKey string) *taskrunner.BaseTask {
	homeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		homeDir = "/root"
	}

	script := fmt.Sprintf(`mkdir -p %s/.ssh
chmod 700 %s/.ssh
cat > %s/.ssh/authorized_keys << 'LAUNCH_EOF'
%s
LAUNCH_EOF
chmod 600 %s/.ssh/authorized_keys
chown -R %s:%s %s/.ssh`, homeDir, homeDir, homeDir, publicKey, homeDir, user, user, homeDir)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Authorized Keys"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
