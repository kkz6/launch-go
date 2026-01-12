#!/bin/bash
{{ shellDefaults }}

{{ range .Hosts }}
sudo mysql --user="{{ $.AdminUser }}" --password="{{ $.AdminPassword }}" -e "CREATE USER IF NOT EXISTS '{{ $.Username }}'@'{{ . }}' IDENTIFIED BY '{{ $.UserPassword }}';"
{{ end }}
