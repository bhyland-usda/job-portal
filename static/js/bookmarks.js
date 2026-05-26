// Bookmark - silent save without page navigation
document.addEventListener('click', function(e) {
  var btn = e.target.closest('.bookmark-btn');
  if (!btn) return;
  e.preventDefault();

  // Already saved
  if (btn.dataset.saved === 'true') return;

  var body = 'target_type=' + encodeURIComponent(btn.dataset.type)
           + '&target_id=' + encodeURIComponent(btn.dataset.id);

  fetch('/bookmarks/add', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: body
  }).then(function(resp) {
    if (resp.ok || resp.redirected) {
      btn.dataset.saved = 'true';
      btn.classList.add('bookmark-saved');
      btn.innerHTML = '&#10003; Saved';
      btn.title = 'Saved to your bookmarks';
      btn.setAttribute('aria-label', 'Saved to your bookmarks');
    }
  }).catch(function() {});
});
