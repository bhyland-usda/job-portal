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

    var main = document.getElementById('main-content');
    var scrollbarWidth = 0;
    if (main) {
      scrollbarWidth = Math.max(0, main.offsetWidth - main.clientWidth);
    }
    document.documentElement.style.setProperty('--app-scrollbar-width', scrollbarWidth + 'px');
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
