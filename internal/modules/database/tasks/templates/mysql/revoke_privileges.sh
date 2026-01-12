#!/bin/bash
{{ shellDefaults }}

{{ range .Hosts }}
sudo mysql --user="{{ $.AdminUser }}" --password="{{ $.AdminPassword }}" -e "REVOKE ALL PRIVILEGES ON {{ $.DatabaseName }}.* FROM '{{ $.Username }}'@'{{ . }}';"
{{ end }}
sudo mysql --user="{{ .AdminUser }}" --password="{{ .AdminPassword }}" -e "FLUSH PRIVILEGES;"
