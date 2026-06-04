// Chat drawer — full inline messaging with modal for new chats
document.addEventListener("DOMContentLoaded", function () {
  var chatList = document.getElementById('chat-drawer-list');
  var listView = document.getElementById('chat-list-view');
  var convView = document.getElementById('chat-conv-view');
  var convMessages = document.getElementById('chat-conv-messages');
  var convName = document.getElementById('chat-conv-name');
  var backBtn = document.getElementById('chat-back-btn');
  var sendForm = document.getElementById('chat-send-form');
  var sendInput = document.getElementById('chat-send-input');

  // Modal elements
  var modal = document.getElementById('new-chat-modal');
  var modalClose = document.getElementById('chat-modal-close');
  var modalSearch = document.getElementById('chat-new-search');
  var modalResults = document.getElementById('chat-new-results');
  var modalSelected = document.getElementById('chat-new-selected');
  var modalStart = document.getElementById('chat-new-start');
  var modalMessage = document.getElementById('chat-first-message');

  var currentConvID = null;
  var chatPollInterval = null;
  var selectedUsers = [];
  var lastTypingSent = 0;

  function getCookie(name) {
    var cookie = document.cookie || '';
    var parts = cookie.split(';');
    for (var i = 0; i < parts.length; i++) {
      var p = parts[i].trim();
      if (p.indexOf(name + '=') === 0) {
        return decodeURIComponent(p.substring(name.length + 1));
      }
    }
    return '';
  }

  function setHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', hidden);
  }

  // Group naming (feature: group messaging). A name input is injected into the
  // modal body and only revealed once the selection is a true group (2+ others).
  // base.html is shared and not edited, so the field is created here at runtime.
  var groupNameInput = null;
  if (modalSelected && modalSelected.parentNode) {
    var gnWrap = document.createElement('div');
    gnWrap.id = 'chat-group-name-wrap';
    gnWrap.className = 'chat-group-name-wrap is-hidden';
    var gnLabel = document.createElement('label');
    gnLabel.className = 'modal-label';
    gnLabel.textContent = 'Group name (optional):';
    groupNameInput = document.createElement('input');
    groupNameInput.type = 'text';
    groupNameInput.id = 'chat-group-name';
    groupNameInput.className = 'chat-search-input';
    groupNameInput.placeholder = 'Name this group...';
    gnWrap.appendChild(gnLabel);
    gnWrap.appendChild(groupNameInput);
    // Place the group-name field right after the selected-users chips.
    modalSelected.parentNode.insertBefore(gnWrap, modalSelected.nextSibling);
  }

  // Reveal the group-name field only for a true group (more than one other user).
  function updateGroupNameVisibility() {
    var wrap = document.getElementById('chat-group-name-wrap');
    if (!wrap) return;
    setHidden(wrap, selectedUsers.length <= 1);
  }

  // SSE + typing indicator state (BUG A)
  var typingEl = document.getElementById('chat-typing');
  var chatEvents = null;       // EventSource for /messages/events
  var typingHideTimer = null;  // hides #chat-typing after a quiet period

  // Hide the typing indicator by default (it has no CSS to hide it otherwise).
  setHidden(typingEl, true);

  if (!chatList) return;

  // --- Pin button (BUG B) ---
  // Renders a small pin/unpin icon button for a contact. The button carries the
  // contact's userID; the click handler POSTs to /messages-pin/{userID}.
  function pinButton(userID, isPinned) {
    var title = isPinned ? 'Unpin contact' : 'Pin contact';
    var icon = isPinned ? '&#128204;' : '&#128205;';
    return '<button type="button" class="chat-pin-btn' + (isPinned ? ' is-pinned' : '')
      + '" data-pin-user="' + userID + '" title="' + title + '" aria-label="' + title + '">'
      + icon + '</button>';
  }

  function togglePin(userID) {
    fetch('/messages-pin/' + encodeURIComponent(userID), {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest', 'Accept': 'application/json' }
    }).then(function() {
      loadConversationList();
    }).catch(function() {});
  }

  // --- Conversation List ---
  function loadConversationList() {
    fetch('/messages/recent', { credentials: 'same-origin' })
      .then(function(r) { return r.text(); })
      .then(function(text) {
        try {
          var data = JSON.parse(text);
          var html = '';
          var pinned = data.pinned || [];
          var recent = data.recent || [];
          if (pinned.length > 0) {
            html += '<div class="chat-section-label">Pinned</div>';
            for (var i = 0; i < pinned.length; i++) {
              html += '<div class="chat-conv-item">'
                + '<button type="button" data-start-conv="' + pinned[i].id + '" data-name="' + pinned[i].name + '" class="chat-conv-launch">'
                + '<div class="chat-conv-name">&#128204; ' + pinned[i].name + '</div>'
                + '</button>'
                + pinButton(pinned[i].id, true)
                + '</div>';
            }
            html += '<div class="chat-divider"></div>';
          }
          for (var j = 0; j < recent.length; j++) {
            var c = recent[j];
            var cls = c.unread > 0 ? ' chat-conv-unread' : '';
            var icon = c.is_group ? '&#128101; ' : '';
            html += '<div class="chat-conv-item' + cls + '">'
              + '<button type="button" data-conv="' + c.id + '" data-name="' + c.name + '" class="chat-conv-launch">'
              + '<div class="chat-conv-name">' + icon + c.name + (c.unread > 0 ? ' <span class="unread-badge">' + c.unread + '</span>' : '') + '</div>'
              + '<div class="chat-conv-preview">' + (c.last_message || '') + '</div>'
              + '</button>'
              + (c.other_user_id ? pinButton(c.other_user_id, c.pinned) : '')
              + '</div>';
          }
          chatList.innerHTML = html || '<p class="chat-placeholder">No conversations yet. Click + New to start one!</p>';
        } catch(e) {
          chatList.innerHTML = '<p class="chat-placeholder">No conversations yet.</p>';
        }
      }).catch(function() {
        chatList.innerHTML = '<p class="chat-placeholder">No conversations yet.</p>';
      });
  }

  // --- Realtime stream (BUG A) ---
  // The SSE stream is per-user. A "typing" data line means the other person is
  // typing (show the indicator, do NOT reload). An "update" data line means a
  // new message arrived (reload the open conversation).
  function showTyping() {
    if (!typingEl) return;
    setHidden(typingEl, false);
    clearTimeout(typingHideTimer);
    typingHideTimer = setTimeout(hideTyping, 3000);
  }

  function hideTyping() {
    clearTimeout(typingHideTimer);
    typingHideTimer = null;
    setHidden(typingEl, true);
  }

  function connectEvents() {
    if (chatEvents || typeof EventSource === 'undefined') return;
    try {
      chatEvents = new EventSource('/messages/events', { withCredentials: true });
    } catch (e) {
      chatEvents = null;
      return;
    }
    chatEvents.onmessage = function(ev) {
      var kind = (ev.data || '').trim();
      if (kind === 'typing') {
        if (currentConvID) showTyping();
      } else if (kind === 'update') {
        // New message: refresh the open conversation, and the list preview.
        if (currentConvID) loadMessages();
        if (listView && !listView.classList.contains('chat-panel-hidden')) loadConversationList();
      }
    };
    chatEvents.onerror = function() {
      // The browser auto-reconnects EventSource; nothing to do here. The 3s
      // poll fallback in openChat covers any gap.
    };
  }

  function disconnectEvents() {
    if (chatEvents) {
      chatEvents.close();
      chatEvents = null;
    }
  }

  // --- Add member control (feature: group messaging) ---
  // base.html is shared and not edited here, so the "add member" button and its
  // inline search picker are created at runtime inside the existing conversation
  // header. The picker reuses /messages/search-users and POSTs the chosen user
  // to /messages/{id}/members; the server only permits this for participants.
  var addMemberBtn = null;
  var addMemberPicker = null;
  var addMemberSearch = null;
  var addMemberResults = null;
  if (convName && convName.parentNode) {
    addMemberBtn = document.createElement('button');
    addMemberBtn.type = 'button';
    addMemberBtn.id = 'chat-add-member-btn';
    addMemberBtn.className = 'chat-back chat-add-member-btn';
    addMemberBtn.title = 'Add member';
    addMemberBtn.setAttribute('aria-label', 'Add member');
    addMemberBtn.innerHTML = '&#128101;+';
    convName.parentNode.appendChild(addMemberBtn);

    addMemberPicker = document.createElement('div');
    addMemberPicker.id = 'chat-add-member-picker';
    addMemberPicker.className = 'chat-add-member-picker is-hidden';
    addMemberSearch = document.createElement('input');
    addMemberSearch.type = 'text';
    addMemberSearch.className = 'chat-search-input';
    addMemberSearch.placeholder = 'Add someone by name...';
    addMemberResults = document.createElement('div');
    addMemberResults.className = 'chat-results';
    addMemberPicker.appendChild(addMemberSearch);
    addMemberPicker.appendChild(addMemberResults);
    // Insert the picker just below the header so it sits above the messages.
    if (convView && convMessages) {
      convView.insertBefore(addMemberPicker, convMessages);
    }

    addMemberBtn.addEventListener('click', function() {
      var open = !addMemberPicker.classList.contains('is-hidden');
      setHidden(addMemberPicker, open);
      if (!open) {
        addMemberSearch.value = '';
        addMemberResults.innerHTML = '';
        addMemberSearch.focus();
      }
    });

    var addSearchTimer = null;
    addMemberSearch.addEventListener('input', function() {
      clearTimeout(addSearchTimer);
      var q = addMemberSearch.value;
      addSearchTimer = setTimeout(function() {
        if (q.length < 2) { addMemberResults.innerHTML = ''; return; }
        fetch('/messages/search-users?q=' + encodeURIComponent(q) + '&scope=all', { credentials: 'same-origin' })
          .then(function(r) { return r.json(); })
          .then(function(users) {
            var html = '';
            for (var i = 0; i < users.length; i++) {
              html += '<button type="button" class="chat-conv-launch" data-add-member="' + users[i].id + '">'
                + '<div class="chat-conv-name">' + users[i].name + '</div></button>';
            }
            addMemberResults.innerHTML = html || '<p class="chat-placeholder">No users found</p>';
          }).catch(function() {});
      }, 300);
    });

    addMemberResults.addEventListener('click', function(e) {
      var item = e.target.closest('[data-add-member]');
      if (!item || !currentConvID) return;
      e.preventDefault();
      var uid = item.getAttribute('data-add-member');
      fetch('/messages/chat/' + encodeURIComponent(currentConvID) + '/members', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'user_id=' + encodeURIComponent(uid)
      }).then(function() {
        setHidden(addMemberPicker, true);
        addMemberSearch.value = '';
        addMemberResults.innerHTML = '';
        loadConversationList();
      }).catch(function() {});
    });
  }

  // --- Open Chat in Drawer ---
  window.openChat = function(convID, name) {
    currentConvID = convID;
    lastMessageCount = 0;
    if (convMessages) convMessages.innerHTML = '';
    if (convName) convName.textContent = name;
    if (addMemberPicker) setHidden(addMemberPicker, true);
    if (listView) listView.classList.add('chat-panel-hidden');
    if (convView) {
      convView.classList.remove('chat-panel-hidden');
    }
    hideTyping();
    loadMessages();
    // Realtime via SSE; keep a low-frequency poll as a fallback.
    connectEvents();
    clearInterval(chatPollInterval);
    chatPollInterval = setInterval(loadMessages, 3000);
  };

  function closeChat() {
    currentConvID = null;
    clearInterval(chatPollInterval);
    disconnectEvents();
    hideTyping();
    if (listView) listView.classList.remove('chat-panel-hidden');
    if (convView) convView.classList.add('chat-panel-hidden');
    loadConversationList();
  }

  // --- Load Messages ---
  var lastMessageCount = 0;

  function renderBubble(m) {
    var bubbleCls = m.is_own ? 'chat-bubble-own' : 'chat-bubble-other';
    var html = '<div class="chat-bubble-row ' + (m.is_own ? 'chat-row-own' : 'chat-row-other') + '" data-msg-id="' + m.id + '">'
      + '<div class="chat-bubble ' + bubbleCls + '">';
    if (!m.is_own) {
      html += '<div class="chat-bubble-name">' + m.name + '</div>';
    }
    html += '<div>' + m.content + '</div>';
    if (m.attachments && m.attachments.length > 0) {
      for (var a = 0; a < m.attachments.length; a++) {
        var att = m.attachments[a];
        var isGif = att.content_type === 'image/gif';
        if (att.category === 'image') {
          html += '<div class="chat-attachment' + (isGif ? ' gif-container' : '') + '">'
            + '<img src="/messages/attachment/' + att.id + '" alt="' + att.original_name + '" class="chat-att-image" data-media="' + (isGif ? 'gif' : 'image') + '" loading="lazy" />'
            + (isGif ? '<div class="gif-overlay">GIF</div>' : '')
            + '</div>';
        } else if (att.category === 'video') {
          html += '<div class="chat-attachment"><video src="/messages/attachment/' + att.id + '" class="chat-att-video" data-media="video" controls preload="metadata"></video></div>';
        } else {
          html += '<a href="/messages/attachment/' + att.id + '" class="chat-att-file" target="_blank">&#128206; ' + att.original_name + '</a>';
        }
      }
    }
    html += '<div class="chat-bubble-time">' + m.created_at;
    if (m.read_at) {
      html += ' &middot; ' + m.read_at;
    }
    html += '</div></div></div>';
    return html;
  }

  function loadMessages() {
    if (!currentConvID || !convMessages) return;
    fetch('/messages/chat/' + currentConvID, { credentials: 'same-origin' })
      .then(function(r) {
        // Group naming: the server surfaces the group's display name as a
        // response header (the body stays a plain message array). When present,
        // it overrides the participant-name header the drawer opened with.
        var gName = r.headers.get('X-Conversation-Name');
        if (gName && convName) convName.textContent = gName;
        return r.text();
      })
      .then(function(text) {
        try {
          var msgs = JSON.parse(text);
          var wasAtBottom = convMessages.scrollHeight - convMessages.scrollTop - convMessages.clientHeight < 50;

          if (msgs.length === 0 && lastMessageCount === 0) {
            convMessages.innerHTML = '<p class="chat-placeholder chat-empty-center">No messages yet. Say hello!</p>';
            lastMessageCount = 0;
            return;
          }

          // Remove placeholder if present
          var placeholder = convMessages.querySelector('.chat-placeholder');
          if (placeholder && msgs.length > 0) placeholder.remove();

          if (msgs.length > lastMessageCount) {
            // Append only new messages
            for (var i = lastMessageCount; i < msgs.length; i++) {
              convMessages.insertAdjacentHTML('beforeend', renderBubble(msgs[i]));
            }
            lastMessageCount = msgs.length;
            if (wasAtBottom) convMessages.scrollTop = convMessages.scrollHeight;
            if (window.initMediaControls) window.initMediaControls();
          } else if (msgs.length === lastMessageCount) {
            // Check for read receipt updates on last own message
            var existing = convMessages.querySelectorAll('[data-msg-id]');
            if (existing.length > 0 && msgs.length > 0) {
              var lastMsg = msgs[msgs.length - 1];
              var lastEl = existing[existing.length - 1];
              if (lastMsg.is_own && lastMsg.read_at) {
                var timeEl = lastEl.querySelector('.chat-bubble-time');
                if (timeEl && timeEl.innerHTML.indexOf(lastMsg.read_at) === -1) {
                  timeEl.innerHTML = lastMsg.created_at + ' &middot; ' + lastMsg.read_at;
                }
              }
            }
          }
        } catch(e) {}
      }).catch(function() {});
  }

  // --- New Chat Modal ---
  function openModal() {
    selectedUsers = [];
    if (modalSearch) modalSearch.value = '';
    if (modalResults) modalResults.innerHTML = '';
    if (modalSelected) modalSelected.innerHTML = '';
    if (modalMessage) modalMessage.value = '';
    if (groupNameInput) groupNameInput.value = '';
    updateGroupNameVisibility();
    document.body.classList.add('chat-modal-open');
    if (window.shellDialogs && modal) {
      window.shellDialogs.openModal(modal, modalSearch);
      return;
    }
    if (modal) modal.classList.add('active');
  }

  function closeModal() {
    document.body.classList.remove('chat-modal-open');
    if (window.shellDialogs && modal) {
      window.shellDialogs.closeModal(modal);
      return;
    }
    if (modal) modal.classList.remove('active');
  }

  function searchContacts(query) {
    if (query.length < 2) {
      if (modalResults) modalResults.innerHTML = '';
      return;
    }
    var scopeEl = document.querySelector('input[name="chat-scope"]:checked');
    var scope = scopeEl ? scopeEl.value : 'contacts';
    var scopeParam = scope === 'all' ? '&scope=all' : '';
    fetch('/messages/search-users?q=' + encodeURIComponent(query) + scopeParam, { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(users) {
        var html = '';
        for (var i = 0; i < users.length; i++) {
          var u = users[i];
          if (!selectedUsers.some(function(s) { return s.id === u.id; })) {
            html += '<button type="button" class="chat-conv-launch" data-add-user="' + u.id + '" data-add-name="' + u.name + '">'
              + '<div class="chat-conv-name">' + u.name + '</div></button>';
          }
        }
        if (modalResults) modalResults.innerHTML = html || '<p class="chat-placeholder">No users found</p>';
      }).catch(function() {});
  }

  function renderSelectedUsers() {
    if (!modalSelected) return;
    var html = '';
    for (var i = 0; i < selectedUsers.length; i++) {
      var u = selectedUsers[i];
      html += '<span class="chat-selected-user">' + u.name
        + ' <button type="button" class="chat-selected-remove" data-remove-user="' + u.id + '" aria-label="Remove ' + u.name + '">&times;</button></span>';
    }
    modalSelected.innerHTML = html;
    updateGroupNameVisibility();
  }

  function startConversation() {
    if (selectedUsers.length === 0) return;
    var body = selectedUsers.map(function(u) { return 'user_ids=' + encodeURIComponent(u.id); }).join('&');
    // Group naming: include the optional name only for a true group (2+ others).
    var groupName = (selectedUsers.length > 1 && groupNameInput) ? groupNameInput.value.trim() : '';
    if (groupName) {
      body += '&name=' + encodeURIComponent(groupName);
    }
    // Header label: prefer the explicit group name, else the joined participants.
    var headerName = groupName || selectedUsers.map(function(u) { return u.name; }).join(', ');
    fetch('/messages/start', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body
    }).then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.id) {
          // Send first message if provided
          var msg = modalMessage ? modalMessage.value.trim() : '';
          if (msg) {
            fetch('/messages/chat/' + data.id, {
              method: 'POST',
              credentials: 'same-origin',
              headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
              body: 'content=' + encodeURIComponent(msg)
            }).then(function() {
              closeModal();
              if (window.shellPanels) {
                window.shellPanels.open('chat-toggle');
              } else {
                document.getElementById('chat-toggle').checked = true;
              }
              window.openChat(data.id, headerName);
            });
          } else {
            closeModal();
            if (window.shellPanels) {
              window.shellPanels.open('chat-toggle');
            } else {
              document.getElementById('chat-toggle').checked = true;
            }
            window.openChat(data.id, headerName);
          }
        }
      }).catch(function() {});
  }

  // --- Event Listeners ---
  chatList.addEventListener('click', function(e) {
    // Pin/unpin button takes priority over opening the conversation.
    var pinBtn = e.target.closest('[data-pin-user]');
    if (pinBtn) {
      e.preventDefault();
      e.stopPropagation();
      togglePin(pinBtn.getAttribute('data-pin-user'));
      return;
    }
    var item = e.target.closest('[data-conv]');
    if (item) {
      e.preventDefault();
      window.openChat(item.getAttribute('data-conv'), item.getAttribute('data-name'));
      return;
    }
    var startItem = e.target.closest('[data-start-conv]');
    if (startItem) {
      e.preventDefault();
      var contactName = (startItem.getAttribute('data-name') || '').trim();
      fetch('/messages/new/' + startItem.getAttribute('data-start-conv'), {
        method: 'POST', credentials: 'same-origin'
      }).then(function(r) { return r.json(); })
        .then(function(data) {
          if (data.id) window.openChat(data.id, contactName);
        }).catch(function() {});
      return;
    }
  });

  if (backBtn) backBtn.addEventListener('click', closeChat);

  // New chat button opens modal
  var newChatBtn = document.getElementById('chat-new-btn');
  if (newChatBtn) newChatBtn.addEventListener('click', openModal);
  if (modalClose) modalClose.addEventListener('click', closeModal);

  // Close modal on overlay click
  if (modal) {
    modal.addEventListener('click', function(e) {
      if (e.target === modal) closeModal();
    });
  }

  // Search contacts in modal
  if (modalSearch) {
    var searchTimer = null;
    modalSearch.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function() { searchContacts(modalSearch.value); }, 300);
    });
  }

  // Add user from search results
  if (modalResults) {
    modalResults.addEventListener('click', function(e) {
      var item = e.target.closest('[data-add-user]');
      if (item) {
        e.preventDefault();
        var id = item.getAttribute('data-add-user');
        var name = item.getAttribute('data-add-name');
        if (!selectedUsers.some(function(s) { return s.id === id; })) {
          selectedUsers.push({ id: id, name: name });
          renderSelectedUsers();
          if (modalSearch) modalSearch.value = '';
          if (modalResults) modalResults.innerHTML = '';
        }
      }
    });
  }

  // Remove selected user
  if (modalSelected) {
    modalSelected.addEventListener('click', function(e) {
      var remove = e.target.closest('[data-remove-user]');
      if (remove) {
        e.preventDefault();
        var id = remove.getAttribute('data-remove-user');
        selectedUsers = selectedUsers.filter(function(s) { return s.id !== id; });
        renderSelectedUsers();
      }
    });
  }

  // Start conversation button
  if (modalStart) modalStart.addEventListener('click', startConversation);

  // Send message in chat
  if (sendForm) {
    sendForm.addEventListener('submit', function(e) {
      e.preventDefault();
      var fileInput = document.getElementById('chat-attach-input');
      var hasFile = fileInput && fileInput.files.length > 0;
      if (!currentConvID || (!sendInput.value.trim() && !hasFile)) return;

      var formData = new FormData();
      if (sendInput.value.trim()) {
        formData.append('content', sendInput.value.trim());
      }
      if (hasFile) {
        formData.append('attachment', fileInput.files[0]);
      }

      sendInput.value = '';
      if (fileInput) fileInput.value = '';

      fetch('/messages/chat/' + currentConvID, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'X-CSRF-Token': getCookie('csrf_token')
        },
        body: formData
      }).then(function() { loadMessages(); }).catch(function() {});
    });
  }

  // Typing indicator
  if (sendInput) {
    sendInput.addEventListener('input', function() {
      var now = Date.now();
      if (currentConvID && now - lastTypingSent > 2000) {
        lastTypingSent = now;
        fetch('/messages-typing/' + currentConvID, {
          method: 'POST', credentials: 'same-origin'
        }).catch(function() {});
      }
    });
  }

  // Profile "Message" button handler
  document.addEventListener('click', function(e) {
    var msgBtn = e.target.closest('[data-message-user]');
    if (!msgBtn) return;
    e.preventDefault();
    var userId = msgBtn.getAttribute('data-message-user');
    var userName = msgBtn.getAttribute('data-message-name');
    if (window.shellPanels) {
      window.shellPanels.open('chat-toggle');
    } else {
      var chatToggle = document.getElementById('chat-toggle');
      if (chatToggle) chatToggle.checked = true;
    }
    fetch('/messages/new/' + userId, {
      method: 'POST', credentials: 'same-origin'
    }).then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.id) window.openChat(data.id, userName);
      }).catch(function() {});
  });

  // Initial load
  loadConversationList();

  // Auto-open chat from URL param (e.g., from notification click)
  var params = new URLSearchParams(window.location.search);
  var openChatID = params.get('open_chat');
  if (openChatID) {
    if (window.shellPanels) {
      window.shellPanels.open('chat-toggle');
    } else {
      var chatToggle = document.getElementById('chat-toggle');
      if (chatToggle) chatToggle.checked = true;
    }
    // Small delay to let conversation list load, then find the name
    setTimeout(function() {
      var convItem = document.querySelector('[data-conv="' + openChatID + '"]');
      var name = convItem ? convItem.getAttribute('data-name') : 'Chat';
      window.openChat(openChatID, name);
    }, 500);
  }
});
