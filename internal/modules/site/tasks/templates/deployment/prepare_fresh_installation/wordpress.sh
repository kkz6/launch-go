cd {{ .SitePath }}

echo "Downloading the latest Wordpress release..."
curl -LOs https://wordpress.org/latest.zip

echo "Extracting Wordpress repository..."
unzip -qq -o latest.zip
mv wordpress/* {{ .RepositoryDirectory }}/
rm latest.zip
rm -rf wordpress

CONFIG_PATH="{{ .RepositoryDirectory }}/wp-config.php"
SAMPLE_CONFIG_PATH="{{ .RepositoryDirectory }}/wp-config-sample.php"

if [ ! -f $CONFIG_PATH ] && [ -f $SAMPLE_CONFIG_PATH ]; then
    cp $SAMPLE_CONFIG_PATH $CONFIG_PATH
    cd {{ .RepositoryDirectory }}

    {{ range $key, $value := .EnvVariables }}
        sed -i --follow-symlinks "s|^define( '{{ $key }}',.*|define('{{ $key }}', '{{ $value }}');|g" wp-config.php
    {{ end }}

    sed -i --follow-symlinks '/\/* Add any custom values between this line and the "stop editing" line.*/a define( "DISABLE_WP_CRON", true );' wp-config.php

fi

echo "Done!"
