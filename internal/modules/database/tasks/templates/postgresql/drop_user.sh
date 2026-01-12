#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "DROP USER IF EXISTS {{ .Username }};"
