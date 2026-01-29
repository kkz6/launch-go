// Package paths provides a unified path builder for server paths.
// It offers a fluent API for constructing common server paths like home directories,
// SSH paths, site directories, release directories, and system configuration paths.
package paths

import (
	"fmt"
	"path/filepath"
	"time"
)

// User represents a server user for path construction.
type User struct {
	Name string
}

// ForUser creates a User path builder for the given username.
func ForUser(username string) User {
	return User{Name: username}
}

// Home returns the home directory for the user.
// Returns "/root" for root user, "/home/{user}" for others.
func (u User) Home() string {
	if u.Name == "root" {
		return "/root"
	}

	return fmt.Sprintf("/home/%s", u.Name)
}

// SSH returns the .ssh directory path.
func (u User) SSH() string {
	return filepath.Join(u.Home(), ".ssh")
}

// AuthorizedKeys returns the authorized_keys file path.
func (u User) AuthorizedKeys() string {
	return filepath.Join(u.SSH(), "authorized_keys")
}

// KnownHosts returns the known_hosts file path.
func (u User) KnownHosts() string {
	return filepath.Join(u.SSH(), "known_hosts")
}

// PrivateKey returns the path to a private key file.
func (u User) PrivateKey(name string) string {
	return filepath.Join(u.SSH(), name)
}

// PublicKey returns the path to a public key file.
func (u User) PublicKey(name string) string {
	return filepath.Join(u.SSH(), name+".pub")
}

// Config returns the SSH config file path.
func (u User) Config() string {
	return filepath.Join(u.SSH(), "config")
}

// Join joins path segments under the user's home directory.
func (u User) Join(parts ...string) string {
	args := make([]string, 0, len(parts)+1)
	args = append(args, u.Home())
	args = append(args, parts...)

	return filepath.Join(args...)
}

// TaskDir returns the .launch directory for the user.
func (u User) TaskDir() string {
	return u.Join(".launch")
}

// Site represents a site for path construction.
type Site struct {
	User User
	Name string
}

// ForSite creates a Site path builder for the given username and site name.
func ForSite(username, siteName string) Site {
	return Site{User: ForUser(username), Name: siteName}
}

// Root returns the site root directory.
func (s Site) Root() string {
	return filepath.Join(s.User.Home(), s.Name)
}

// Current returns the current symlink path (used in zero-downtime deployments).
func (s Site) Current() string {
	return filepath.Join(s.Root(), "current")
}

// Repository returns the repository directory (used in standard deployments).
func (s Site) Repository() string {
	return filepath.Join(s.Root(), "repository")
}

// Releases returns the releases directory.
func (s Site) Releases() string {
	return filepath.Join(s.Root(), "releases")
}

// Release returns a specific release directory for the given timestamp.
func (s Site) Release(timestamp time.Time) string {
	return filepath.Join(s.Releases(), timestamp.Format("20060102150405"))
}

// ReleaseNamed returns a specific release directory by name.
func (s Site) ReleaseNamed(name string) string {
	return filepath.Join(s.Releases(), name)
}

// Shared returns the shared directory.
func (s Site) Shared() string {
	return filepath.Join(s.Root(), "shared")
}

// Storage returns the shared storage directory.
func (s Site) Storage() string {
	return filepath.Join(s.Shared(), "storage")
}

// Env returns the shared .env file path.
func (s Site) Env() string {
	return filepath.Join(s.Shared(), ".env")
}

// Logs returns the logs directory.
func (s Site) Logs() string {
	return filepath.Join(s.Root(), "logs")
}

// Join joins path segments under the site root directory.
func (s Site) Join(parts ...string) string {
	args := make([]string, 0, len(parts)+1)
	args = append(args, s.Root())
	args = append(args, parts...)

	return filepath.Join(args...)
}

// Application returns the current application directory based on deployment type.
// For zero-downtime deployments, returns the current symlink path.
// For standard deployments, returns the repository path.
func (s Site) Application(zeroDowntime bool) string {
	if zeroDowntime {
		return s.Current()
	}

	return s.Repository()
}

// WebDirectory returns the web directory path for the given web folder.
func (s Site) WebDirectory(zeroDowntime bool, webFolder string) string {
	return filepath.Join(s.Application(zeroDowntime), webFolder)
}

// TaskPaths holds all file paths for a task execution.
type TaskPaths struct {
	Script   string // task-{id}.sh
	Output   string // task-{id}.log
	ExitCode string // task-{id}.exit
}

// GetTaskPaths returns all file paths for a task given the base directory and task ID.
func GetTaskPaths(baseDir, taskID string) TaskPaths {
	return TaskPaths{
		Script:   filepath.Join(baseDir, fmt.Sprintf("task-%s.sh", taskID)),
		Output:   filepath.Join(baseDir, fmt.Sprintf("task-%s.log", taskID)),
		ExitCode: filepath.Join(baseDir, fmt.Sprintf("task-%s.exit", taskID)),
	}
}

// GetTaskDir returns the .launch directory for a user.
func GetTaskDir(user string) string {
	return ForUser(user).TaskDir()
}

// HomeDir returns the home directory for a user.
// Returns "/root" for root user, "/home/{user}" for others.
// This is a convenience function for simple cases where the fluent API isn't needed.
func HomeDir(user string) string {
	return ForUser(user).Home()
}

// SSHDir returns the .ssh directory for a user.
// This is a convenience function for simple cases where the fluent API isn't needed.
func SSHDir(user string) string {
	return ForUser(user).SSH()
}

// AuthorizedKeysPath returns the authorized_keys file path for a user.
// This is a convenience function for simple cases where the fluent API isn't needed.
func AuthorizedKeysPath(user string) string {
	return ForUser(user).AuthorizedKeys()
}

// JoinHome joins path segments under a user's home directory.
// Example: JoinHome("deploy", ".config", "composer") -> "/home/deploy/.config/composer"
// This is a convenience function for simple cases where the fluent API isn't needed.
func JoinHome(user string, parts ...string) string {
	return ForUser(user).Join(parts...)
}
