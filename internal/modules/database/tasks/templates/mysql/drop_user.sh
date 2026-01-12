#!/bin/bash
{{ shellDefaults }}

{{ range .Hosts }}
sudo mysql --user="{{ $.AdminUser }}" --password="{{ $.AdminPassword }}" -e "DROP USER IF EXISTS '{{ $.Username }}'@'{{ . }}';"
{{ end }}
