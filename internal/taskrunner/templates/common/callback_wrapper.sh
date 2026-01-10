{{/* Wrapper script for background execution with callbacks */}}
{{define "callback_wrapper"}}
#!/bin/bash
set -e

{{template "helpers"}}

# ============================================================================
# Background Task Wrapper with Callbacks
# ============================================================================

# Create temp script with actual task content
TASK_SCRIPT=$(mktemp)
cat > "$TASK_SCRIPT" << 'LAUNCH_TASK_EOF'
{{.Script}}
LAUNCH_TASK_EOF

# Execute with timeout
print_info "Starting task execution (timeout: {{.Timeout}}s)"
timeout {{.Timeout}}s bash "$TASK_SCRIPT"
EXIT_CODE=$?

# Clean up
rm -f "$TASK_SCRIPT"

# Call appropriate webhook based on exit code
if [ $EXIT_CODE -eq 0 ]; then
    print_success "Task completed successfully"
    httpPostSilently "{{.FinishedURL}}"
elif [ $EXIT_CODE -eq 124 ]; then
    print_error "Task timed out"
    httpPostSilently "{{.TimeoutURL}}"
else
    print_error "Task failed with exit code: $EXIT_CODE"
    httpPostSilently "{{.FailedURL}}" "{\"exit_code\":$EXIT_CODE}"
fi

exit $EXIT_CODE
{{end}}
