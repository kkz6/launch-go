cd {{ .SitePath }}
ln -nfs --relative {{ .ReleaseDirectory }} {{ .CurrentDirectory }}
