// Apply the theme before first paint to avoid a flash of the wrong theme.
// Explicit choice (localStorage) wins, else follow the OS.
(function () {
  try {
    var stored = localStorage.getItem("pp:theme");
    var dark = stored
      ? stored === "dark"
      : window.matchMedia("(prefers-color-scheme: dark)").matches;
    var el = document.documentElement;
    el.classList.toggle("dark", dark);
    el.style.colorScheme = dark ? "dark" : "light";
  } catch (e) {}
})();
