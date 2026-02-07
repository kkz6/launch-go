cd {{ .SitePath }}

echo "Downloading phpMyAdmin {{ .Version }}..."
curl -LOs {{ .DownloadURL }}

echo "Extracting phpMyAdmin..."
tar -xzf phpMyAdmin-*-all-languages.tar.gz
mv phpMyAdmin-*-all-languages/* {{ .RepositoryDirectory }}/
rm -f phpMyAdmin-*-all-languages.tar.gz
rm -rf phpMyAdmin-*-all-languages

CONFIG_PATH="{{ .RepositoryDirectory }}/config.inc.php"
SAMPLE_CONFIG_PATH="{{ .RepositoryDirectory }}/config.sample.inc.php"

if [ ! -f "$CONFIG_PATH" ] && [ -f "$SAMPLE_CONFIG_PATH" ]; then
    cp "$SAMPLE_CONFIG_PATH" "$CONFIG_PATH"
    BLOWFISH_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    sed -i "s|\$cfg\['blowfish_secret'\] = ''|\$cfg['blowfish_secret'] = '${BLOWFISH_SECRET}'|g" "$CONFIG_PATH"
fi

echo "Done!"
