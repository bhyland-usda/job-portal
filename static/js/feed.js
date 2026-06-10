// Feed - Per-post surgical updates via SSE
(function() {
  var feedLayout = document.querySelector('.feed-layout');
  if (!feedLayout) return;

  var evtSource = new EventSource('/feed/events');
  var expandedPosts = new Set();
  var attachmentInput = document.getElementById('attachment');
  var attachmentSelected = document.getElementById('attachment-selected');

  if (attachmentInput && attachmentSelected) {
    attachmentInput.addEventListener('change', function() {
      if (!attachmentInput.files || attachmentInput.files.length === 0) {
        attachmentSelected.textContent = '';
        return;
      }
      attachmentSelected.textContent = attachmentInput.files[0].name;
    });
  }

  function getPostId(postEl) {
    var match = postEl.innerHTML.match(/\/feed\/([0-9a-f-]+)\/like/);
    return match ? match[1] : null;
  }

  // Update a single post's stats and comments without touching interaction state
  function updateSinglePost(postId) {
    var tab = new URLSearchParams(window.location.search).get('tab') || 'social';
    fetch('/feed?tab=' + tab, { credentials: 'same-origin' })
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var parser = new DOMParser();
        var doc = parser.parseFromString(html, 'text/html');
        var newPost = doc.getElementById('post-' + postId);
        var oldPost = document.getElementById('post-' + postId);

        if (!newPost || !oldPost) return;

        // Update stats only
        var oldStats = oldPost.querySelector('.post-stats');
        var newStats = newPost.querySelector('.post-stats');
        if (oldStats && newStats && oldStats.innerHTML !== newStats.innerHTML) {
          oldStats.innerHTML = newStats.innerHTML;
        }

        // Append new comments only
        var oldComments = oldPost.querySelector('.post-comments');
        var newComments = newPost.querySelector('.post-comments');

        if (!oldComments && newComments) {
          var commentForm = oldPost.querySelector('.comment-form');
          if (commentForm) {
            var clone = newComments.cloneNode(true);
            commentForm.parentNode.insertBefore(clone, commentForm);
            oldComments = clone;
          }
        } else if (oldComments && newComments) {
          var oldCount = oldComments.querySelectorAll('.comment').length;
          var newCount = newComments.querySelectorAll('.comment').length;
          if (newCount > oldCount) {
            var els = newComments.querySelectorAll('.comment');
            for (var i = oldCount; i < newCount; i++) {
              oldComments.appendChild(els[i].cloneNode(true));
            }
          }
        }

        // Restore expanded state
        if (expandedPosts.has(postId) && oldComments) {
          oldComments.classList.add('expanded');
          var toggle = oldPost.querySelector('.comment-toggle');
          if (toggle) {
            toggle.textContent = 'Hide Comments';
            toggle.setAttribute('aria-expanded', 'true');
          }
        }
      });
  }

  // Full refresh - only for new posts or deletions
  function fullRefresh() {
    var tab = new URLSearchParams(window.location.search).get('tab') || 'social';

    // Don't refresh if user is typing
    if (feedLayout.querySelector('input:focus, textarea:focus')) return;

    fetch('/feed?tab=' + tab, { credentials: 'same-origin' })
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var parser = new DOMParser();
        var doc = parser.parseFromString(html, 'text/html');
        var newFeed = doc.querySelector('.feed-layout');
        if (!newFeed) return;

        if (tab === 'postings') {
          feedLayout.innerHTML = newFeed.innerHTML;
          return;
        }

        // Save expanded states
        feedLayout.querySelectorAll('.feed-post').forEach(function(p) {
          var pid = getPostId(p);
          if (pid && p.querySelector('.post-comments.expanded')) {
            expandedPosts.add(pid);
          }
        });

        var oldPosts = feedLayout.querySelectorAll('.feed-post');
        var newPosts = newFeed.querySelectorAll('.feed-post');

        var oldPostMap = {};
        oldPosts.forEach(function(p) {
          var id = getPostId(p);
          if (id) oldPostMap[id] = p;
        });

        var newPostMap = {};
        newPosts.forEach(function(p) {
          var id = getPostId(p);
          if (id) newPostMap[id] = p;
        });

        // Add new posts at top
        var composer = feedLayout.querySelector('.feed-composer');
        newPosts.forEach(function(np) {
          var id = getPostId(np);
          if (id && !oldPostMap[id]) {
            var clone = np.cloneNode(true);
            if (composer && composer.nextSibling) {
              feedLayout.insertBefore(clone, composer.nextSibling);
            }
          }
        });

        // Remove deleted posts
        oldPosts.forEach(function(op) {
          var id = getPostId(op);
          if (id && !newPostMap[id]) {
            op.remove();
          }
        });

        // Update stats on existing posts (without touching comments)
        for (var id in oldPostMap) {
          if (!newPostMap[id]) continue;
          var os = oldPostMap[id].querySelector('.post-stats');
          var ns = newPostMap[id].querySelector('.post-stats');
          if (os && ns && os.innerHTML !== ns.innerHTML) {
            os.innerHTML = ns.innerHTML;
          }
        }

        // Restore all expanded states
        expandedPosts.forEach(function(pid) {
          var post = document.getElementById('post-' + pid);
          if (!post) {
            expandedPosts.delete(pid);
            return;
          }
          var comments = post.querySelector('.post-comments');
          var toggle = post.querySelector('.comment-toggle');
          if (comments) {
            comments.classList.add('expanded');
            if (toggle) {
              toggle.textContent = 'Hide Comments';
              toggle.setAttribute('aria-expanded', 'true');
            }
          }
        });
      });
  }

  // SSE: per-post update (like, comment, share)
  evtSource.addEventListener('post-update', function(e) {
    var postId = e.data;
    if (postId) updateSinglePost(postId);
  });

  // SSE: full refresh (new post, delete)
  evtSource.addEventListener('feed-update', function() {
    fullRefresh();
  });

  // Comment toggle
  document.addEventListener('click', function(e) {
    var toggle = e.target.closest('.comment-toggle');
    if (!toggle) return;
    var post = toggle.closest('.feed-post');
    if (!post) return;
    var comments = post.querySelector('.post-comments');
    if (!comments) return;

    var postId = getPostId(post);
    comments.classList.toggle('expanded');

    if (comments.classList.contains('expanded')) {
      toggle.textContent = 'Hide Comments';
      toggle.setAttribute('aria-expanded', 'true');
      if (postId) expandedPosts.add(postId);
    } else {
      toggle.textContent = 'Show Comments';
      toggle.setAttribute('aria-expanded', 'false');
      if (postId) expandedPosts.delete(postId);
    }
  });

  // Submit reactions asynchronously so clicking an emoji does not reload page.
  document.addEventListener('submit', function(e) {
    var form = e.target.closest('.reaction-bar form[action$="/like"]');
    if (!form) return;

    e.preventDefault();

    fetch(form.action, {
      method: 'POST',
      credentials: 'same-origin',
      body: new FormData(form)
    }).then(function(resp) {
      if (!resp.ok) return;
      var match = form.action.match(/\/feed\/([^/]+)\/like$/);
      if (match && match[1]) {
        updateSinglePost(match[1]);
      }
    }).catch(function() {});
  });

  // Submit comments asynchronously so adding a comment does not reload page.
  document.addEventListener('submit', function(e) {
    var form = e.target.closest('.comment-submit-form');
    if (!form) return;

    e.preventDefault();

    var commentInput = form.querySelector('input[name="content"]');
    var content = commentInput ? (commentInput.value || '').trim() : '';
    if (!content) return;

    fetch(form.action, {
      method: 'POST',
      credentials: 'same-origin',
      body: new FormData(form)
    }).then(function(resp) {
      if (!resp.ok) return;
      if (commentInput) commentInput.value = '';
      var match = form.action.match(/\/feed\/([^/]+)\/comment$/);
      if (match && match[1]) {
        var postId = match[1];
        expandedPosts.add(postId);
        updateSinglePost(postId);
      }
    }).catch(function() {});
  });

  // Highlight from notification
  var params = new URLSearchParams(window.location.search);
  var highlightId = params.get('highlight');
  if (highlightId) {
    var targetPost = document.getElementById('post-' + highlightId);
    if (targetPost) {
      targetPost.scrollIntoView({ behavior: 'smooth', block: 'center' });
      targetPost.classList.add('feed-post-highlight');
      setTimeout(function() { targetPost.classList.remove('feed-post-highlight'); }, 3000);

      var comments = targetPost.querySelector('.post-comments');
      var toggle = targetPost.querySelector('.comment-toggle');
      if (comments && !comments.classList.contains('expanded')) {
        comments.classList.add('expanded');
        if (toggle) {
          toggle.textContent = 'Hide Comments';
          toggle.setAttribute('aria-expanded', 'true');
        }
        var pid = getPostId(targetPost);
        if (pid) expandedPosts.add(pid);
        setTimeout(function() {
          var allComments = comments.querySelectorAll('.comment');
          if (allComments.length > 0) {
            allComments[allComments.length - 1].scrollIntoView({ behavior: 'smooth', block: 'center' });
          }
        }, 500);
      }
      window.history.replaceState({}, '', '/feed');
    }
  }
})();

// PII warning (Feature: Content Moderation + PII redaction warnings)
// Before a post is submitted, scan its content for likely PII (SSN, bare
// 9-digit numbers, email addresses, phone numbers). If any pattern matches,
// ask the author to confirm. Non-blocking: confirming posts anyway.
(function() {
  var composer = document.querySelector('.feed-composer form');
  if (!composer) return;

  var piiPatterns = [
    { label: 'SSN', re: /\b\d{3}-\d{2}-\d{4}\b/ },
    { label: 'a 9-digit number (possible SSN/ID)', re: /\b\d{9}\b/ },
    { label: 'email address', re: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b/ },
    { label: 'phone number', re: /(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b/ }
  ];

  function detectPII(text) {
    var hits = [];
    for (var i = 0; i < piiPatterns.length; i++) {
      if (piiPatterns[i].re.test(text)) hits.push(piiPatterns[i].label);
    }
    return hits;
  }

  composer.addEventListener('submit', function(e) {
    var textarea = composer.querySelector('textarea[name="content"]');
    if (!textarea) return;
    var hits = detectPII(textarea.value || '');
    if (hits.length === 0) return;

    var msg = 'This looks like it may contain PII (' + hits.join(', ') +
      '). Sharing personal data publicly may violate privacy policy. Post anyway?';
    if (!window.confirm(msg)) {
      e.preventDefault();
      textarea.focus();
    }
  });
})();
