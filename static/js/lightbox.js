// Attachment lightbox - click static image to view at full size.
// Uses delegated click so SSE-added posts work without re-init.
(function () {
  var overlay = document.getElementById('attachment-lightbox');
  if (!overlay) return;

  var img = overlay.querySelector('.lightbox-image');
  var caption = overlay.querySelector('.lightbox-caption');
  var closeBtn = overlay.querySelector('.lightbox-close');

  function open(src, alt) {
    img.src = src;
    img.alt = alt || '';
    caption.textContent = alt || '';
    overlay.classList.add('active');
    document.body.style.overflow = 'hidden';
  }

  function close() {
    overlay.classList.remove('active');
    document.body.style.overflow = '';
    img.src = '';
  }

  document.addEventListener('click', function (evt) {
    var target = evt.target.closest('img[data-media="image"]');
    if (!target) return;

    evt.preventDefault();
    open(target.src, target.alt);
  });

  closeBtn.addEventListener('click', close);

  overlay.addEventListener('click', function (evt) {
    if (evt.target === overlay) close();
  });

  document.addEventListener('keydown', function (evt) {
    if (evt.key === 'Escape' && overlay.classList.contains('active')) close();
  });
})();
