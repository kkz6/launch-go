#!/bin/bash
{{ shellDefaults }}

{{ range .Hosts }}
sudo mysql --user="{{ $.AdminUser }}" --password="{{ $.AdminPassword }}" -e "ALTER USER '{{ $.Username }}'@'{{ . }}' IDENTIFIED BY '{{ $.NewPassword }}';"
{{ end }}
sudo mysql --user="{{ .AdminUser }}" --password="{{ .AdminPassword }}" -e "FLUSH PRIVILEGES;"
