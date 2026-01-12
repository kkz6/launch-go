#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "DROP DATABASE IF EXISTS {{ .DatabaseName }};"
