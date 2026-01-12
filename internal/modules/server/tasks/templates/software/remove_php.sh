#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove PHP {{ .Version }}"

waitForAptUnlock
sudo apt-get purge -y 'php{{ .Version }}-*'
sudo apt-get autoremove -y

echo "PHP {{ .Version }} removed successfully."
