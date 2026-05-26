// Dark mode — instant load before body renders
if (localStorage.getItem('dark') === 'true') {
  document.documentElement.classList.add('dark-mode');
}

// Bind toggle after DOM ready
document.addEventListener('DOMContentLoaded', function () {
  function syncShellOffset() {
    var header = document.querySelector('.site-header');
    var height = header ? header.offsetHeight : 0;
    document.documentElement.style.setProperty('--app-header-height', height + 'px');
  }

  syncShellOffset();
  window.addEventListener('load', syncShellOffset);
  window.addEventListener('resize', syncShellOffset);

  var toggle = document.getElementById('dark-mode-toggle');
  if (toggle) {
    toggle.addEventListener('click', function (e) {
      e.preventDefault();
      document.documentElement.classList.toggle('dark-mode');
      var isDark = document.documentElement.classList.contains('dark-mode');
      localStorage.setItem('dark', isDark);
      toggle.textContent = isDark ? 'Light Mode' : 'Dark Mode';
    });

    // Set initial label
    var isDark = document.documentElement.classList.contains('dark-mode');
    toggle.textContent = isDark ? 'Light Mode' : 'Dark Mode';
  }
});
