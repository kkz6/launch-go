{{/* Go Template: deployment/cleanup-old-releases.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/cleanup-old-releases.blade.php */}}
DEPLOYMENT_KEEP="{{ .DeploymentKeep }}"

# Get a list of all deployments, sorted by timestamp in ascending order
DEPLOYMENT_LIST=($(ls -1 {{ .ReleasesDirectory }} | sort -n))

# Determine how many deployments to delete
NUM_TO_DELETE=$((${#DEPLOYMENT_LIST[@]} - {{ .Site.DeploymentReleasesRetention }}))

# Loop through the deployments to delete
for ((i=0; i<$NUM_TO_DELETE; i++)); do
    DEPLOY=${DEPLOYMENT_LIST[$i]}
    # Skip the deployment to keep
    if [[ $DEPLOY == $DEPLOYMENT_KEEP ]]; then
        continue
    fi

    # Delete the deployment
    rm -rf {{ .ReleasesDirectory }}/$DEPLOY
done
