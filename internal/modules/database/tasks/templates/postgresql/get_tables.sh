#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -d {{ .DatabaseName }} -t -c "SELECT tablename FROM pg_tables WHERE schemaname = 'public';"
