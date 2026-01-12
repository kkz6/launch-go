#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE {{ .DatabaseName }} TO {{ .Username }};"
