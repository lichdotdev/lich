// The landing page: the download button picks the visitor's system, the window in the hero switches views, and the
// install commands copy. Its words come from the page's own #strings, so every language shares this file.
var strings = JSON.parse(document.getElementById("strings").textContent);
var latest = fetch("https://api.github.com/repos/lichdotdev/lich/releases/latest")
  .then(function (r) { return r.ok ? r.json() : null; })
  .catch(function () { return null; });
latest.then(function (release) {
  if (!release) return;
  var v = document.getElementById("version");
  v.textContent = release.tag_name;
  v.hidden = false;
  document.querySelectorAll("[data-asset]").forEach(function (a) {
    var pattern = new RegExp(a.getAttribute("data-asset"));
    var file = release.assets.filter(function (x) { return pattern.test(x.name); })[0];
    if (file) a.href = file.browser_download_url;
  });
});

(function () {
  var ua = navigator.userAgentData && navigator.userAgentData.platform || navigator.userAgent || "";
  if (/android|iphone|ipad/i.test(ua)) return;
  var os = /win/i.test(ua) ? ["Windows", "windows"] : /mac/i.test(ua) ? ["macOS", "macos"] : /linux|x11/i.test(ua) ? ["Linux", "linux"] : null;
  if (!os) return;
  var a = document.getElementById("download");
  a.href = "#install-" + os[1];
  a.querySelector("span").textContent = strings.downloadFor.replace("{os}", os[0]);
  var others = [["macos", "Apple Silicon", "darwin-arm64\\.zip$"], ["macos", "Intel", "darwin-amd64\\.zip$"],
    ["windows", "Windows", "windows-amd64-setup\\.exe$"], ["linux", "Linux", ""]].filter(function (o) { return o[0] !== os[1]; });
  document.getElementById("also").innerHTML = strings.alsoFor + " " + others.map(function (o) {
    return '<a href="#install-' + o[0] + '"' + (o[2] ? ' data-asset="' + o[2] + '"' : "") + ">" + o[1] + "</a>";
  }).join(", ") + ' · <a href="#install">' + strings.otherWays + "</a>";
  document.getElementById("os-" + os[1]).checked = true;
  // Linux has no single file to hand over (.deb, .rpm or .pkg.tar.zst depends on the distribution), so its button
  // stays on the install section. A Mac reports its CPU only to Chromium; the rest get Apple Silicon.
  var arch = os[1] === "macos" && navigator.userAgentData && navigator.userAgentData.getHighEntropyValues
    ? navigator.userAgentData.getHighEntropyValues(["architecture"]).then(function (v) { return v.architecture; })
    : Promise.resolve("");
  var asset = { windows: function () { return /windows-amd64-setup\.exe$/; },
                macos: function (cpu) { return cpu === "x86" ? /darwin-amd64\.zip$/ : /darwin-arm64\.zip$/; } }[os[1]];
  if (!asset) return;
  Promise.all([latest, arch]).then(function (got) {
    var pattern = asset(got[1]);
    var file = got[0] && got[0].assets.filter(function (x) { return pattern.test(x.name); })[0];
    if (file) a.href = file.browser_download_url;
  });
})();

// The window: a sidebar row or the GUI | TUI switch puts its view on the stage. The first session remembers which
// of its two views it was left on, as the app does.
(function () {
  var win = document.getElementById("win");
  var rows = win.querySelectorAll(".w-side [data-go]");
  var first = "chat";
  function show(view, row) {
    win.setAttribute("data-view", view);
    if (view === "chat" || view === "term") first = view;
    rows.forEach(function (r) { r.classList.toggle("on", r === row); });
  }
  rows.forEach(function (r) {
    r.addEventListener("click", function () {
      var view = r.getAttribute("data-go");
      show(view === "chat" ? first : view, r);
    });
  });
  win.querySelectorAll(".seg2 [data-go]").forEach(function (b) {
    b.addEventListener("click", function () { show(b.getAttribute("data-go"), rows[0]); });
  });
})();

document.querySelectorAll(".copy").forEach(function (b) {
  b.addEventListener("click", function () {
    navigator.clipboard.writeText(b.previousElementSibling.textContent).then(function () {
      b.classList.add("done");
      setTimeout(function () { b.classList.remove("done"); }, 1500);
    });
  });
});
