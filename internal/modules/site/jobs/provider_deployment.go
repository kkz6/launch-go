package jobs

import "github.com/kkz6/launch-go/internal/pkg/util"

func deploymentLogURL(frontendURL, serverID, siteID, deploymentID string) string {
	if frontendURL == "" {
		return ""
	}
	return util.New(frontendURL).
		Path("servers", serverID, "sites", siteID).
		Query("tab", "deployments").
		Query("deployment", deploymentID).
		String()
}
