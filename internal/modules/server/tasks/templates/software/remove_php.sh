#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove PHP {{ .Version }}"

waitForAptUnlock
aptGet purge -y 'php{{ .Version }}-*'
aptGet autoremove -y
aptGet autoclean -y

echo "PHP {{ .Version }} removed successfully."
