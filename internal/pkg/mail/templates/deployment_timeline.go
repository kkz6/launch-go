package templates

import "html/template"

// deploymentTimelineContentTemplate is the operational, evidence-rich form of
// the shared timeline. It uses tables and inline styles so the sequence remains
// intact in Outlook and in clients that remove the head stylesheet.
var deploymentTimelineContentTemplate = template.Must(template.New("deployment-timeline-content").Parse(`
<table class="incident-timeline" width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; table-layout:fixed;">
<tr>
<td class="incident-rail" width="52" valign="top" style="width:52px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:43px; font-size:0; line-height:0;"><span style="display:inline-block; width:10px; height:10px; margin-right:-6px; border:2px solid #ffffff; background-color:#ef4444; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td class="incident-pad" valign="top" style="padding:38px 36px 24px 28px;">
<p style="margin:0 0 8px; color:#b91c1c; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; font-weight:700; letter-spacing:0.08em; line-height:16px; text-transform:uppercase;">&#10007; {{ .StatusLabel }}</p>
<h1 class="incident-title" style="margin:0; color:#09090b; font-family:'Plus Jakarta Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif; font-size:31px; font-weight:700; letter-spacing:-0.045em; line-height:39px;">{{ .StatusHeading }}</h1>
<p class="incident-route" style="margin:11px 0 0; color:#27272a; font-size:15px; font-weight:650; line-height:23px; overflow-wrap:anywhere;">{{ .SiteAddress }}{{ if .HasServer }} <span style="color:#a1a1aa; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-weight:400;">&#8594;</span> {{ .ServerName }}{{ end }}</p>
</td>
</tr>

{{ if .FailureSummary }}
<tr>
<td class="incident-rail" width="52" valign="top" style="width:52px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:19px; font-size:0; line-height:0;"><span style="display:inline-block; width:6px; height:6px; margin-right:-4px; border:1px solid #ffffff; background-color:#ef4444; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td class="incident-pad" valign="top" style="padding:0 36px 24px 28px;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fff7f7" style="width:100%; border-top:1px solid #ef4444; border-bottom:1px solid #fecaca; background-color:#fff7f7;"><tr><td style="padding:14px 16px;">
<p style="margin:0 0 5px; color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; font-weight:700; letter-spacing:0.07em; line-height:15px; text-transform:uppercase;">Current failure</p>
<p style="margin:0; color:#991b1b; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:12px; font-weight:600; line-height:19px; overflow-wrap:anywhere; word-break:break-word;">{{ .FailureSummary }}</p>
</td></tr></table>
</td>
</tr>
{{ end }}

<tr>
<td class="incident-rail" width="52" valign="top" style="width:52px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:16px; font-size:0; line-height:0;"><span style="display:inline-block; width:5px; height:5px; margin-right:-3px; border:1px solid #71717a; background-color:#ffffff; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td class="incident-pad" valign="top" style="padding:0 36px 25px 28px;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation" style="width:100%; border-top:1px solid #27272a;"><tr><td style="padding:13px 0 0;">
<p style="margin:0 0 9px; color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; font-weight:700; letter-spacing:0.07em; line-height:15px; text-transform:uppercase;">Run context</p>
{{ if .HasCommit }}<p style="margin:0; color:#27272a; font-size:14px; font-weight:650; line-height:22px; overflow-wrap:anywhere;">{{ if .ShortGitHash }}<span style="color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:11px; font-weight:500;">{{ .ShortGitHash }}</span>{{ end }}{{ if and .ShortGitHash .CommitMessage }} <span style="color:#d4d4d8;">/</span> {{ end }}{{ .CommitMessage }}</p>{{ end }}
<p style="margin:{{ if .HasCommit }}8px{{ else }}0{{ end }} 0 0; color:#71717a; font-size:11px; line-height:18px;">{{ range .RunMeta }}<span style="display:block; overflow-wrap:anywhere; word-break:break-word;">{{ .Label }} <strong style="color:#3f3f46; font-weight:650;">{{ .Value }}</strong></span>{{ end }}</p>
</td></tr></table>
</td>
</tr>

{{ if .ActionURL }}
<tr>
<td class="incident-rail" width="52" valign="top" style="width:52px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:14px; font-size:0; line-height:0;"><span style="display:inline-block; width:7px; height:7px; margin-right:-4px; border:1px solid #ffffff; background-color:#18181b; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td class="incident-pad" valign="top" style="padding:0 36px 30px 28px;">
<table border="0" cellpadding="0" cellspacing="0" role="presentation"><tr><td bgcolor="#18181b" style="background-color:#18181b; mso-padding-alt:12px 16px;"><a href="{{ .ActionURL }}" target="_blank" rel="noopener" style="display:inline-block; border:12px solid #18181b; background-color:#18181b; color:#ffffff; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:12px; font-weight:650; line-height:16px; text-decoration:none;">{{ .ActionText }} <span style="color:#a1a1aa;">&#8594;</span></a></td></tr></table>
</td>
</tr>
{{ end }}

{{ if .OutputLines }}
<tr>
<td class="incident-rail" width="52" valign="top" style="width:52px; border-right:1px solid #d4d4d8; padding:0;">
<table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="right" style="padding-top:17px; font-size:0; line-height:0;"><span style="display:inline-block; width:5px; height:5px; margin-right:-3px; border:1px solid #71717a; background-color:#ffffff; font-size:0; line-height:0;">&nbsp;</span></td></tr></table>
</td>
<td valign="top" style="padding:0 0 30px 28px;">
<table class="trace-shell" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fafafa" style="width:100%; table-layout:fixed; border-top:1px solid #27272a; border-bottom:1px solid #e4e4e7; background-color:#fafafa;">
<tr><td class="trace-heading-pad" bgcolor="#ffffff" style="padding:13px 36px 11px 0; background-color:#ffffff;"><table width="100%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="left"><p style="margin:0; color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; font-weight:700; letter-spacing:0.07em; line-height:15px; text-transform:uppercase;">Execution output</p></td><td align="right"><span style="color:#71717a; font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; line-height:15px;">{{ .OutputLabel }}</span></td></tr></table></td></tr>
<tr><td bgcolor="#fafafa" style="padding:7px 0 11px; background-color:#fafafa;"><table class="deployment-trace" width="100%" cellpadding="0" cellspacing="0" role="presentation" bgcolor="#fafafa" style="width:100%; table-layout:fixed; background-color:#fafafa;">
{{ range .OutputLines }}<tr class="trace-line{{ if .IsFailure }} trace-error{{ else if .IsCleanup }} trace-cleanup{{ end }}">
<td width="22" valign="top" align="center" {{ if .IsFailure }}bgcolor="#fef2f2"{{ end }} style="width:22px; padding:3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#b91c1c;{{ else }}color:#d4d4d8;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; font-weight:700; line-height:17px;">{{ if .Marker }}{{ .Marker }}{{ else }}&nbsp;{{ end }}</td>
<td width="30" valign="top" align="right" {{ if .IsFailure }}bgcolor="#fef2f2"{{ end }} style="width:30px; padding:3px 7px 3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#b91c1c;{{ else }}color:#71717a;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:9px; line-height:17px;">{{ if .Number }}{{ .Number }}{{ else }}&nbsp;{{ end }}</td>
<td class="trace-code" valign="top" {{ if .IsFailure }}bgcolor="#fef2f2"{{ end }} style="max-width:0; padding:3px 24px 3px 0; {{ if .IsFailure }}background-color:#fef2f2; color:#991b1b; font-weight:600;{{ else if .IsCleanup }}color:#71717a;{{ else }}color:#3f3f46;{{ end }} font-family:'JetBrains Mono','SFMono-Regular',Consolas,'Courier New',monospace; font-size:10px; line-height:17px; overflow-wrap:anywhere; word-break:break-word; white-space:pre-wrap;">{{ .Text }}</td>
</tr>{{ end }}
</table></td></tr></table>
</td>
</tr>
{{ end }}
<tr><td width="52" style="width:52px; padding:0; font-size:0; line-height:0;">&nbsp;</td><td style="height:1px; padding:0; font-size:0; line-height:0;">&nbsp;</td></tr>
</table>
`))
