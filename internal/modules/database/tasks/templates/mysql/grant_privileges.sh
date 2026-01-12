#!/bin/bash
{{ shellDefaults }}

{{ range .Hosts }}
sudo mysql --user="{{ $.AdminUser }}" --password="{{ $.AdminPassword }}" -e "GRANT ALL PRIVILEGES ON {{ $.DatabaseName }}.* TO '{{ $.Username }}'@'{{ . }}';"
{{ end }}
sudo mysql --user="{{ .AdminUser }}" --password="{{ .AdminPassword }}" -e "FLUSH PRIVILEGES;"
