#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "REVOKE ALL PRIVILEGES ON DATABASE {{ .DatabaseName }} FROM {{ .Username }};"
