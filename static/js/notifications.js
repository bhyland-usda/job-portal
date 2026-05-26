// Notification badge + dropdown
document.addEventListener("DOMContentLoaded", function () {
  var badge = document.getElementById("notif-badge");
  var notifList = document.getElementById("notif-list");

  function setEmptyState(message) {
    if (!notifList) return;
    var empty = document.createElement("p");
    empty.className = "notif-empty";
    empty.textContent = message;
    notifList.replaceChildren(empty);
  }

  function updateBadge(count) {
    if (!badge) return;
    if (count > 0) {
      badge.textContent = count;
      badge.hidden = false;
    } else {
      badge.hidden = true;
    }
  }

  // Initial badge count
  fetch("/notifications/count", { credentials: "same-origin" })
    .then(function (res) { return res.json(); })
    .then(function (data) { updateBadge(data.count); })
    .catch(function () {});

  // SSE for real-time badge updates
  var source = new EventSource("/notifications/stream");
  source.onmessage = function(event) {
    try {
      var data = JSON.parse(event.data);
      updateBadge(data.count);
      loadNotifications();
    } catch(e) {}
  };

  function createActionForm(url, className, label, icon) {
    var form = document.createElement("form");
    form.method = "POST";
    form.action = url;
    form.className = "inline";

    var button = document.createElement("button");
    button.type = "submit";
    button.className = className;
    button.setAttribute("aria-label", label);
    button.title = label;
    button.innerHTML = icon;

    form.appendChild(button);
    return form;
  }

  function buildNotificationItem(n) {
    var item = document.createElement("div");
    item.className = "notif-item" + (n.read ? "" : " notif-item-unread");

    var avatarLink = document.createElement("a");
    avatarLink.href = n.click_url;
    avatarLink.className = "notif-avatar";
    avatarLink.setAttribute("aria-hidden", "true");
    avatarLink.tabIndex = -1;

    if (n.sender_avatar) {
      var avatarImg = document.createElement("img");
      avatarImg.src = n.sender_avatar;
      avatarImg.alt = "";
      avatarLink.appendChild(avatarImg);
    } else {
      avatarLink.textContent = n.sender_initials;
    }

    var bodyLink = document.createElement("a");
    bodyLink.href = n.click_url;
    bodyLink.className = "notif-body";

    var text = document.createElement("div");
    text.className = "notif-text";
    text.textContent = n.message;

    var time = document.createElement("div");
    time.className = "notif-time";
    time.textContent = n.time_ago;

    bodyLink.appendChild(text);
    bodyLink.appendChild(time);

    var actions = document.createElement("div");
    actions.className = "notif-actions";

    if (!n.read) {
      actions.appendChild(
        createActionForm(
          "/notifications/" + n.id + "/read",
          "notif-read-icon",
          "Mark notification as read",
          "&#128064;"
        )
      );
    } else {
      var readIcon = document.createElement("span");
      readIcon.className = "notif-read-icon is-read";
      readIcon.setAttribute("aria-label", "Notification read");
      readIcon.title = "Notification read";
      readIcon.innerHTML = "&#128065;";
      actions.appendChild(readIcon);
    }

    actions.appendChild(
      createActionForm(
        "/notifications/" + n.id + "/delete",
        "notif-delete-btn",
        "Delete notification",
        "&#128465;"
      )
    );

    item.appendChild(avatarLink);
    item.appendChild(bodyLink);
    item.appendChild(actions);
    return item;
  }

  // Load notifications into dropdown
  function loadNotifications() {
    if (!notifList) return;
    fetch("/notifications/recent", { credentials: "same-origin" })
      .then(function(r) { return r.text(); })
      .then(function(text) {
        try {
          var notifs = JSON.parse(text);
          if (!notifs || notifs.length === 0) {
            setEmptyState("No notifications yet.");
            return;
          }
          var fragment = document.createDocumentFragment();
          for (var i = 0; i < notifs.length; i++) {
            fragment.appendChild(buildNotificationItem(notifs[i]));
          }
          notifList.replaceChildren(fragment);
        } catch(e) {
          setEmptyState("No notifications yet.");
        }
      })
      .catch(function() {
        setEmptyState("No notifications yet.");
      });
  }

  // Handle mark read and delete without page reload
  if (notifList) {
    notifList.addEventListener('submit', function(e) {
      var form = e.target.closest('form');
      if (!form) return;
      e.preventDefault();
      fetch(form.action, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Accept': 'application/json' }
      }).then(function() {
        loadNotifications();
        fetch("/notifications/count", { credentials: "same-origin" })
          .then(function(r) { return r.json(); })
          .then(function(d) { updateBadge(d.count); });
      }).catch(function() {});
    });
  }

  // Mark all read handler
  var markAllBtn = document.getElementById('mark-all-read-btn');
  if (markAllBtn) {
    markAllBtn.addEventListener('click', function(e) {
      e.preventDefault();
      fetch('/notifications/read-all', {
        method: 'POST',
        credentials: 'same-origin'
      }).then(function() {
        loadNotifications();
        updateBadge(0);
      }).catch(function() {});
    });
  }

  // Initial load
  loadNotifications();
});
