// Renders catalog.json (extension.Catalog in extension/catalog.go), so a newly listed extension shows here without
// editing this page. #<id> opens that extension's details over the list, and Back closes them. Each page in another
// language carries its words in #grid's data-strings and reads an entry's name, description and categories in its
// data-locale, falling back to English.
(function () {
  var CATALOG = "https://raw.githubusercontent.com/lichdotdev/extensions/main/catalog.json";
  // A glyph is a Lucide name (docs/extensions.md, the manifest's surface icon); lobe: and devicon: names fall back to
  // the extension's initial.
  var LUCIDE = "https://cdn.jsdelivr.net/npm/lucide-static@0.460.0/icons/";
  var PLATFORMS = [
    ["linux_amd64", "Linux x64"], ["linux_arm64", "Linux ARM"],
    ["darwin_arm64", "macOS Apple Silicon"], ["darwin_amd64", "macOS Intel"],
    ["windows_amd64", "Windows x64"], ["windows_arm64", "Windows ARM"]
  ];
  var STAR = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"/></svg>';
  var CHECK = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3.85 8.62a4 4 0 0 1 4.78-4.77 4 4 0 0 1 6.74 0 4 4 0 0 1 4.78 4.78 4 4 0 0 1 0 6.74 4 4 0 0 1-4.77 4.78 4 4 0 0 1-6.75 0 4 4 0 0 1-4.78-4.77 4 4 0 0 1 0-6.76Z"/><path d="m9 12 2 2 4-4"/></svg>';

  var grid = document.getElementById("grid");
  var cats = document.getElementById("cats");
  var query = document.getElementById("q");
  var sortMenu = document.getElementById("sort");
  var dialog = document.getElementById("detail");
  var body = document.getElementById("xd-body");
  var bar = document.querySelector(".xc-bar");
  var locale = grid.getAttribute("data-locale") || "en";
  var t = JSON.parse(grid.getAttribute("data-strings"));
  var plural = new Intl.PluralRules(locale);

  var all = [], names = {}, byId = {};
  var state = { cat: "", q: "", sort: "updated" };

  function escape(text) {
    return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }
  // inLocale reads a map keyed by language tag, whose case the catalog does not fix (docs/extensions.md, Translating
  // an extension).
  function inLocale(map) {
    var want = locale.toLowerCase();
    for (var tag in map || {}) if (tag.toLowerCase() === want) return map[tag];
  }
  function date(iso) {
    return new Date(iso).toLocaleDateString(locale, { year: "numeric", month: "long", day: "numeric", timeZone: "UTC" });
  }
  function tile(e) {
    if (e.icon) return '<img class="xc-pic" src="' + escape(e.icon) + '" alt="" />';
    if (e.glyph && e.glyph.indexOf(":") < 0) return '<i class="xc-glyph" style="--g: url(\'' + LUCIDE + escape(e.glyph) + '.svg\')"></i>';
    return '<b class="xc-letter">' + escape(e.name.charAt(0)) + "</b>";
  }
  function byline(e) {
    var official = e.official ? '<span class="xc-off">' + CHECK + escape(t.official) + "</span>" : "";
    return '<span class="xc-by">' + escape(e.publisher) + official + "</span>";
  }
  function stars(n) {
    return (t.stars[plural.select(n)] || t.stars.other).replace("{n}", n);
  }

  function card(e) {
    var top = e.cover ? '<img src="' + escape(e.cover) + '" alt="" loading="lazy" />' : tile(e);
    return '<a class="feat xc" href="#' + escape(e.id) + '"><div class="xc-top">' + top + '</div><div class="txt">' +
      "<h3>" + escape(e.name) + "</h3>" + byline(e) + "<p>" + escape(e.description) + "</p>" +
      '<div class="xc-meta"><span>' + e.categories.map(function (c) { return escape(names[c] || c); }).join(", ") + "</span>" +
      '<span class="xc-stars" aria-label="' + escape(stars(e.stars)) + '">' + STAR + e.stars + "</span></div></div></a>";
  }

  var ORDER = {
    updated: function (a, b) { return b.released.localeCompare(a.released); },
    stars: function (a, b) { return b.stars - a.stars || a.name.localeCompare(b.name, locale); },
    name: function (a, b) { return a.name.localeCompare(b.name, locale); }
  };
  function matches(e) {
    if (state.cat && e.categories.indexOf(state.cat) < 0) return false;
    if (!state.q) return true;
    return [e.name, e.description, e.publisher, e.id].join(" ").toLowerCase().indexOf(state.q) >= 0;
  }
  function render() {
    if (!all.length) { bar.style.display = "none"; grid.innerHTML = '<p class="cl-loading">' + escape(t.empty) + "</p>"; return; }
    var shown = all.filter(matches).sort(ORDER[state.sort]);
    grid.innerHTML = shown.length ? shown.map(card).join("") : '<p class="cl-loading">' + escape(t.noMatch) + "</p>";
  }

  function chips(categories) {
    var count = function (id) { return all.filter(function (e) { return e.categories.indexOf(id) >= 0; }).length; };
    var chip = function (id, label, n) {
      return '<button type="button" value="' + escape(id) + '" aria-pressed="' + (state.cat === id) + '">' + escape(label) + " <i>" + n + "</i></button>";
    };
    cats.innerHTML = chip("", t.all, all.length) + categories.map(function (c) { return chip(c.id, names[c.id], count(c.id)); }).join("");
  }
  cats.addEventListener("click", function (ev) {
    var b = ev.target.closest("button");
    if (!b) return;
    state.cat = b.value;
    cats.querySelectorAll("button").forEach(function (x) { x.setAttribute("aria-pressed", String(x === b)); });
    render();
  });
  query.addEventListener("input", function () { state.q = query.value.trim().toLowerCase(); render(); });
  sortMenu.querySelectorAll("button").forEach(function (b) {
    b.addEventListener("click", function () {
      state.sort = b.value;
      sortMenu.querySelector("summary").textContent = b.textContent;
      sortMenu.querySelectorAll("button").forEach(function (x) { x.setAttribute("aria-pressed", String(x === b)); });
      sortMenu.open = false;
      render();
    });
  });

  function detail(e) {
    var shots = e.screenshots && e.screenshots.length ? e.screenshots : [];
    var media = shots.length ? '<div class="xd-shots">' + shots.map(function (s) {
      return '<img src="' + escape(s.url) + '" alt="' + escape(s.alt) + '" />';
    }).join("") + "</div>" : "";
    var perms = (e.permissions || []).map(function (p) { return "<li>" + escape(t.permission[p] || p) + "</li>"; }).join("");
    var builds = e.builds || {};
    var plats = PLATFORMS.map(function (p) {
      return builds[p[0]] ? '<span class="cap-y">' + p[1] + "</span>" : '<span class="cap-n" title="' + escape(t.noBuild) + '">' + p[1] + "</span>";
    }).join("");
    return '<div class="xd-head"><div class="xc-top xd-icon">' + tile(e) + "</div><div>" +
      '<h2 id="xd-name">' + escape(e.name) + "</h2>" + byline(e) + "</div></div>" +
      '<p class="xd-desc">' + escape(e.description) + "</p>" + media +
      '<dl class="xd-facts"><div><dt>' + escape(t.version) + "</dt><dd>" + escape(e.version) + "</dd></div>" +
      "<div><dt>" + escape(t.released) + "</dt><dd>" + date(e.released) + "</dd></div>" +
      (e.license ? "<div><dt>" + escape(t.license) + "</dt><dd>" + escape(e.license) + "</dd></div>" : "") + "</dl>" +
      '<h3 class="rel-group">' + escape(t.permissions) + '</h3><p class="xd-p">' + escape(t.runsAsYou) + "</p>" +
      (perms ? '<p class="xd-p">' + escape(t.canAlso) + '</p><ul class="xd-perms">' + perms + "</ul>" : "") +
      '<h3 class="rel-group">' + escape(t.platforms) + '</h3><div class="caps xd-plats">' + plats + "</div>" +
      '<div class="xd-install"><button class="btn primary small" type="button" data-repo="' + escape(e.repo) + '">' + escape(t.copy) + "</button>" +
      "<span>" + escape(t.then) + "</span>" +
      '<a class="rel-more" href="' + escape(e.repo) + '#readme">' + escape(t.readme) + " →</a></div>";
  }
  body.addEventListener("click", function (ev) {
    var b = ev.target.closest("[data-repo]");
    if (!b) return;
    navigator.clipboard.writeText(b.getAttribute("data-repo")).then(function () {
      b.textContent = t.copied;
      setTimeout(function () { b.textContent = t.copy; }, 1600);
    });
  });

  // pushed is whether this page put #<id> on the history, so closing goes Back to the list instead of off the site.
  var pushed = false, opener = null;
  function sync() {
    var e = byId[decodeURIComponent(location.hash.slice(1))];
    if (!e) { if (dialog.open) dialog.close(); return; }
    body.innerHTML = detail(e);
    body.scrollTop = 0;
    opener = grid.querySelector('a[href="#' + CSS.escape(e.id) + '"]');
    if (!dialog.open) dialog.showModal();
  }
  grid.addEventListener("click", function (ev) { if (ev.target.closest("a.xc")) pushed = true; });
  window.addEventListener("hashchange", sync);
  dialog.querySelector(".xd-x").addEventListener("click", function () { dialog.close(); });
  dialog.addEventListener("click", function (ev) { if (ev.target === dialog) dialog.close(); });
  dialog.addEventListener("close", function () {
    if (byId[decodeURIComponent(location.hash.slice(1))]) {
      if (pushed) history.back();
      else history.replaceState(null, "", location.pathname + location.search);
    }
    pushed = false;
    if (opener) opener.focus();
  });

  fetch(CATALOG)
    .then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); })
    .then(function (c) {
      c.categories.forEach(function (k) { names[k.id] = inLocale(k.names) || k.names.en; });
      all = c.extensions.map(function (e) {
        var own = inLocale(e.translations) || {};
        return Object.assign({}, e, { name: own.name || e.name, description: own.description || e.description });
      });
      all.forEach(function (e) { byId[e.id] = e; });
      chips(c.categories);
      render();
      sync();
    })
    .catch(function () { bar.style.display = "none"; document.getElementById("fallback").hidden = false; grid.innerHTML = ""; });
})();
