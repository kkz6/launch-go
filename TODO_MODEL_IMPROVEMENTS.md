# Model Improvements TODO

## Completed

### High Priority

- [x] **Use JSONStringSlice for JSON array fields**
  - [x] `site.go` - Replace `Aliases *string` with `JSONStringSlice`
  - [x] `site.go` - Replace `SharedDirectories string` with `JSONStringSlice`
  - [x] `site.go` - Replace `WriteableDirectories string` with `JSONStringSlice`
  - [x] `site.go` - Replace `SharedFiles string` with `JSONStringSlice`
  - [x] `site.go` - Replace `Features *string` with `JSONStringSlice`
  - [x] `certificate.go` - Replace `Domains *string` with `JSONStringSlice`
  - [x] `daemon.go` - Replace `Info *string` with `JSONMap`
  - [x] Removed all Get/Set boilerplate methods after migration

- [x] **Encrypt sensitive fields**
  - [x] `certificate.go` - Change `PrivateKey *string` to `EncryptedString`
  - [x] `site.go` - Change `DeployKeyPrivate *string` to `EncryptedString`

### Medium Priority

- [x] **Add SiteScopedModel** - Added to `base.go`

- [x] **Centralize token generation**
  - [x] Created `internal/pkg/utils/token.go` with unified functions
  - [x] Updated `server.go` to use new utility
  - [x] Updated `site/models/helpers.go` to delegate to utility
  - [x] Updated `backup.go` to use new utility

- [x] **Extract log path helper**
  - [x] Added `GetUserHomeDir()` and `GetWorkingDir()` to `base.go`
  - [x] Updated `cron.go` to use helper
  - [x] Updated `daemon.go` to use helper

### Low Priority

- [x] **Standardize TableName() receivers**
  - [x] Changed all pointer receivers to value receivers for TableName()

- [x] **Remove redundant wrapper methods**
  - [x] `backup.go` - Removed `IsPendingInstallation()` and `IsInstallationFailed()`

## Summary of Changes

### New Files Created
- `internal/pkg/utils/token.go` - Centralized token generation utilities

### Models Updated (47 files analyzed, 30+ files modified)

#### Server Module
- `server.go` - Updated to use utils.GenerateHexToken, value receiver for TableName
- `cron.go` - Simplified GetLogPath using basemodels.GetWorkingDir
- `daemon.go` - Updated Info to JSONMap, simplified log path methods
- `firewall_rule.go` - Value receiver for TableName
- `metric.go` - Value receivers for TableName (5 models)
- `ssh_key.go` - Value receivers for TableName (2 models)
- `task.go` - Value receiver for TableName
- `server_provider.go` - Value receiver for TableName
- `installed_service.go` - Value receiver for TableName

#### Site Module
- `site.go` - JSONStringSlice for arrays, EncryptedString for DeployKeyPrivate, value receiver
- `certificate.go` - JSONStringSlice for Domains, EncryptedString for PrivateKey, value receiver
- `deployment.go` - Value receiver for TableName
- `queue.go` - Value receiver for TableName
- `redirect.go` - Value receiver for TableName
- `release.go` - Value receiver for TableName
- `command.go` - Value receiver for TableName
- `helpers.go` - Delegated to utils package

#### Auth Module
- `user.go` - Value receiver for TableName
- `team.go` - Value receiver for TableName
- `team_member.go` - Value receiver for TableName
- `team_invitation.go` - Value receiver for TableName
- `passkey.go` - Value receiver for TableName
- `password_reset_token.go` - Value receiver for TableName
- `personal_access_token.go` - Value receiver for TableName

#### Backup Module
- `backup.go` - Updated to use utils.GenerateSecureToken, removed redundant methods

#### Base Models
- `base.go` - Added SiteScopedModel, GetUserHomeDir(), GetWorkingDir()

### Lines of Code Reduced
- ~60 lines of getter/setter boilerplate removed from site.go
- ~25 lines of getter/setter boilerplate removed from certificate.go
- ~10 lines of getter/setter boilerplate removed from daemon.go
- ~20 lines of duplicated token generation logic removed
- ~20 lines of duplicated log path logic simplified
- ~10 lines of redundant wrapper methods removed

### Benefits
1. **Type Safety** - JSON arrays now properly typed as slices
2. **Automatic Encryption** - Sensitive fields encrypted at rest automatically
3. **DRY Principle** - Common utilities centralized
4. **Consistency** - All TableName methods use value receivers
5. **Maintainability** - Less boilerplate, cleaner code
