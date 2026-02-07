cd {{ .SitePath }}

echo "Downloading the latest phpMyAdmin release..."
curl -LOs https://www.phpmyadmin.net/downloads/phpMyAdmin-latest-all-languages.tar.gz

echo "Extracting phpMyAdmin..."
tar -xzf phpMyAdmin-latest-all-languages.tar.gz
mv phpMyAdmin-*-all-languages/* {{ .RepositoryDirectory }}/
rm phpMyAdmin-latest-all-languages.tar.gz
rm -rf phpMyAdmin-*-all-languages

CONFIG_PATH="{{ .RepositoryDirectory }}/config.inc.php"
SAMPLE_CONFIG_PATH="{{ .RepositoryDirectory }}/config.sample.inc.php"

if [ ! -f "$CONFIG_PATH" ] && [ -f "$SAMPLE_CONFIG_PATH" ]; then
    cp "$SAMPLE_CONFIG_PATH" "$CONFIG_PATH"

    # Generate a random blowfish secret (32 characters)
    BLOWFISH_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)

    # Set the blowfish secret
    sed -i "s|\$cfg\['blowfish_secret'\] = ''|\$cfg['blowfish_secret'] = '${BLOWFISH_SECRET}'|g" "$CONFIG_PATH"
fi

echo "Done!"
