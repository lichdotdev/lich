// The header's GitHub star count, on every page of the site.
fetch("https://api.github.com/repos/lichdotdev/lich")
  .then(function (r) { return r.ok ? r.json() : null; })
  .then(function (repo) {
    if (!repo) return;
    var n = repo.stargazers_count;
    var label = n >= 1000 ? (n / 1000).toFixed(n >= 10000 ? 0 : 1).replace(/\.0$/, "") + "k" : String(n);
    var el = document.getElementById("nav-stars");
    el.textContent = label;
    el.hidden = false;
  });
