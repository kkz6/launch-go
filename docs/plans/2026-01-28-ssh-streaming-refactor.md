# SSH Streaming Refactor: Remove HTTP Callbacks

**Date:** 2026-01-28
**Status:** Mostly Complete (reconnect/resume pending)

## Overview

Refactor the task runner to use SSH-based output streaming with markers instead of HTTP callbacks. This simplifies the architecture and removes the requirement for servers to reach back to the application.

## Background

### Current Architecture (PHP Legacy)

```
┌─────────────────────────────────────────────────────────────┐
│  Server                                                      │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ nohup bash script.sh > output.log 2>&1 &                ││
│  │                                                          ││
│  │ # Script contains HTTP callbacks:                        ││
│  │ httpPostSilently "$URL" '{"step":"configure_swap"}'      ││
│  │ httpPostSilently "$URL" '{"software":"php83"}'           ││
│  └──────────────────────┬──────────────────────────────────┘│
│                         │                                    │
└─────────────────────────┼────────────────────────────────────┘
                          │ HTTP POST (requires signed URLs)
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Go App                                                      │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ /api/tasks/:id/callback endpoint                         ││
│  │   - Verify signed URL                                    ││
│  │   - Update database                                      ││
│  │   - Broadcast to WebSocket                               ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Local Mode: SSH polling fallback (different code path)     │
└─────────────────────────────────────────────────────────────┘
```

**Problems:**
- Server must be able to reach the application (problematic behind NAT/firewall)
- Requires signed URL generation and verification
- Requires HTTP callback endpoints
- Two different code paths for local vs production
- Complex error handling for failed callbacks

### New Architecture (SSH Streaming)

```
┌─────────────────────────────────────────────────────────────┐
│  Server                                                      │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ nohup bash script.sh > output.log 2>&1 &                ││
│  │                                                          ││
│  │ # Script outputs markers to stdout:                      ││
│  │ echo "::LAUNCH::step_completed::configure_swap"          ││
│  │ echo "::LAUNCH::software_installed::php83"               ││
│  │ echo "::LAUNCH::progress::50"                            ││
│  └─────────────────────────────────────────────────────────┘│
│                         ▲                                    │
│                         │ tail -f output.log                 │
│                         │                                    │
└─────────────────────────┼────────────────────────────────────┘
                          │ SSH (Go initiates connection)
                          │
┌─────────────────────────┼────────────────────────────────────┐
│  Go App                 │                                    │
│  ┌──────────────────────┴──────────────────────────────────┐│
│  │ StreamMonitor                                            ││
│  │   - Parse output for markers                             ││
│  │   - Update database on markers                           ││
│  │   - Broadcast to WebSocket                               ││
│  │   - Reconnect on connection drop                         ││
│  └─────────────────────────────────────────────────────────┘│
│                                                              │
│  Same code path for local and production                     │
└─────────────────────────────────────────────────────────────┘
```

**Benefits:**
- Works everywhere (app initiates connection to server)
- No signed URLs needed
- No HTTP callback endpoints
- Single code path for all environments
- Simpler error handling
- Debuggable (markers visible in logs)

## Marker Protocol

### Format

```
::LAUNCH::<type>::<value>
```

### Marker Types

| Type | Value | Description |
|------|-------|-------------|
| `step_completed` | step name | Provision step finished |
| `software_installed` | software name | Software installation finished |
| `progress` | 0-100 | Progress percentage |
| `status` | message | Status message for UI |
| `error` | message | Error message (non-fatal) |

### Examples

```bash
echo "::LAUNCH::status::Configuring swap file"
echo "::LAUNCH::step_completed::configure_swap"
echo "::LAUNCH::progress::15"

echo "::LAUNCH::status::Installing PHP 8.3"
echo "::LAUNCH::software_installed::php83"
echo "::LAUNCH::progress::50"
```

## Implementation Plan

### Phase 1: Add Marker Infrastructure

- [x] Create `internal/pkg/taskrunner/markers` package
- [x] Add marker parsing functions
- [x] Add marker constants for all types
- [x] Write tests for marker parsing

### Phase 2: Update Stream Monitor

- [x] Update `StreamMonitor` to parse markers from output
- [x] Add marker handler callbacks (MarkerHandler interface)
- [x] Add WebSocket broadcast for markers
- [x] Add database update logic for markers (via ProvisionMarkerHandler)
- [ ] Handle reconnection and resume

### Phase 3: Update Task Scripts

- [x] Update `ProvisionFreshServer` to use markers instead of HTTP callbacks
- [x] Remove `CallbackURL` from ProvisionFreshServerConfig
- [ ] Update all tasks that use HTTP callbacks
- [ ] Remove `httpPostSilently` from common functions (or keep for other uses)

### Phase 4: Remove HTTP Callback Infrastructure

- [x] Remove `CallbackURLs` struct from runner.go
- [x] Remove `WithCallbacks()` method from TaskRunner
- [x] Remove `hasCallbackURLs()` and `runWithCallbacks()` methods
- [x] Remove `wrapTaskForBackground()` method
- [x] Remove `WithOutputPolling()` and related code
- [x] Simplify `RunInBackground()` to directly use SSH streaming
- [x] Remove `localMode` flag from dispatcher
- [x] Remove `LocalMode` from TaskRunnerDeps
- [x] Update tests to remove callback-related tests
- [x] Remove callback endpoint handlers (task_webhook_handler.go)
- [x] Remove legacy marker constants from stream_monitor.go
- [x] Remove legacy marker parsing from StreamTaskOutput and MonitorBackgroundTask

### Phase 5: Cleanup

- [x] Update tests
- [x] Remove callback URL support from BaseTask
- [x] Update TaskMarkerFunctions to use new marker format
- [ ] Update documentation (this file)
- [ ] Verify all other tasks use new markers (not just ProvisionFreshServer)

## Files to Modify

### New Files

```
internal/pkg/taskrunner/markers/
├── markers.go        # Marker constants and parsing ✅
└── markers_test.go   # Tests ✅

internal/modules/server/tasks/
├── provision_marker_handler.go  # Database/WebSocket handler for provision markers ✅
```

### Modified Files

```
internal/pkg/taskrunner/
├── stream_monitor.go   # Add marker parsing and handling ✅
├── dispatcher.go       # Remove localMode, simplify to single path ✅
├── task.go             # Remove callback URL support ✅

internal/modules/server/tasks/
├── provision_fresh_server.go  # Use markers instead of HTTP callbacks ✅
├── runner.go                  # Remove callbacks, add WithMarkerHandler ✅
├── runner_test.go             # Remove callback tests ✅

internal/modules/server/jobs/
├── provision_server.go        # Use marker handler during provisioning ✅

internal/pkg/taskrunner/templates/
├── functions.go        # SIGPIPE handling + update marker functions to new format ✅

cmd/worker/main.go      # Remove LocalMode from dispatcher config ✅
```

### Removed Files/Code

```
internal/modules/server/handlers/task_webhook_handler.go  # Removed ✅
internal/modules/server/tasks/runner.go:
  - CallbackURLs struct ✅
  - WithCallbacks(), hasCallbackURLs(), runWithCallbacks() ✅
  - wrapTaskForBackground() ✅
  - WithOutputPolling(), dispatchOutputPolling() ✅
internal/modules/server/tasks/runner_test.go:
  - TestTaskRunner_WithCallbacks ✅
  - TestCallbackURLs ✅
  - TestTaskRunner_hasCallbackURLs ✅
  - TestTaskRunner_wrapTaskForBackground ✅
internal/pkg/taskrunner/dispatcher.go:
  - localMode field ✅
  - LocalMode config option ✅
  - IsLocalMode() method ✅
internal/pkg/taskrunner/stream_monitor.go:
  - Legacy marker constants ✅
  - Legacy marker parsing code ✅
internal/pkg/taskrunner/task.go:
  - WithCallbackURL() option ✅
  - callbackURL field ✅
  - GetCallbackURL() method ✅
internal/pkg/taskrunner/templates/functions.go:
  - Legacy marker functions updated to new format ✅
```

## Rollout Strategy

1. **Implement markers alongside callbacks** - Both work during transition
2. **Test with markers** - Verify marker-based progress works
3. **Remove callbacks** - Once markers are proven, remove callback code
4. **Cleanup** - Remove all callback infrastructure

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| SSH connection drops during long tasks | Background execution + reconnect + resume |
| Marker parsing errors | Strict format, ignore malformed markers |
| Missing progress updates | Periodic polling as fallback |
| Output buffer issues | Line-based processing, bounded buffers |

## Success Criteria

- [x] Provisioning works with SSH streaming only
- [x] Progress updates appear in real-time in UI (via markers)
- [x] Connection drops don't kill long-running tasks (nohup background execution)
- [ ] Can resume monitoring after app restart
- [x] No HTTP callback code remains
- [x] Single code path for all environments
