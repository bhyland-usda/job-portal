(function() {
  var editors = document.querySelectorAll('[data-rich-editor]');
  if (!editors.length) return;

  function renderNodeToMarkdown(node) {
    if (!node) return '';
    if (node.nodeType === Node.TEXT_NODE) {
      return node.textContent || '';
    }
    if (node.nodeType !== Node.ELEMENT_NODE) {
      return '';
    }

    var tag = node.tagName.toLowerCase();
    var childText = '';
    for (var i = 0; i < node.childNodes.length; i++) {
      childText += renderNodeToMarkdown(node.childNodes[i]);
    }

    if (tag === 'br') return '\n';
    if (tag === 'strong' || tag === 'b') return '**' + childText + '**';
    if (tag === 'em' || tag === 'i') return '*' + childText + '*';
    if (tag === 'a') {
      var href = node.getAttribute('href') || '';
      var txt = childText || href;
      if (!href) return txt;
      return '[' + txt + '](' + href + ')';
    }
    if (tag === 'li') return '- ' + childText.trim() + '\n';
    if (tag === 'ul' || tag === 'ol') return '\n' + childText + '\n';
    if (tag === 'div' || tag === 'p') return childText + '\n';

    return childText;
  }

  function toMarkdown(input) {
    var out = '';
    for (var i = 0; i < input.childNodes.length; i++) {
      out += renderNodeToMarkdown(input.childNodes[i]);
    }
    return out
      .replace(/\r\n/g, '\n')
      .replace(/\n{3,}/g, '\n\n')
      .trim();
  }

  function normalizeText(text) {
    return (text || '').replace(/\u00a0/g, ' ').trim();
  }

  function syncOutput(editor) {
    var input = editor.querySelector('[data-rich-input]');
    var output = editor.querySelector('[data-rich-output]');
    if (!input || !output) return '';

    output.value = toMarkdown(input);
    return output.value;
  }

  function runCommand(editor, command) {
    var input = editor.querySelector('[data-rich-input]');
    if (!input) return;

    input.focus();
    if (command === 'createLink') {
      var url = window.prompt('Enter link URL');
      if (!url) return;
      document.execCommand('createLink', false, url);
      return;
    }
    document.execCommand(command, false, null);
  }

  function setupScheduleControls(form) {
    var toggle = form.querySelector('[data-schedule-toggle]');
    var panel = form.querySelector('[data-schedule-panel]');
    var scheduleSubmit = form.querySelector('[data-schedule-submit]');
    var scheduleInput = form.querySelector('[data-schedule-input]');
    if (!toggle || !panel || !scheduleSubmit || !scheduleInput) return;

    function openPanel() {
      panel.hidden = false;
      scheduleSubmit.hidden = false;
    }

    function closePanel() {
      panel.hidden = true;
      scheduleSubmit.hidden = true;
      scheduleInput.value = '';
    }

    toggle.addEventListener('click', function() {
      if (panel.hidden) {
        openPanel();
        scheduleInput.focus();
      } else {
        closePanel();
      }
    });

    form.addEventListener('submit', function(e) {
      var submitter = e.submitter;
      if (!submitter || !submitter.matches('[data-schedule-submit]')) return;
      if (!scheduleInput.value) {
        e.preventDefault();
        openPanel();
        scheduleInput.focus();
      }
    });
  }

  editors.forEach(function(editor) {
    var input = editor.querySelector('[data-rich-input]');
    if (!input) return;

    syncOutput(editor);

    editor.addEventListener('click', function(e) {
      var btn = e.target.closest('[data-rich-command]');
      if (!btn) return;
      e.preventDefault();
      runCommand(editor, btn.getAttribute('data-rich-command'));
      syncOutput(editor);
    });

    input.addEventListener('input', function() {
      syncOutput(editor);
    });

    input.addEventListener('paste', function(e) {
      e.preventDefault();
      var txt = (e.clipboardData || window.clipboardData).getData('text');
      document.execCommand('insertText', false, txt);
      syncOutput(editor);
    });

    var form = editor.closest('form');
    if (!form) return;

    setupScheduleControls(form);

    form.addEventListener('submit', function(e) {
      var value = normalizeText(syncOutput(editor));
      if (!value) {
        e.preventDefault();
        input.focus();
      }
    });
  });
})();
