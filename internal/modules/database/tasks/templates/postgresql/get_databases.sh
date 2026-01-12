#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -t -c "SELECT datname FROM pg_database WHERE datistemplate = false;"
