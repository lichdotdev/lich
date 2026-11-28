// The landing page: the download button picks the visitor's system, the window under the hero switches screens,
// and the install commands copy. Its words come from the page's own #strings, so every language shares this file.
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

// The window: each tab shows its screenshot, arrows move between tabs the way a tablist does.
(function () {
  var tabs = Array.prototype.slice.call(document.querySelectorAll(".gtabs [role=tab]"));
  function show(tab, focus) {
    tabs.forEach(function (t) {
      var on = t === tab;
      t.setAttribute("aria-selected", String(on));
      t.tabIndex = on ? 0 : -1;
      document.getElementById(t.getAttribute("aria-controls")).hidden = !on;
    });
    if (focus) tab.focus();
  }
  tabs.forEach(function (t, i) {
    t.tabIndex = i === 0 ? 0 : -1;
    t.addEventListener("click", function () { show(t); });
    t.addEventListener("keydown", function (e) {
      var step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
      if (!step) return;
      e.preventDefault();
      show(tabs[(i + step + tabs.length) % tabs.length], true);
    });
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
