{{/* Go Template: deployment/make-deployment-current.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/deployment/make-deployment-current.blade.php */}}
cd {{ .Site.Path }}
ln -nfs --relative {{ .ReleaseDirectory }} {{ .CurrentDirectory }}
