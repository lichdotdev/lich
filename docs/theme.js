// The header's theme menu, on every page of the site. Loaded in <head> so a saved choice applies before the first
// paint; "system" removes data-theme and leaves the page to prefers-color-scheme.
(function () {
  var root = document.documentElement;
  function apply(theme) {
    if (theme === "light" || theme === "dark") root.setAttribute("data-theme", theme);
    else root.removeAttribute("data-theme");
    document.querySelectorAll(".theme button").forEach(function (b) {
      b.setAttribute("aria-pressed", String(b.value === (theme || "system")));
    });
  }
  var saved = null;
  try { saved = localStorage.getItem("theme"); } catch (e) {}
  apply(saved);
  document.addEventListener("DOMContentLoaded", function () {
    apply(saved);
    document.querySelectorAll(".theme button").forEach(function (b) {
      b.addEventListener("click", function () {
        try { localStorage.setItem("theme", b.value); } catch (e) {}
        apply(b.value);
        b.closest("details").open = false;
      });
    });
  });
})();
