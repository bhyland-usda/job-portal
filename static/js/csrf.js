(function () {
  function getCookie(name) {
    var cookie = document.cookie || "";
    var parts = cookie.split(";");
    for (var i = 0; i < parts.length; i++) {
      var p = parts[i].trim();
      if (p.indexOf(name + "=") === 0) {
        return decodeURIComponent(p.substring(name.length + 1));
      }
    }
    return "";
  }

  function isUnsafe(method) {
    var m = (method || "GET").toUpperCase();
    return m === "POST" || m === "PUT" || m === "PATCH" || m === "DELETE";
  }

  function ensureFormTokens() {
    var token = getCookie("csrf_token");
    if (!token) {
      return;
    }

    var forms = document.querySelectorAll("form");
    for (var i = 0; i < forms.length; i++) {
      var form = forms[i];
      var method = (form.getAttribute("method") || "GET").toUpperCase();
      if (!isUnsafe(method)) {
        continue;
      }
      if (form.querySelector('input[name="csrf_token"]')) {
        continue;
      }
      var input = document.createElement("input");
      input.type = "hidden";
      input.name = "csrf_token";
      input.value = token;
      form.appendChild(input);
    }
  }

  function isSameOriginUrl(raw) {
    if (!raw) {
      return true;
    }
    var u = new URL(raw, window.location.origin);
    return u.origin === window.location.origin;
  }

  function patchFetch() {
    if (!window.fetch) {
      return;
    }

    var originalFetch = window.fetch;
    window.fetch = function (input, init) {
      var cfg = init || {};
      var method = (cfg.method || "GET").toUpperCase();
      var url = typeof input === "string" ? input : (input && input.url) || "";

      if (isUnsafe(method) && isSameOriginUrl(url)) {
        var token = getCookie("csrf_token");
        if (token) {
          var headers = new Headers(cfg.headers || (input && input.headers) || undefined);
          if (!headers.has("X-CSRF-Token")) {
            headers.set("X-CSRF-Token", token);
          }
          cfg.headers = headers;
        }
      }

      return originalFetch.call(window, input, cfg);
    };
  }

  ensureFormTokens();
  patchFetch();
})();
