document.addEventListener('DOMContentLoaded', function () {
  var focusState = {};

  function getToggle(id) {
    return document.querySelector('[data-shell-toggle="' + id + '"]');
  }

  function getCheckbox(id) {
    return document.getElementById(id);
  }

  function getPanel(id) {
    var toggle = getToggle(id);
    if (!toggle) return null;
    return document.getElementById(toggle.getAttribute('aria-controls'));
  }

  function getFocusable(panel) {
    if (!panel) return [];
    return Array.from(panel.querySelectorAll(
      'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter(function (el) {
      return !el.classList.contains('is-hidden') && el.offsetParent !== null;
    });
  }

  function focusPanel(panel) {
    var focusables = getFocusable(panel);
    if (focusables.length > 0) {
      focusables[0].focus();
      return;
    }
    panel.focus();
  }

  function setPanelState(id, open) {
    var checkbox = getCheckbox(id);
    var toggle = getToggle(id);
    var panel = getPanel(id);

    if (!checkbox || !toggle || !panel) return;

    if (open && document.activeElement) {
      focusState[id] = document.activeElement;
    }

    checkbox.checked = open;
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    panel.setAttribute('aria-hidden', open ? 'false' : 'true');

    if (open) {
      window.requestAnimationFrame(function () {
        focusPanel(panel);
      });
      return;
    }

    if (focusState[id] && typeof focusState[id].focus === 'function') {
      focusState[id].focus();
    }
    focusState[id] = null;
  }

  function activePanel() {
    if (getCheckbox('chat-toggle') && getCheckbox('chat-toggle').checked) {
      return getPanel('chat-toggle');
    }
    if (getCheckbox('drawer-toggle') && getCheckbox('drawer-toggle').checked) {
      return getPanel('drawer-toggle');
    }
    return null;
  }

  document.querySelectorAll('[data-shell-toggle]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var id = btn.getAttribute('data-shell-toggle');
      var checkbox = getCheckbox(id);
      if (!checkbox) return;
      setPanelState(id, !checkbox.checked);
    });
  });

  document.querySelectorAll('[data-shell-close]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var id = btn.getAttribute('data-shell-close');
      setPanelState(id, false);
    });
  });

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      var modal = document.querySelector('.modal-overlay.active');
      if (modal && window.shellDialogs) {
        window.shellDialogs.closeModal(modal);
        return;
      }

      if (getCheckbox('chat-toggle') && getCheckbox('chat-toggle').checked) {
        setPanelState('chat-toggle', false);
        return;
      }

      if (getCheckbox('drawer-toggle') && getCheckbox('drawer-toggle').checked) {
        setPanelState('drawer-toggle', false);
        return;
      }

      document.querySelectorAll('.nav-dropdown[open]').forEach(function (details) {
        details.open = false;
      });
      return;
    }

    if (e.key !== 'Tab') return;

    var modal = document.querySelector('.modal-overlay.active');
    var panel = modal || activePanel();
    if (!panel) return;

    var focusables = getFocusable(panel);
    if (focusables.length === 0) return;

    var first = focusables[0];
    var last = focusables[focusables.length - 1];

    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
      return;
    }

    if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  });

  document.querySelectorAll('.nav-dropdown').forEach(function (details) {
    var summary = details.querySelector('summary');
    if (!summary) return;
    summary.setAttribute('aria-expanded', 'false');
    details.addEventListener('toggle', function () {
      summary.setAttribute('aria-expanded', details.open ? 'true' : 'false');
    });
  });

  document.addEventListener('click', function (e) {
    document.querySelectorAll('.nav-dropdown[open]').forEach(function (details) {
      if (!details.contains(e.target)) {
        details.open = false;
      }
    });
  });

  window.shellPanels = {
    open: function (id) { setPanelState(id, true); },
    close: function (id) { setPanelState(id, false); }
  };

  window.shellDialogs = {
    openModal: function (modal, focusTarget) {
      if (!modal) return;
      focusState.modal = document.activeElement;
      if (modal.classList.contains('chat-modal')) {
        document.body.classList.add('chat-modal-open');
      }
      modal.classList.add('active');
      modal.setAttribute('aria-hidden', 'false');
      window.requestAnimationFrame(function () {
        if (focusTarget && typeof focusTarget.focus === 'function') {
          focusTarget.focus();
          return;
        }
        focusPanel(modal);
      });
    },
    closeModal: function (modal) {
      if (!modal) return;
      modal.classList.remove('active');
      modal.setAttribute('aria-hidden', 'true');
      if (modal.classList.contains('chat-modal')) {
        document.body.classList.remove('chat-modal-open');
      }
      if (focusState.modal && typeof focusState.modal.focus === 'function') {
        focusState.modal.focus();
      }
      focusState.modal = null;
    }
  };
});
