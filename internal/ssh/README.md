# SSH Module

Provides SSH client functionality for remote server management, command execution, and file transfers.

## Features

- Private key authentication
- Password authentication (fallback)
- Command execution with timeout
- Script execution
- File upload via SCP
- File download
- Connection pooling (planned)
- Proxy jump support (planned)

## Usage

### Basic Connection
```go
client, err := ssh.NewClient(ssh.Config{
    Host:       "192.168.1.100",
    Port:       22,
    User:       "root",
    PrivateKey: privateKeyPEM,
    Timeout:    30 * time.Second,
})
if err != nil {
    return err
}
defer client.Close()

if err := client.Connect(); err != nil {
    return err
}
```

### Run Command
```go
result, err := client.Run(ctx, "apt-get update")
if err != nil {
    return err
}

fmt.Println("Output:", result.Stdout)
fmt.Println("Exit code:", result.ExitCode)
```

### Run Script
```go
script := `
#!/bin/bash
set -e
apt-get update
apt-get install -y nginx
systemctl enable nginx
`

result, err := client.RunScript(ctx, script)
```

### Upload File
```go
content := []byte("server { listen 80; }")
err := client.Upload(ctx, content, "/etc/nginx/sites-available/mysite", 0644)
```

### Download File
```go
content, err := client.Download(ctx, "/var/log/nginx/access.log")
```

### Check File/Directory Exists
```go
exists, err := client.FileExists(ctx, "/etc/nginx/nginx.conf")
exists, err := client.DirExists(ctx, "/var/www/html")
```

### Wait for Connection
```go
// Wait up to 30 retries (5 minutes) for server to be ready
err := client.WaitForConnection(ctx, 30)
```

## CommandResult

```go
type CommandResult struct {
    Stdout   string  // Standard output
    Stderr   string  // Standard error
    ExitCode int     // Process exit code
}
```

## Configuration

```go
type Config struct {
    Host           string        // Server hostname/IP
    Port           int           // SSH port (default: 22)
    User           string        // SSH user
    PrivateKey     string        // PEM-encoded private key
    PrivateKeyPath string        // Path to private key file
    Password       string        // Password (fallback)
    Timeout        time.Duration // Connection timeout
}
```

## Security Notes

- Host key verification is currently disabled (`InsecureIgnoreHostKey`)
- TODO: Implement proper host key verification for production
- Private keys should be stored encrypted in database
- Use SSH agent forwarding for additional security (planned)

## Laravel Migration Reference

| Laravel | Go |
|---------|-----|
| `spatie/ssh` | `golang.org/x/crypto/ssh` |
| `Connection` class | `ssh.Config` + `ssh.Client` |
| `RemoteProcessRunner` | Integrated into `Client.Run()` |
| SCP upload | `Client.Upload()` |

## Connection Management

For long-running operations, maintain connection:
```go
// Connect once
client.Connect()

// Run multiple commands
client.Run(ctx, "command1")
client.Run(ctx, "command2")
client.Run(ctx, "command3")

// Close when done
client.Close()
```

## Error Handling

```go
result, err := client.Run(ctx, "some-command")
if err != nil {
    // Connection or execution error
    return err
}

if result.ExitCode != 0 {
    // Command failed
    return fmt.Errorf("command failed: %s", result.Stderr)
}
```

## Planned Features

- [ ] Connection pooling for multiple concurrent operations
- [ ] SSH agent forwarding
- [ ] Proxy jump (bastion host) support
- [ ] SFTP support for directory operations
- [ ] Host key verification
- [ ] Multiplexed connections
