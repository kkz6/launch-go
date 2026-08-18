package templates

import "time"

// baseLayout is a continuous execution sheet shared by every transactional
// email. Critical layout rules are inline for clients that strip head styles.
var baseLayout = `<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .AppName }}</title>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<style>` + emailCSS + `</style>
</head>
<body style="margin:0; padding:0; width:100% !important; background-color:#f4f4f5; color:#3f3f46; -webkit-text-size-adjust:none;">
{{ .DirectionContract }}
{{ if .Preheader }}
<div class="preheader" style="display:none !important; max-height:0; max-width:0; opacity:0; overflow:hidden; mso-hide:all; color:transparent; line-height:1px; font-size:1px;">{{ .Preheader }}&#847;&zwnj;&nbsp;&#847;&zwnj;&nbsp;&#847;&zwnj;&nbsp;</div>
{{ end }}
<table class="wrapper" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#f4f4f5" style="width:100%; margin:0; padding:0; background-color:#f4f4f5;">
<tr><td class="wrapper-cell" align="center" style="padding:36px 12px;">
<!--[if mso]><table width="600" cellpadding="0" cellspacing="0" role="presentation"><tr><td><![endif]-->
<table class="sheet" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#ffffff" style="width:100%; max-width:600px; margin:0 auto; table-layout:fixed; background-color:#ffffff; border:1px solid #d4d4d8;">

<tr><td class="sheet-header" style="padding:22px 28px; border-bottom:1px solid #e4e4e7;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%;"><tr>
<td align="left"><a href="{{ .AppURL }}" class="brand" style="display:inline-block; color:#09090b; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif; font-size:20px; font-weight:800; letter-spacing:-0.045em; text-decoration:none;">{{ .BrandName }}</a></td>
<td align="right"><span class="header-route" style="color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; font-weight:600; letter-spacing:0.05em; line-height:16px; text-transform:uppercase;">{{ if eq .LayoutVariant "incident" }}lctl / deploy{{ else }}{{ .Context }}{{ end }}</span></td>
</tr></table>
</td></tr>

{{ if eq .LayoutVariant "incident" }}
<tr><td style="padding:0;">{{ .Content }}</td></tr>
{{ else }}
<tr><td class="timeline-pad" style="padding:36px 28px 10px;">
<table class="timeline" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; table-layout:fixed;">
<tr>
<td class="timeline-rail" width="28" valign="top" style="width:28px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:5px; font-size:0; line-height:0;"><span class="state-node state-{{ .StateTone }}" style="display:inline-block; width:9px; height:9px; margin-right:-5px; border:2px solid #ffffff; background-color:{{ if eq .StateTone "error" }}#ef4444{{ else if eq .StateTone "success" }}#16a34a{{ else if eq .StateTone "warning" }}#d97706{{ else }}#18181b{{ end }}; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td class="timeline-copy hero-copy" valign="top" style="padding:0 0 24px 24px;">
<p class="state-label state-label-{{ .StateTone }}" style="margin:0 0 8px; color:{{ if eq .StateTone "error" }}#b91c1c{{ else if eq .StateTone "success" }}#15803d{{ else if eq .StateTone "warning" }}#a16207{{ else }}#71717a{{ end }}; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; font-weight:700; letter-spacing:0.08em; line-height:16px; text-transform:uppercase;">{{ .StateLabel }}</p>
{{ if .Greeting }}<h1 style="margin:0; color:#09090b; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif; font-size:30px; font-weight:700; letter-spacing:-0.045em; line-height:38px;">{{ .Greeting }}</h1>{{ end }}
</td>
</tr>

{{ range .Blocks }}
<tr class="timeline-block timeline-{{ .Kind }}">
<td class="timeline-rail" width="28" valign="top" style="width:28px; border-right:1px solid #d4d4d8; padding:0;">
{{ if eq .Kind "action" }}<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:17px; font-size:0; line-height:0;"><span style="display:inline-block; width:7px; height:7px; margin-right:-4px; border:1px solid #ffffff; background-color:#18181b; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>{{ end }}
{{ if or (eq .Kind "panel") (eq .Kind "table") }}<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:15px; font-size:0; line-height:0;"><span style="display:inline-block; width:5px; height:5px; margin-right:-3px; border:1px solid #71717a; background-color:#ffffff; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>{{ end }}
</td>
<td class="timeline-copy" valign="top" style="padding:0 0 {{ if eq .Kind "action" }}26px{{ else if or (eq .Kind "panel") (eq .Kind "table") }}22px{{ else }}8px{{ end }} 24px; overflow:hidden;">
{{ if or (eq .Kind "intro") (eq .Kind "content") (eq .Kind "outro") }}<div class="prose prose-{{ .Kind }}">{{ .Content }}</div>{{ end }}
{{ if eq .Kind "action" }}
<table class="action" border="0" cellpadding="0" cellspacing="0" role="presentation"><tr><td bgcolor="{{ if eq .Action.Color "error" }}#b91c1c{{ else if eq .Action.Color "success" }}#15803d{{ else }}#18181b{{ end }}" style="background-color:{{ if eq .Action.Color "error" }}#b91c1c{{ else if eq .Action.Color "success" }}#15803d{{ else }}#18181b{{ end }}; mso-padding-alt:12px 16px;"><a href="{{ .Action.URL }}" class="button button-{{ .Action.Color }}" target="_blank" rel="noopener" style="display:inline-block; border:12px solid {{ if eq .Action.Color "error" }}#b91c1c{{ else if eq .Action.Color "success" }}#15803d{{ else }}#18181b{{ end }}; background-color:{{ if eq .Action.Color "error" }}#b91c1c{{ else if eq .Action.Color "success" }}#15803d{{ else }}#18181b{{ end }}; color:#ffffff; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:12px; font-weight:650; line-height:16px; text-decoration:none; -webkit-text-size-adjust:none;">{{ .Action.Text }} <span style="color:#d4d4d8;">&#8594;</span></a></td></tr></table>
{{ end }}
{{ if eq .Kind "panel" }}<table class="evidence" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fafafa" style="width:100%; border-top:1px solid #27272a; border-bottom:1px solid #e4e4e7; background-color:#fafafa;"><tr><td class="evidence-content" style="padding:16px 18px; color:#52525b;">{{ .Content }}</td></tr></table>{{ end }}
{{ if eq .Kind "table" }}<table class="data-table" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; border-collapse:collapse; border-top:1px solid #27272a;"><thead><tr>{{ range .Table.Headers }}<th>{{ . }}</th>{{ end }}</tr></thead><tbody>{{ range .Table.Rows }}<tr>{{ range . }}<td>{{ . }}</td>{{ end }}</tr>{{ end }}</tbody></table>{{ end }}
</td>
</tr>
{{ end }}

{{ if .Subcopy }}
<tr><td class="timeline-rail timeline-rail-end" width="28" valign="top" style="width:28px; border-right:1px solid #d4d4d8; padding:0;">&nbsp;</td><td class="timeline-copy" valign="top" style="padding:8px 0 28px 24px;"><p class="sub" style="margin:0; color:#71717a; font-size:11px; line-height:18px; overflow-wrap:anywhere;">{{ .Subcopy }}</p></td></tr>
{{ end }}
<tr><td width="28" style="width:28px; padding:0; font-size:0; line-height:0;">&nbsp;</td><td style="height:1px; padding:0; font-size:0; line-height:0;">&nbsp;</td></tr>
</table>
</td></tr>
{{ end }}

<tr><td class="sheet-footer" style="padding:18px 28px; border-top:1px solid #e4e4e7;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%;"><tr>
<td align="left"><p style="margin:0; color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; line-height:15px; text-transform:uppercase;">{{ if .FooterText }}{{ .FooterText }}{{ else }}transaction recorded / ` + currentYear() + `{{ end }}</p></td>
<td align="right"><p style="margin:0; color:#71717a; font-size:10px; line-height:15px;">{{ .BrandName }}</p></td>
</tr></table>
</td></tr>
</table>
<!--[if mso]></td></tr></table><![endif]-->
</td></tr></table>
</body>
</html>`

const directionContract = `<!--
THESIS: Transactional email is an execution timeline, not a stack of notification cards.
OWN-WORLD: White operational sheet, zinc ink, one-pixel lifecycle lines, square state nodes, Plus Jakarta Sans interface text, JetBrains Mono for identifiers and output, status color only.
STORY: The recipient sees what happened, which resource changed, and the next useful action before supporting evidence.
FIRST VIEWPORT: launchctl header, decisive event title, affected resource, current state and primary action; context follows on the same line.
FORM: Execution Timeline, candidate 4 in the grounded set, seed 47c85962.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
-->`

func currentYear() string {
	return time.Now().Format("2006")
}

const emailCSS = `
body, table, td, a, p, span, h1, h2, h3 { box-sizing:border-box; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif; }
body { margin:0; padding:0; width:100% !important; background:#f4f4f5; color:#3f3f46; -webkit-text-size-adjust:none; }
table { border-collapse:separate; }
p, ul, ol, blockquote { margin-top:0; color:#52525b; font-size:14px; line-height:1.65; text-align:left; }
p { margin-bottom:14px; }
.prose p:last-child, .evidence-content p:last-child { margin-bottom:0; }
h1 { margin:0; color:#09090b; font-size:30px; font-weight:700; letter-spacing:-0.045em; line-height:1.27; }
h2 { margin:0 0 9px; color:#27272a; font-size:14px; font-weight:700; line-height:1.45; }
h3 { margin:0 0 7px; color:#27272a; font-size:12px; font-weight:700; line-height:1.45; }
a { color:#18181b; }
strong { color:#27272a; font-weight:650; }
code { padding:2px 5px; background:#f4f4f5; color:#27272a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace; font-size:88%; }
pre { box-sizing:border-box; width:100%; max-width:100%; margin:0; padding:0; overflow:hidden; background:transparent; color:#3f3f46; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace; font-size:11px; line-height:1.7; white-space:pre-wrap; overflow-wrap:anywhere; word-break:break-word; }
pre code { display:block; padding:0; background:transparent; color:inherit; font-size:inherit; line-height:inherit; white-space:inherit; }
.sheet a:not(.button) { overflow-wrap:anywhere; }
.data-table th { padding:10px 8px 8px 0; border-bottom:1px solid #e4e4e7; color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,monospace; font-size:9px; font-weight:700; letter-spacing:.06em; text-align:left; text-transform:uppercase; }
.data-table td { padding:10px 8px 10px 0; border-bottom:1px solid #e4e4e7; color:#52525b; font-size:12px; line-height:18px; }
@media only screen and (max-width:620px) {
  .wrapper-cell { padding:12px 6px !important; }
  .sheet-header, .sheet-footer { padding-left:20px !important; padding-right:20px !important; }
  .timeline-pad { padding:28px 18px 8px !important; }
  .incident-pad { padding-left:18px !important; padding-right:18px !important; }
  .trace-heading-pad { padding-left:18px !important; padding-right:18px !important; }
  .trace-code { padding-right:14px !important; }
}
@media only screen and (max-width:420px) {
  .header-route { font-size:9px !important; }
  .hero-copy h1, .incident-title { font-size:25px !important; line-height:32px !important; }
  .timeline-copy { padding-left:18px !important; }
  .button { display:block !important; text-align:center !important; }
  .detail-label { width:88px !important; }
}
`
