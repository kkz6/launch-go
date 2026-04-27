package types

import "testing"

// ----- Kind -----

func TestKind_IsValid(t *testing.T) {
	for _, k := range []Kind{KindPostgres, KindMySQL, KindRedis} {
		if !k.IsValid() {
			t.Errorf("Kind %q must be valid", k)
		}
	}
	if Kind("oracle").IsValid() {
		t.Error("unknown kind should not be valid")
	}
}

func TestKind_DefaultImage(t *testing.T) {
	cases := map[Kind]string{
		KindPostgres: "postgres:16",
		KindMySQL:    "mysql:8.0",
		KindRedis:    "redis:7-alpine",
	}
	for k, want := range cases {
		if got := k.DefaultImage(); got != want {
			t.Errorf("DefaultImage(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestKind_InternalPort(t *testing.T) {
	cases := map[Kind]int{
		KindPostgres: 5432,
		KindMySQL:    3306,
		KindRedis:    6379,
	}
	for k, want := range cases {
		if got := k.InternalPort(); got != want {
			t.Errorf("InternalPort(%s) = %d, want %d", k, got, want)
		}
	}
}

func TestKind_RequiresDatabaseName(t *testing.T) {
	if !KindPostgres.RequiresDatabaseName() {
		t.Error("Postgres requires a database name")
	}
	if !KindMySQL.RequiresDatabaseName() {
		t.Error("MySQL requires a database name")
	}
	if KindRedis.RequiresDatabaseName() {
		t.Error("Redis must not require a database name")
	}
}

func TestKind_ContainerName(t *testing.T) {
	cases := map[Kind]string{
		KindPostgres: "launch-postgres",
		KindMySQL:    "launch-mysql",
		KindRedis:    "launch-redis",
	}
	for k, want := range cases {
		if got := k.ContainerName(); got != want {
			t.Errorf("ContainerName(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestKind_VolumeName(t *testing.T) {
	if KindPostgres.VolumeName() != "launch-postgres-data" {
		t.Errorf("postgres volume = %q", KindPostgres.VolumeName())
	}
	if KindMySQL.VolumeName() != "launch-mysql-data" {
		t.Errorf("mysql volume = %q", KindMySQL.VolumeName())
	}
	if KindRedis.VolumeName() != "launch-redis-data" {
		t.Errorf("redis volume = %q", KindRedis.VolumeName())
	}
}

func TestKind_RunTemplateName(t *testing.T) {
	if KindPostgres.RunTemplateName() != "dockerservice/run_postgres.sh" {
		t.Errorf("got %q", KindPostgres.RunTemplateName())
	}
	if KindMySQL.RunTemplateName() != "dockerservice/run_mysql.sh" {
		t.Errorf("got %q", KindMySQL.RunTemplateName())
	}
	if KindRedis.RunTemplateName() != "dockerservice/run_redis.sh" {
		t.Errorf("got %q", KindRedis.RunTemplateName())
	}
}

func TestKind_Label(t *testing.T) {
	cases := map[Kind]string{
		KindPostgres: "PostgreSQL",
		KindMySQL:    "MySQL",
		KindRedis:    "Redis",
	}
	for k, want := range cases {
		if got := k.Label(); got != want {
			t.Errorf("Label(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestParseKind(t *testing.T) {
	for _, in := range []string{"postgres", "mysql", "redis"} {
		k, err := ParseKind(in)
		if err != nil {
			t.Errorf("ParseKind(%s) error: %v", in, err)
		}
		if string(k) != in {
			t.Errorf("ParseKind(%s) = %s", in, k)
		}
	}
	if _, err := ParseKind("oracle"); err == nil {
		t.Error("ParseKind(oracle) must fail")
	}
}

// ----- Status -----

func TestStatus_IsValid(t *testing.T) {
	for _, s := range []Status{
		StatusPending, StatusInstalling, StatusRunning, StatusStopped, StatusFailed,
	} {
		if !s.IsValid() {
			t.Errorf("Status %q must be valid", s)
		}
	}
	if Status("zombie").IsValid() {
		t.Error("unknown status must not be valid")
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	if !StatusRunning.IsTerminal() {
		t.Error("running is terminal")
	}
	if !StatusStopped.IsTerminal() {
		t.Error("stopped is terminal")
	}
	if !StatusFailed.IsTerminal() {
		t.Error("failed is terminal")
	}
	if StatusPending.IsTerminal() {
		t.Error("pending is not terminal")
	}
	if StatusInstalling.IsTerminal() {
		t.Error("installing is not terminal")
	}
}
