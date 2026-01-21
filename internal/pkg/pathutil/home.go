package pathutil

import "path/filepath"

// HomeDir returns the home directory for a user.
// Returns "/root" for root user, "/home/{user}" for others.
func HomeDir(user string) string {
	if user == "root" {
		return "/root"
	}

	return "/home/" + user
}

// SSHDir returns the .ssh directory for a user.
func SSHDir(user string) string {
	return filepath.Join(HomeDir(user), ".ssh")
}

// AuthorizedKeysPath returns the authorized_keys file path for a user.
func AuthorizedKeysPath(user string) string {
	return filepath.Join(SSHDir(user), "authorized_keys")
}

// JoinHome joins path segments under a user's home directory.
// Example: JoinHome("deploy", ".config", "composer") -> "/home/deploy/.config/composer"
func JoinHome(user string, parts ...string) string {
	args := make([]string, 0, len(parts)+1)
	args = append(args, HomeDir(user))
	args = append(args, parts...)

	return filepath.Join(args...)
}
