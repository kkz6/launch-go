{{ shellDefaults }}

echo "Rolling back to previous release..."

{{ template "deployment/make_deployment_current.sh" . }}

echo "Rollback completed successfully!"
printf 'Current release is now: %s\n' '{{ escape .ReleaseDirectory }}'
