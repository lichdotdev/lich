// Renders the changelog from main, so this page never needs editing: each "## [X.Y.Z] - date" section becomes a
// release, its GitHub alert blocks become the headline and callouts, its "###" groups keep their order. A page in
// another language reads changelog/<locale>.md too and shows each release it translates in place of the English
// one; older releases, which only CHANGELOG.md holds, stay in English under a note that says so.
// The file only uses headings, "- " items with indented continuation lines, alert blocks and inline
// **bold**, `code` and [links](url), which is all this reads.
(function () {
  var RAW = "https://raw.githubusercontent.com/lichdotdev/lich/main/";
  var RELEASES = "https://github.com/lichdotdev/lich/releases/tag/v";
  var PAGE = 8;
  var list = document.getElementById("releases");
  var more = document.getElementById("older");
  var locale = list.getAttribute("data-locale") || "en";
  var groups = JSON.parse(list.getAttribute("data-groups") || "{}");
  var releaseLink = list.getAttribute("data-release") || "Release on GitHub";
  var englishNote = list.getAttribute("data-english");

  function escape(text) {
    return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function inline(text) {
    var codes = [];
    var out = escape(text).replace(/`([^`]+)`/g, function (_, c) { codes.push(c); return "\u0000" + (codes.length - 1) + "\u0000"; });
    out = out.replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>")
      .replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2">$1</a>');
    return out.replace(/\u0000(\d+)\u0000/g, function (_, i) { return "<code>" + codes[i] + "</code>"; });
  }

  // parse splits the file into releases, skipping [Unreleased] and the link references at the end.
  function parse(md) {
    var releases = [], release = null, group = null, item = null, alert = null;
    md.split("\n").forEach(function (line) {
      var head = /^## \[(\d[^\]]*)\](?: - (\d{4}-\d{2}-\d{2}))?/.exec(line);
      if (head) {
        release = { version: head[1], date: head[2], alerts: [], groups: [] };
        releases.push(release);
        group = item = alert = null;
        return;
      }
      if (/^## /.test(line)) { release = null; return; }
      if (!release || /^\[[^\]]+\]: /.test(line)) return;
      var kind = /^> \[!(\w+)\]/.exec(line);
      if (kind) { alert = { kind: kind[1].toLowerCase(), text: "" }; release.alerts.push(alert); return; }
      if (alert && /^>/.test(line)) { alert.text += " " + line.replace(/^>\s?/, ""); return; }
      alert = null;
      var g = /^### (.+)/.exec(line);
      if (g) { group = { name: g[1], items: [] }; release.groups.push(group); item = null; return; }
      if (!group) return;
      if (/^- /.test(line)) { item = line.slice(2); group.items.push(item); return; }
      if (item !== null && /^\s+\S/.test(line)) { group.items[group.items.length - 1] += " " + line.trim(); return; }
      if (!line.trim()) item = null;
    });
    return releases;
  }

  function date(iso) {
    if (!iso) return "";
    return new Date(iso + "T12:00:00Z").toLocaleDateString(locale, { year: "numeric", month: "long", day: "numeric", timeZone: "UTC" });
  }
  function render(r) {
    var important = r.alerts.filter(function (a) { return a.kind === "important"; })[0];
    var note = r.firstEnglish ? '<p class="rel-english">' + escape(englishNote) + "</p>" : "";
    var html = note + '<article class="rel"' + (r.english ? ' lang="en"' : "") + '><div class="rel-date">' + date(r.date) + "</div>" +
      '<a class="rel-ver" href="' + RELEASES + r.version + '">' + r.version + "</a>";
    if (important) html += '<p class="rel-lead">' + inline(important.text.trim()) + "</p>";
    r.alerts.forEach(function (a) {
      if (a !== important) html += '<p class="rel-note ' + a.kind + '">' + inline(a.text.trim()) + "</p>";
    });
    r.groups.forEach(function (g) {
      html += '<h3 class="rel-group">' + escape(groups[g.name] || g.name) + "</h3><ul>" +
        g.items.map(function (i) { return "<li>" + inline(i) + "</li>"; }).join("") + "</ul>";
    });
    return html + '<a class="rel-more" href="' + RELEASES + r.version + '">' + escape(releaseLink) + " →</a></article>";
  }

  var releases = [], shown = 0;
  function show() {
    list.insertAdjacentHTML("beforeend", releases.slice(shown, shown + PAGE).map(render).join(""));
    shown += PAGE;
    more.hidden = shown >= releases.length;
  }
  more.addEventListener("click", show);
  function read(path) {
    return fetch(RAW + path).then(function (r) { if (!r.ok) throw new Error(r.status); return r.text(); });
  }
  // A missing translation file only costs the translations: the English changelog still renders.
  var translated = locale === "en" ? Promise.resolve("") : read("changelog/" + locale + ".md").catch(function () { return ""; });
  Promise.all([read("CHANGELOG.md"), translated])
    .then(function (got) {
      var local = {};
      parse(got[1]).forEach(function (r) { local[r.version] = r; });
      var shownEnglish = false;
      releases = parse(got[0]).map(function (r) {
        if (local[r.version]) return local[r.version];
        r.english = locale !== "en";
        r.firstEnglish = r.english && !shownEnglish && Boolean(englishNote);
        if (r.english) shownEnglish = true;
        return r;
      });
      list.innerHTML = "";
      show();
    })
    .catch(function () { document.getElementById("fallback").hidden = false; list.innerHTML = ""; });
})();
