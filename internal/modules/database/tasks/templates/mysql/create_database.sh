#!/bin/bash
{{ shellDefaults }}

sudo mysql --user="{{ .User }}" --password="{{ .Password }}" -e "CREATE DATABASE {{ .DatabaseName }} CHARACTER SET {{ .Charset }} COLLATE {{ .Collation }};"
