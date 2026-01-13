# Tests

This directory contains all tests for the launch-go application.

## Structure

```
tests/
├── snapshots/          # Snapshot test data
│   ├── tasks/          # Task script snapshots
│   └── jobs/           # Job output snapshots
├── jobs/               # Job tests
├── tasks/              # Task tests
└── integration/        # Integration tests
```

## Running Tests

```bash
# Run all tests
go test ./tests/...

# Run with verbose output
go test -v ./tests/...

# Run specific package tests
go test ./tests/jobs/...
go test ./tests/tasks/...

# Update snapshots
UPDATE_SNAPSHOTS=true go test ./tests/...
```

## Snapshot Testing

Snapshots are used to test that task scripts and job outputs remain consistent.
When you make intentional changes, update the snapshots by running tests with
`UPDATE_SNAPSHOTS=true`.

Snapshot files are stored in `tests/snapshots/` and are committed to version control.
