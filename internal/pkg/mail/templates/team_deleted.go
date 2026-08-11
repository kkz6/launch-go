package templates

import "fmt"

var renderTeamDeletedEmail = func(builder *EmailBuilder) (string, error) {
	return builder.Build()
}

func TeamDeletedEmail(teamName, destinationName string) (htmlContent string, plainText string, err error) {
	builder := NewEmail().
		WithGreeting("Team Deleted").
		WithIntro(fmt.Sprintf("The **%s** team has been deleted.", teamName)).
		WithIntro(fmt.Sprintf("Its resources were transferred to **%s**.", destinationName)).
		WithOutro("No further action is required.")

	html, err := renderTeamDeletedEmail(builder)
	if err != nil {
		return "", "", err
	}

	return html, builder.BuildPlainText(), nil
}
