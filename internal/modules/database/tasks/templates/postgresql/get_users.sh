#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -t -c "SELECT usename FROM pg_user;"
