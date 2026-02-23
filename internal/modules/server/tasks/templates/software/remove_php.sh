#!/bin/bash
{{ shellDefaults }}

{{ aptFunctions }}

echo "Remove PHP {{ .Version }}"

waitForAptUnlock
sudo apt-get purge -y 'php{{ .Version }}-*'
sudo apt-get autoremove -y
sudo apt-get autoclean -y

echo "PHP {{ .Version }} removed successfully."
