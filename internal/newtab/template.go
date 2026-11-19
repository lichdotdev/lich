package newtab

import "html/template"

// pageTemplate is the page. html/template escapes every value for the
// context it lands in: titles and addresses come from pages the agent browsed,
// and names from files it wrote.
var pageTemplate = template.Must(template.New("newtab").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Text.Title}}</title>
<style>
:root{--bg:{{.Colors.Background}};--card:{{.Colors.Card}};--fg:{{.Colors.Foreground}};--muted:{{.Colors.Muted}};--border:{{.Colors.Border}};--accent:{{.Colors.Accent}};--pass:{{.Colors.Pass}};color-scheme:dark light}
*{box-sizing:border-box;margin:0}
body{background:var(--bg);color:var(--fg);font:14px/1.45 Onest,system-ui,sans-serif;padding:104px 16px 56px}
main{max-width:560px;margin:0 auto;display:flex;flex-direction:column;gap:26px}
form{display:flex}
input{flex:1;height:42px;border-radius:9px;background:var(--card);color:var(--fg);border:0;box-shadow:inset 0 0 0 1px var(--border);padding:0 14px;font:inherit;font-size:15px}
input::placeholder{color:var(--muted)}
input:focus{outline:0;box-shadow:inset 0 0 0 1px var(--muted)}
.err{color:var(--muted);font-size:12.5px;margin-top:-18px}
h2{font-size:10.5px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:var(--muted);margin-bottom:6px}
.row{min-height:34px;display:flex;align-items:center;gap:10px;padding:0 8px;border-radius:6px;font-size:13.5px;color:inherit;text-decoration:none;overflow:hidden}
a.row:hover,a.row:focus-visible{background:var(--accent);outline:0}
.row span{white-space:nowrap}
.row .name{overflow:hidden;text-overflow:ellipsis}
.row .u,.row .r{color:var(--muted);font-size:12px}
.row .u{font-family:ui-monospace,Menlo,monospace;overflow:hidden;text-overflow:ellipsis}
.row .r{margin-left:auto;padding-left:8px}
.dot{width:7px;height:7px;border-radius:50%;background:var(--pass);flex:none}
</style>
</head>
<body><main>
<form action="{{.Action}}" method="get"><input name="q" type="text" placeholder="{{.Text.Address}}" aria-label="{{.Text.Address}}" autocomplete="off" spellcheck="false" autofocus></form>
{{if .Error}}<p class="err" role="alert">{{.Text.AddressError}}</p>{{end}}
{{if or .Services .Worktree}}<section><h2>{{.Text.Running}}</h2>
{{range .Services}}<div class="row"><i class="dot"></i><span class="name">{{.Name}}</span><span class="u">{{.Command}}</span></div>
{{end}}{{if .Worktree}}<a class="row" href="{{.Worktree}}"><i class="dot"></i><span class="u">{{.WorktreeHost}}</span><span class="r">{{.Text.Worktree}}</span></a>
{{end}}</section>{{end}}
{{if .Artifacts}}<section><h2>{{.Text.Artifacts}}</h2>
{{range .Artifacts}}<a class="row" href="{{.URL}}"><span class="name">{{.Name}}</span><time class="r" datetime="{{.Stamp}}">{{.Day}}</time></a>
{{end}}</section>{{end}}
{{if .Recent}}<section><h2>{{.Text.Recent}}</h2>
{{range .Recent}}<a class="row" href="{{.URL}}"><span class="name">{{if .Title}}{{.Title}}{{else}}{{.Shown}}{{end}}</span>{{if .Title}}<span class="u">{{.Shown}}</span>{{end}}</a>
{{end}}</section>{{end}}
</main>
<script nonce="{{.Nonce}}">
const rtf=new Intl.RelativeTimeFormat(document.documentElement.lang,{numeric:"auto"});
const units=[["day",86400],["hour",3600],["minute",60]];
for(const t of document.querySelectorAll("time[datetime]")){
  const s=(Date.parse(t.dateTime)-Date.now())/1000;
  const [unit,size]=units.find(u=>Math.abs(s)>=u[1])||["minute",60];
  t.textContent=rtf.format(Math.round(s/size),unit);
}
</script>
</body>
</html>
`))
