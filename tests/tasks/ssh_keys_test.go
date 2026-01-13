package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestGenerateRsaKeyPair_DefaultBits(t *testing.T) {
	task := tasks.GenerateRsaKeyPair("/home/launcher/.ssh/id_rsa", 0)

	testutil.AssertTask(t, task).
		HasName("Generate RSA Key Pair").
		ScriptContains("ssh-keygen -t rsa").
		ScriptContains("-b 4096").
		ScriptContains("-f /home/launcher/.ssh/id_rsa").
		ScriptContains(`-N ""`).
		ScriptMatches("tasks/ssh_generate_rsa_default")
}

func TestGenerateRsaKeyPair_CustomBits(t *testing.T) {
	task := tasks.GenerateRsaKeyPair("/tmp/test_key", 2048)

	testutil.AssertTask(t, task).
		HasName("Generate RSA Key Pair").
		ScriptContains("-b 2048")
}

func TestGenerateEd25519KeyPair(t *testing.T) {
	task := tasks.GenerateEd25519KeyPair("/home/launcher/.ssh/id_ed25519")

	testutil.AssertTask(t, task).
		HasName("Generate Ed25519 Key Pair").
		ScriptContains("ssh-keygen -t ed25519").
		ScriptContains("-f /home/launcher/.ssh/id_ed25519").
		ScriptMatches("tasks/ssh_generate_ed25519")
}

func TestGeneratePublicKey(t *testing.T) {
	task := tasks.GeneratePublicKey("/home/launcher/.ssh/id_rsa")

	testutil.AssertTask(t, task).
		HasName("Generate Public Key").
		ScriptContains("ssh-keygen -y").
		ScriptContains("-f /home/launcher/.ssh/id_rsa").
		ScriptMatches("tasks/ssh_generate_public")
}

func TestAuthorizePublicKey_RegularUser(t *testing.T) {
	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... user@example.com"

	task := tasks.AuthorizePublicKey(publicKey, "launcher")

	testutil.AssertTask(t, task).
		HasName("Authorize Public Key").
		ScriptContains("mkdir -p /home/launcher/.ssh").
		ScriptContains("chmod 700").
		ScriptContains("authorized_keys").
		ScriptContains("chmod 600").
		ScriptContains("chown -R launcher:launcher").
		ScriptMatches("tasks/ssh_authorize_user")
}

func TestAuthorizePublicKey_RootUser(t *testing.T) {
	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... root@example.com"

	task := tasks.AuthorizePublicKey(publicKey, "root")

	testutil.AssertTask(t, task).
		HasName("Authorize Public Key").
		ScriptContains("/root/.ssh").
		ScriptContains("chown -R root:root").
		ScriptMatches("tasks/ssh_authorize_root")
}

func TestDeauthorizePublicKey_RegularUser(t *testing.T) {
	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."

	task := tasks.DeauthorizePublicKey(publicKey, "launcher")

	testutil.AssertTask(t, task).
		HasName("Deauthorize Public Key").
		ScriptContains("sed -i").
		ScriptContains("/home/launcher/.ssh/authorized_keys").
		ScriptMatches("tasks/ssh_deauthorize_user")
}

func TestDeauthorizePublicKey_RootUser(t *testing.T) {
	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."

	task := tasks.DeauthorizePublicKey(publicKey, "root")

	testutil.AssertTask(t, task).
		HasName("Deauthorize Public Key").
		ScriptContains("/root/.ssh/authorized_keys").
		ScriptMatches("tasks/ssh_deauthorize_root")
}

func TestGetAuthorizedKeys_RegularUser(t *testing.T) {
	task := tasks.GetAuthorizedKeys("launcher")

	testutil.AssertTask(t, task).
		HasName("Get Authorized Keys").
		ScriptContains("cat /home/launcher/.ssh/authorized_keys").
		ScriptMatches("tasks/ssh_get_keys_user")
}

func TestGetAuthorizedKeys_RootUser(t *testing.T) {
	task := tasks.GetAuthorizedKeys("root")

	testutil.AssertTask(t, task).
		HasName("Get Authorized Keys").
		ScriptContains("cat /root/.ssh/authorized_keys").
		ScriptMatches("tasks/ssh_get_keys_root")
}
