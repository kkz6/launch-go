#!/bin/bash
{{ shellDefaults }}

sudo -u postgres psql -c "CREATE DATABASE {{ .DatabaseName }} OWNER {{ .Owner }};"
