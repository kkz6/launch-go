package templates

import (
	"fmt"
	"time"
)

// ServerProvisionedEmail creates an HTML email for server provisioning success
func ServerProvisionedEmail(serverName, serverIP, serverUsername, dashboardURL string, databasePassword string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Server Provisioned Successfully").
		WithIntro(fmt.Sprintf("Your server **%s** has been provisioned and is ready to use.", serverName)).
		WithIntro("You can access your server by clicking the button below.").
		WithAction("View Server Dashboard", dashboardURL, "primary")

	// Server details panel
	details := fmt.Sprintf(`**Server Name:** `+"`%s`"+`

**IP Address:** `+"`%s`"+`

**Username:** `+"`%s`"+`

**Status:** Online`, serverName, serverIP, serverUsername)

	if databasePassword != "" {
		details += fmt.Sprintf(`

**Database Password:** `+"`%s`", databasePassword)
	}

	builder.WithPanel(details)

	// SSH access panel
	builder.WithIntro("Instead of accessing your server through the web interface, you can also connect to it using the following command:")
	builder.WithPanel(fmt.Sprintf("`ssh %s@%s`", serverUsername, serverIP))
	builder.WithOutro("**Note:** SSH access requires key-based authentication. Password login is disabled for security.")
	builder.WithOutro("Good luck with your new server!")

	if dashboardURL != "" {
		builder.WithSubcopy(fmt.Sprintf("If you're having trouble clicking the \"View Server Dashboard\" button, copy and paste the URL below into your web browser: %s", dashboardURL))
	}

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// DeploymentFailedEmail creates an HTML email for deployment failures
func DeploymentFailedEmail(siteAddress, serverName, statusLabel, gitHash, commitMessage, commitAuthor, triggeredBy string, deploymentTime time.Time, output string, siteURL string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting(fmt.Sprintf("Deployment %s", statusLabel))

	intro := fmt.Sprintf("Deployment **%s** for site **%s** on server **%s**.", statusLabel, siteAddress, serverName)
	builder.WithIntro(intro)

	// Build details panel
	var details string
	if gitHash != "" {
		shortHash := gitHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		details += fmt.Sprintf("**Commit:** `%s`\n\n", shortHash)
	}

	if commitMessage != "" {
		details += fmt.Sprintf("**Commit Message:** %s\n\n", commitMessage)
	}

	if commitAuthor != "" {
		details += fmt.Sprintf("**Author:** %s\n\n", commitAuthor)
	}

	if triggeredBy != "" {
		details += fmt.Sprintf("**Triggered by:** %s\n\n", triggeredBy)
	}

	details += fmt.Sprintf("**Time:** %s", deploymentTime.Format(time.RFC1123))

	if details != "" {
		builder.WithPanel(details)
	}

	if output != "" {
		builder.WithIntro("**Last lines of output:**")
		builder.WithPanel("```\n" + output + "\n```")
	}

	if siteURL != "" {
		builder.WithAction("View Site", siteURL, "primary")
	}

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// TeamInvitationEmail creates an HTML email for team invitations
func TeamInvitationEmail(teamName, acceptURL, registerURL string, hasRegistration bool) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Team Invitation").
		WithIntro(fmt.Sprintf("You have been invited to join the **%s** team!", teamName))

	if hasRegistration && registerURL != "" {
		builder.WithIntro("If you do not have an account, you may create one by clicking the button below. After creating an account, you may click the invitation acceptance button in this email to accept the team invitation:")
		builder.WithAction("Create Account", registerURL, "success")
		builder.WithIntro("If you already have an account, you may accept this invitation by clicking the button below:")
	} else {
		builder.WithIntro("You may accept this invitation by clicking the button below:")
	}

	builder.WithAction("Accept Invitation", acceptURL, "primary")
	builder.WithOutro("If you did not expect to receive an invitation to this team, you may discard this email.")

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// PlatformInvitationEmail creates an HTML email inviting someone to try the
// platform. The trial runs free until trialEndsAt; the recipient activates it
// by registering through inviteURL.
func PlatformInvitationEmail(inviteURL string, trialEndsAt time.Time) (htmlContent string, plainText string, err error) {
	cfg := GetConfig()
	appName := cfg.AppName
	if appName == "" {
		appName = "Launch"
	}

	builder := NewEmail().
		WithGreeting("You're Invited").
		WithIntro(fmt.Sprintf("You've been invited to try **%s** — free until **%s**.", appName, trialEndsAt.Format("January 2, 2006"))).
		WithIntro("Click the button below to create your account and start your trial:").
		WithAction("Accept Invitation", inviteURL, "primary").
		WithOutro("If you did not expect this invitation, you may discard this email.").
		WithSubcopy(fmt.Sprintf("If you're having trouble clicking the \"Accept Invitation\" button, copy and paste the URL below into your web browser: %s", inviteURL))

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// PasswordResetEmail creates an HTML email for password reset
func PasswordResetEmail(resetURL string, expiresIn int) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Reset Your Password").
		WithIntro("You are receiving this email because we received a password reset request for your account.").
		WithAction("Reset Password", resetURL, "primary").
		WithOutro(fmt.Sprintf("This password reset link will expire in %d minutes.", expiresIn)).
		WithOutro("If you did not request a password reset, no further action is required.").
		WithSubcopy(fmt.Sprintf("If you're having trouble clicking the \"Reset Password\" button, copy and paste the URL below into your web browser: %s", resetURL))

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// EmailVerificationEmail creates an HTML email for email verification
func EmailVerificationEmail(verifyURL string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Verify Your Email Address").
		WithIntro("Please click the button below to verify your email address.").
		WithAction("Verify Email Address", verifyURL, "primary").
		WithOutro("If you did not create an account, no further action is required.").
		WithSubcopy(fmt.Sprintf("If you're having trouble clicking the \"Verify Email Address\" button, copy and paste the URL below into your web browser: %s", verifyURL))

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// ServerThresholdExceededEmail creates an HTML email for threshold alerts
func ServerThresholdExceededEmail(serverName, metricName string, threshold, currentValue float64, serverURL string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Server Threshold Alert").
		WithIntro(fmt.Sprintf("The server **%s** has exceeded the configured threshold for **%s**.", serverName, metricName))

	details := fmt.Sprintf(`**Metric:** %s

**Threshold:** %.1f%%

**Current Value:** %.1f%%`, metricName, threshold, currentValue)

	builder.WithPanel(details)
	builder.WithOutro("Please investigate this issue to ensure optimal server performance.")

	if serverURL != "" {
		builder.WithAction("View Server", serverURL, "error")
	}

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// GenericFailureEmail creates an HTML email for generic failures
func GenericFailureEmail(title, message, output, errorMessage, actionURL, actionText string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting(title).
		WithIntro(message)

	if output != "" {
		builder.WithIntro("**Last lines of output:**")
		builder.WithPanel("```\n" + output + "\n```")
	}

	if errorMessage != "" {
		builder.WithIntro("**Error message:**")
		builder.WithPanel("```\n" + errorMessage + "\n```")
	}

	if actionURL != "" && actionText != "" {
		builder.WithAction(actionText, actionURL, "error")
	}

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// VulnerabilityAuditEmail creates an HTML email for vulnerability audit results
func VulnerabilityAuditEmail(serverName string, vulnerabilitiesFound int, reportURL string) (htmlContent string, plainText string, err error) {
	var greeting string
	var color string

	if vulnerabilitiesFound > 0 {
		greeting = "Vulnerability Audit Completed - Issues Found"
		color = "error"
	} else {
		greeting = "Vulnerability Audit Completed - All Clear"
		color = "success"
	}

	builder := NewEmail().
		WithGreeting(greeting).
		WithIntro(fmt.Sprintf("The vulnerability audit for server **%s** has completed.", serverName))

	if vulnerabilitiesFound > 0 {
		builder.WithIntro(fmt.Sprintf("**%d vulnerabilities** were found during the audit.", vulnerabilitiesFound))
		builder.WithOutro("Please review the results and apply necessary security patches.")
	} else {
		builder.WithIntro("No vulnerabilities were found during the audit.")
		builder.WithOutro("Your server is up to date with security patches.")
	}

	if reportURL != "" {
		builder.WithAction("View Report", reportURL, color)
	}

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}

// ConnectionTestEmail creates an HTML email for testing email connections
func ConnectionTestEmail() (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Email Connection Successful").
		WithIntro("This email confirms that your email notification settings are configured correctly.").
		WithOutro("You will now receive notifications at this email address.")

	html, err := builder.Build()
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}
