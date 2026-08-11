package templates

import "time"

// baseLayout is the shared, email-client-safe product shell used by all HTML emails.
var baseLayout = `<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .AppName }}</title>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<style>
` + emailCSS + `
</style>
</head>
<body style="margin:0; padding:0; width:100% !important; background-color:#f4f4f5; color:#3f3f46; -webkit-text-size-adjust:none;">
{{ if .Preheader }}
<div class="preheader" style="display:none !important; max-height:0; max-width:0; opacity:0; overflow:hidden; mso-hide:all; color:transparent; line-height:1px; font-size:1px;">
{{ .Preheader }}&#847;&zwnj;&nbsp;&#847;&zwnj;&nbsp;&#847;&zwnj;&nbsp;&#847;&zwnj;&nbsp;
</div>
{{ end }}

<table class="wrapper" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; margin:0; padding:0; background-color:#f4f4f5;">
<tr>
<td class="wrapper-cell" align="center" style="padding:32px 12px;">
<!--[if mso]>
<table width="600" cellpadding="0" cellspacing="0" role="presentation"><tr><td>
<![endif]-->
<table class="content" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; max-width:600px; margin:0 auto; padding:0; table-layout:fixed;">

<!-- Header -->
<tr>
<td style="padding:0 4px 18px;">
<table class="email-header" align="center" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; max-width:600px; margin:0 auto;">
<tr>
<td align="left">
<a href="{{ .AppURL }}" class="brand" style="display:inline-block; color:#18181b; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif; font-size:20px; font-weight:800; letter-spacing:-0.04em; text-decoration:none;">{{ .BrandName }}</a>
</td>
<td align="right">
<span class="header-label" style="color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,monospace; font-size:11px; font-weight:600; line-height:16px;">{{ if eq .LayoutVariant "incident" }}lctl / deploy{{ else }}mail / notification{{ end }}</span>
</td>
</tr>
</table>
</td>
</tr>

<!-- Email body -->
<tr>
<td class="body" width="100%" style="width:100%; margin:0; padding:0;">
<table class="inner-body{{ if eq .LayoutVariant "incident" }} incident-body{{ end }}" align="center" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#ffffff" style="width:100%; max-width:600px; margin:0 auto; padding:0; table-layout:fixed; background-color:#ffffff; {{ if eq .LayoutVariant "incident" }}border:1px solid #27272a; border-radius:0;{{ else }}border:1px solid #e4e4e7; border-radius:8px; overflow:hidden;{{ end }}">
{{ if ne .LayoutVariant "incident" }}
<tr>
<td style="height:3px; background-color:#18181b; font-size:0; line-height:0;">&nbsp;</td>
</tr>
{{ end }}
<tr>
{{ if eq .LayoutVariant "incident" }}<td width="6" bgcolor="#ef4444" style="width:6px; padding:0; background-color:#ef4444; font-size:0; line-height:0;">&nbsp;</td>{{ end }}
<td class="content-cell{{ if eq .LayoutVariant "incident" }} incident-content{{ end }}" style="max-width:100vw; {{ if eq .LayoutVariant "incident" }}padding:0;{{ else }}padding:40px; overflow:hidden;{{ end }}">
{{ if .Greeting }}<h1>{{ .Greeting }}</h1>{{ end }}

{{ range .Intros }}
{{ . }}
{{ end }}

{{ if .Content }}{{ .Content }}{{ end }}

{{ range .Actions }}
<table class="action" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; margin:28px 0 0; padding:0;">
<tr>
<td align="left">
<table border="0" cellpadding="0" cellspacing="0" role="presentation">
<tr>
<td>
<a href="{{ .URL }}" class="button button-{{ .Color }}" target="_blank" rel="noopener" style="display:inline-block; padding:11px 17px; border-radius:6px; {{ if eq .Color "error" }}background-color:#ef4444;{{ else if eq .Color "success" }}background-color:#16a34a;{{ else }}background-color:#18181b;{{ end }} color:#ffffff; font-size:14px; font-weight:600; line-height:20px; text-decoration:none; -webkit-text-size-adjust:none;">{{ .Text }}</a>
</td>
</tr>
</table>
</td>
</tr>
</table>
{{ end }}

{{ range .Panels }}
<table class="panel" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; margin:20px 0; border:1px solid #e4e4e7; border-radius:6px; background-color:#fafafa;">
<tr>
<td class="panel-content" style="padding:18px; color:#52525b;">
{{ . }}
</td>
</tr>
</table>
{{ end }}

{{ range .Tables }}
<table class="data-table" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; margin:24px 0; border-collapse:collapse;">
<thead>
<tr>
{{ range .Headers }}<th>{{ . }}</th>{{ end }}
</tr>
</thead>
<tbody>
{{ range .Rows }}
<tr>
{{ range . }}<td>{{ . }}</td>{{ end }}
</tr>
{{ end }}
</tbody>
</table>
{{ end }}

{{ range .Outros }}
{{ . }}
{{ end }}

{{ if .Subcopy }}
<table class="subcopy" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; margin-top:28px; border-top:1px solid #e4e4e7;">
<tr>
<td style="padding-top:20px;">
<p class="sub" style="margin:0; color:#a1a1aa; font-size:12px; line-height:18px;">{{ .Subcopy }}</p>
</td>
</tr>
</table>
{{ end }}
</td>
</tr>
</table>
</td>
</tr>

<!-- Footer -->
<tr>
<td style="padding:20px 12px 0;">
<table class="footer" align="center" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; max-width:600px; margin:0 auto; text-align:center;">
<tr>
<td align="center">
<p style="margin:0; color:#a1a1aa; font-size:11px; line-height:18px; text-align:center;">{{ if .FooterText }}{{ .FooterText }}{{ else }}Sent by {{ .BrandName }} &middot; &copy; ` + currentYear() + ` {{ .AppName }}{{ end }}</p>
</td>
</tr>
</table>
</td>
</tr>

</table>
<!--[if mso]>
</td></tr></table>
<![endif]-->
</td>
</tr>
</table>
</body>
</html>`

func currentYear() string {
	return time.Now().Format("2006")
}

// emailCSS enhances clients that retain head styles; critical layout styles are
// duplicated inline so the message remains readable when head CSS is stripped.
const emailCSS = `
body,
table,
td,
a,
p,
span,
h1,
h2,
h3 {
    box-sizing: border-box;
    font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
}

body {
    margin: 0;
    padding: 0;
    width: 100% !important;
    background-color: #f4f4f5;
    color: #3f3f46;
    -webkit-text-size-adjust: none;
}

table {
    border-collapse: separate;
}

p,
ul,
ol,
blockquote {
    margin-top: 0;
    color: #52525b;
    font-size: 15px;
    line-height: 1.65;
    text-align: left;
}

p {
    margin-bottom: 16px;
}

h1 {
    margin: 0 0 16px;
    color: #18181b;
    font-size: 26px;
    font-weight: 700;
    letter-spacing: -0.035em;
    line-height: 1.25;
    text-align: left;
}

h2 {
    margin: 0 0 10px;
    color: #27272a;
    font-size: 15px;
    font-weight: 700;
    letter-spacing: -0.01em;
    line-height: 1.4;
    text-align: left;
}

h3 {
    margin: 0 0 8px;
    color: #27272a;
    font-size: 13px;
    font-weight: 700;
    line-height: 1.4;
    text-align: left;
}

a {
    color: #18181b;
}

strong {
    color: #27272a;
    font-weight: 600;
}

code {
    border-radius: 4px;
    background-color: #f4f4f5;
    color: #27272a;
    font-family: 'JetBrains Mono', 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
    font-size: 88%;
    padding: 2px 5px;
}

pre {
    box-sizing: border-box;
    width: 100%;
    max-width: 100%;
    margin: 0;
    padding: 18px;
    overflow: hidden;
    border: 1px solid #27272a;
    border-radius: 6px;
    background-color: #09090b;
    color: #d4d4d8;
    font-family: 'JetBrains Mono', 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
    font-size: 12px;
    line-height: 1.65;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
}

pre code {
    display: block;
    padding: 0;
    border-radius: 0;
    background: transparent;
    color: inherit;
    font-size: inherit;
    line-height: inherit;
    white-space: inherit;
}

.inner-body a:not(.button) {
    overflow-wrap: anywhere;
}

.panel-content p:last-child {
    margin-bottom: 0;
}

.data-table th {
    padding: 0 0 9px;
    border-bottom: 1px solid #e4e4e7;
    color: #71717a;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-align: left;
    text-transform: uppercase;
}

.data-table td {
    padding: 11px 0;
    border-bottom: 1px solid #f4f4f5;
    color: #52525b;
    font-size: 14px;
    line-height: 20px;
}

@media only screen and (max-width: 620px) {
    .wrapper-cell {
        padding: 18px 8px !important;
    }

    .content-cell {
        padding: 28px 22px !important;
    }

    .incident-content {
        padding: 0 !important;
    }

    .incident-pad {
        padding-left: 22px !important;
        padding-right: 22px !important;
    }

    .incident-hero .incident-pad {
        padding-top: 32px !important;
    }

    .trace-heading-pad {
        padding-left: 22px !important;
        padding-right: 22px !important;
    }

    .trace-code {
        padding-right: 16px !important;
    }

    .inner-body,
    .email-header,
    .footer {
        width: 100% !important;
    }
}

@media only screen and (max-width: 420px) {
    h1 {
        font-size: 23px !important;
    }

    .incident-title {
        font-size: 27px !important;
        line-height: 34px !important;
    }

    .incident-route {
        font-size: 15px !important;
        line-height: 23px !important;
    }

    .button {
        display: block !important;
        text-align: center !important;
    }

    .detail-label {
        width: 100px !important;
    }

}
`
