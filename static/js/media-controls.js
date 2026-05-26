// Media controls — video autoplay user setting + GIF play/pause overlay
document.addEventListener('DOMContentLoaded', function () {
  var AUTOPLAY_KEY = 'video-autoplay';

  // ========================
  // Video autoplay setting
  // ========================

  function isAutoplayEnabled() {
    return localStorage.getItem(AUTOPLAY_KEY) === 'true';
  }

  function applyAutoplay() {
    var videos = document.querySelectorAll('video[data-media="video"]');
    for (var i = 0; i < videos.length; i++) {
      if (isAutoplayEnabled()) {
        videos[i].muted = true;
        videos[i].loop = true;
        videos[i].playsInline = true;
        videos[i].play().catch(function () {});
      } else {
        videos[i].muted = false;
        videos[i].loop = false;
        videos[i].pause();
      }
    }
  }

  // Toggle button handler
  var toggle = document.getElementById('video-autoplay-toggle');
  if (toggle) {
    toggle.checked = isAutoplayEnabled();
    toggle.addEventListener('change', function () {
      localStorage.setItem(AUTOPLAY_KEY, toggle.checked ? 'true' : 'false');
      applyAutoplay();
    });
  }

  // Apply on load
  applyAutoplay();

  // Re-apply when new content is added (feed updates, chat messages)
  var observer = new MutationObserver(function (mutations) {
    var hasNew = false;
    for (var i = 0; i < mutations.length; i++) {
      if (mutations[i].addedNodes.length > 0) {
        hasNew = true;
        break;
      }
    }
    if (hasNew) {
      applyAutoplay();
      initGifOverlays();
    }
  });

  var feedContainer = document.querySelector('.feed-layout');
  if (feedContainer) {
    observer.observe(feedContainer, { childList: true, subtree: true });
  }
  var chatMessages = document.getElementById('chat-conv-messages');
  if (chatMessages) {
    observer.observe(chatMessages, { childList: true, subtree: true });
  }

  // ========================
  // GIF play/pause overlay
  // ========================

  function initGifOverlays() {
    var gifs = document.querySelectorAll('[data-media="gif"]');
    for (var i = 0; i < gifs.length; i++) {
      var img = gifs[i];
      if (img.dataset.gifInit) continue;
      img.dataset.gifInit = 'true';

      // Capture first frame once loaded
      if (img.complete) {
        setupGif(img);
      } else {
        img.addEventListener('load', function () {
          setupGif(this);
        });
      }
    }
  }

  function setupGif(img) {
    var container = img.closest('.gif-container');
    if (!container) return;

    // Create canvas with first frame
    var canvas = document.createElement('canvas');
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    canvas.className = 'gif-still';
    var ctx = canvas.getContext('2d');
    ctx.drawImage(img, 0, 0);

    // Insert canvas, hide the animated img by default
    container.insertBefore(canvas, img);
    img.classList.add('gif-hidden');

    // Default state: paused (show canvas + overlay)
    container.dataset.playing = 'false';

    container.addEventListener('click', function () {
      var playing = this.dataset.playing === 'true';
      var gifImg = this.querySelector('[data-media="gif"]');
      var still = this.querySelector('.gif-still');
      var overlay = this.querySelector('.gif-overlay');

      if (playing) {
        // Pause: show still frame
        if (still) still.style.display = '';
        if (gifImg) gifImg.classList.add('gif-hidden');
        if (overlay) {
          overlay.textContent = 'GIF';
          overlay.classList.remove('gif-playing');
        }
        this.dataset.playing = 'false';
      } else {
        // Play: show animated gif
        if (still) still.style.display = 'none';
        if (gifImg) gifImg.classList.remove('gif-hidden');
        if (overlay) {
          overlay.textContent = 'II';
          overlay.classList.add('gif-playing');
        }
        this.dataset.playing = 'true';
      }
    });
  }

  initGifOverlays();

  // Expose for chat drawer to call after rendering messages
  window.initMediaControls = function () {
    applyAutoplay();
    initGifOverlays();
  };
});
