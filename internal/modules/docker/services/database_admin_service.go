package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/tasks"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// database_admin_service.go manages the individual databases that live
// INSIDE a managed Docker database instance (#75) — list / create / drop —
// by running engine CLIs (mysql / psql) over `docker exec` on the host,
// using the instance's stored admin credentials. Live: nothing is mirrored
// into our DB; each call reflects the container's real state.

// dbObjectNamePattern bounds names to a safe charset so they can be
// interpolated into engine SQL without escaping (and never injected).
var dbObjectNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// engineManageable reports whether per-database management applies to an
// engine. MySQL/MariaDB + PostgreSQL only — Redis/Mongo don't share the
// "named databases" model.
func engineManageable(e dockertypes.DatabaseEngine) bool {
	switch e {
	case dockertypes.DatabaseEngineMySQL, dockertypes.DatabaseEngineMariaDB, dockertypes.DatabaseEnginePostgres:
		return true
	default:
		return false
	}
}

type dbInstanceContext struct {
	container string
	creds     Credentials
	engine    dockertypes.DatabaseEngine
}

// resolveInstance loads the instance + project, validates ownership and
// engine support, and returns the container name + admin credentials.
func (s *DatabaseService) resolveInstance(
	ctx context.Context, id, projectID, serverID, teamID string,
) (dbInstanceContext, error) {
	project, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return dbInstanceContext{}, err
	}
	d, err := s.Repos().Database().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dbInstanceContext{}, err
	}
	if d.ProjectID != projectID {
		return dbInstanceContext{}, fiberutil.NotFound()
	}
	if !engineManageable(d.Engine) {
		return dbInstanceContext{}, fiberutil.BadRequest(
			"Database management isn't supported for " + string(d.Engine) + " instances")
	}
	creds, err := decodeCredentials(d.Credentials)
	if err != nil {
		return dbInstanceContext{}, fiberutil.BadRequest("Instance credentials are missing or corrupt")
	}
	return dbInstanceContext{
		container: tasks.DatabaseContainerName(tasks.SlugFromName(project.Name), tasks.SlugFromName(d.Name)),
		creds:     creds,
		engine:    d.Engine,
	}, nil
}

// sshRunOnServer runs a single command on the docker host over SSH and
// returns stdout, surfacing a 400 with stderr when it fails.
func (s *DatabaseService) sshRunOnServer(ctx context.Context, serverID, teamID, cmd string) (string, error) {
	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return "", err
	}
	client, err := taskrunner.NewSSHClientFromServerAsRoot(server)
	if err != nil {
		return "", err
	}
	if err := client.Connect(); err != nil {
		return "", err
	}
	defer func() { _ = client.Close() }()

	res, err := client.Run(ctx, cmd)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return "", fiberutil.BadRequest("Command failed: " + msg)
	}
	return res.Stdout, nil
}

// shQuote single-quotes a string for safe shell interpolation.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isSystemDatabase hides engine-internal databases from the list.
func isSystemDatabase(engine dockertypes.DatabaseEngine, name string) bool {
	if engine == dockertypes.DatabaseEnginePostgres {
		switch name {
		case "postgres", "template0", "template1":
			return true
		}
		return false
	}
	switch name { // mysql / mariadb
	case "information_schema", "performance_schema", "mysql", "sys":
		return true
	}
	return false
}

// ListInstanceDatabases returns the user databases inside the instance.
func (s *DatabaseService) ListInstanceDatabases(
	ctx context.Context, id, projectID, serverID, teamID string,
) ([]string, error) {
	inst, err := s.resolveInstance(ctx, id, projectID, serverID, teamID)
	if err != nil {
		return nil, err
	}

	var cmd string
	if inst.engine == dockertypes.DatabaseEnginePostgres {
		cmd = fmt.Sprintf(
			"docker exec -e PGPASSWORD=%s %s psql -U %s -tAc 'SELECT datname FROM pg_database WHERE datistemplate = false'",
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username),
		)
	} else {
		cmd = fmt.Sprintf(
			"docker exec -e MYSQL_PWD=%s %s mysql -u %s -N -e 'SHOW DATABASES'",
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username),
		)
	}

	out, err := s.sshRunOnServer(ctx, serverID, teamID, cmd)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || isSystemDatabase(inst.engine, name) {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// CreateInstanceDatabase creates a database inside the instance.
func (s *DatabaseService) CreateInstanceDatabase(
	ctx context.Context, id, projectID, serverID, teamID, name string,
) error {
	if !dbObjectNamePattern.MatchString(name) {
		return fiberutil.BadRequest("Database name must be 1-64 characters of letters, digits, or underscore")
	}
	inst, err := s.resolveInstance(ctx, id, projectID, serverID, teamID)
	if err != nil {
		return err
	}

	var cmd string
	if inst.engine == dockertypes.DatabaseEnginePostgres {
		cmd = fmt.Sprintf(
			`docker exec -e PGPASSWORD=%s %s psql -U %s -tAc 'CREATE DATABASE "%s"'`,
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username), name,
		)
	} else {
		cmd = fmt.Sprintf(
			"docker exec -e MYSQL_PWD=%s %s mysql -u %s -e 'CREATE DATABASE `%s`'",
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username), name,
		)
	}
	_, err = s.sshRunOnServer(ctx, serverID, teamID, cmd)
	return err
}

// DropInstanceDatabase drops a database inside the instance.
func (s *DatabaseService) DropInstanceDatabase(
	ctx context.Context, id, projectID, serverID, teamID, name string,
) error {
	if !dbObjectNamePattern.MatchString(name) {
		return fiberutil.BadRequest("Invalid database name")
	}
	inst, err := s.resolveInstance(ctx, id, projectID, serverID, teamID)
	if err != nil {
		return err
	}

	var cmd string
	if inst.engine == dockertypes.DatabaseEnginePostgres {
		cmd = fmt.Sprintf(
			`docker exec -e PGPASSWORD=%s %s psql -U %s -tAc 'DROP DATABASE IF EXISTS "%s"'`,
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username), name,
		)
	} else {
		cmd = fmt.Sprintf(
			"docker exec -e MYSQL_PWD=%s %s mysql -u %s -e 'DROP DATABASE IF EXISTS `%s`'",
			shQuote(inst.creds.Password), shQuote(inst.container), shQuote(inst.creds.Username), name,
		)
	}
	_, err = s.sshRunOnServer(ctx, serverID, teamID, cmd)
	return err
}
