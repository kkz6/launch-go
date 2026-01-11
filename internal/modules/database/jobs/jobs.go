package jobs

// Script templates for database operations

// GetDatabaseScriptTemplate returns the script template for creating a database
func GetDatabaseScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "CREATE DATABASE IF NOT EXISTS ` + "`{{.Name}}`" + ` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
echo "Database {{.Name}} created successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "CREATE DATABASE \"{{.Name}}\";"
echo "Database {{.Name}} created successfully"
`
	default:
		return ""
	}
}

// GetDropDatabaseScriptTemplate returns the script template for dropping a database
func GetDropDatabaseScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "DROP DATABASE IF EXISTS ` + "`{{.Name}}`" + `;"
echo "Database {{.Name}} dropped successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "DROP DATABASE IF EXISTS \"{{.Name}}\";"
echo "Database {{.Name}} dropped successfully"
`
	default:
		return ""
	}
}

// GetCreateUserScriptTemplate returns the script template for creating a database user
func GetCreateUserScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "CREATE USER IF NOT EXISTS '{{.Name}}'@'{{.Host}}' IDENTIFIED BY '{{.Password}}';"
{{range .Databases}}
mysql -u root -e "GRANT ALL PRIVILEGES ON ` + "`{{.}}`" + `.* TO '{{$.Name}}'@'{{$.Host}}';"
{{end}}
mysql -u root -e "FLUSH PRIVILEGES;"
echo "User {{.Name}} created successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "CREATE USER \"{{.Name}}\" WITH PASSWORD '{{.Password}}';"
{{range .Databases}}
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE \"{{.}}\" TO \"{{$.Name}}\";"
{{end}}
echo "User {{.Name}} created successfully"
`
	default:
		return ""
	}
}

// GetUpdateUserPasswordScriptTemplate returns the script template for updating a user password
func GetUpdateUserPasswordScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "ALTER USER '{{.Name}}'@'{{.Host}}' IDENTIFIED BY '{{.Password}}';"
mysql -u root -e "FLUSH PRIVILEGES;"
echo "User {{.Name}} password updated successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "ALTER USER \"{{.Name}}\" WITH PASSWORD '{{.Password}}';"
echo "User {{.Name}} password updated successfully"
`
	default:
		return ""
	}
}

// GetDropUserScriptTemplate returns the script template for dropping a database user
func GetDropUserScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "DROP USER IF EXISTS '{{.Name}}'@'{{.Host}}';"
mysql -u root -e "FLUSH PRIVILEGES;"
echo "User {{.Name}} dropped successfully"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "DROP USER IF EXISTS \"{{.Name}}\";"
echo "User {{.Name}} dropped successfully"
`
	default:
		return ""
	}
}

// GetListDatabasesScriptTemplate returns the script template for listing databases
func GetListDatabasesScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -N -e "SHOW DATABASES;" | grep -Ev "^(information_schema|performance_schema|mysql|sys)$"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -t -c "SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres');"
`
	default:
		return ""
	}
}

// GetListUsersScriptTemplate returns the script template for listing database users
func GetListUsersScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -N -e "SELECT DISTINCT User FROM mysql.user WHERE User NOT IN ('root', 'mysql.sys', 'mysql.session', 'mysql.infoschema', 'debian-sys-maint');"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -t -c "SELECT usename FROM pg_user WHERE usename NOT IN ('postgres');"
`
	default:
		return ""
	}
}

// GetGrantPrivilegesScriptTemplate returns the script template for granting privileges
func GetGrantPrivilegesScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "GRANT ALL PRIVILEGES ON ` + "`{{.Database}}`" + `.* TO '{{.User}}'@'{{.Host}}';"
mysql -u root -e "FLUSH PRIVILEGES;"
echo "Privileges granted on {{.Database}} to {{.User}}"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE \"{{.Database}}\" TO \"{{.User}}\";"
echo "Privileges granted on {{.Database}} to {{.User}}"
`
	default:
		return ""
	}
}

// GetRevokePrivilegesScriptTemplate returns the script template for revoking privileges
func GetRevokePrivilegesScriptTemplate(dbType string) string {
	switch dbType {
	case "mysql":
		return `#!/bin/bash
set -e
mysql -u root -e "REVOKE ALL PRIVILEGES ON ` + "`{{.Database}}`" + `.* FROM '{{.User}}'@'{{.Host}}';"
mysql -u root -e "FLUSH PRIVILEGES;"
echo "Privileges revoked on {{.Database}} from {{.User}}"
`
	case "postgres":
		return `#!/bin/bash
set -e
sudo -u postgres psql -c "REVOKE ALL PRIVILEGES ON DATABASE \"{{.Database}}\" FROM \"{{.User}}\";"
echo "Privileges revoked on {{.Database}} from {{.User}}"
`
	default:
		return ""
	}
}
