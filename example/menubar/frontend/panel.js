// The tray panel. It talks to Go only through window.vitra.invoke; the
// grant lets it call usage.follow and panel.close, nothing else.
(function () {
  "use strict";

  function invoke(command) {
    return window.vitra.invoke(command, {}, "");
  }

  function formatBytes(n) {
    var units = ["B", "KB", "MB", "GB", "TB", "PB"];
    var i = 0;
    while (n >= 1000 && i < units.length - 1) {
      n /= 1000;
      i++;
    }
    return (n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)) + " " + units[i];
  }

  function show(u) {
    document.getElementById("path").textContent = u.path;
    document.getElementById("free").textContent = formatBytes(u.free);
    document.getElementById("used").textContent =
      formatBytes(u.total - u.free) + " of " + formatBytes(u.total) + " used (" + u.usedPercent.toFixed(1) + "%)";
    var fill = document.getElementById("fill");
    fill.style.width = u.usedPercent + "%";
    fill.className = "fill" + (u.usedPercent >= 90 ? " full" : "");
    var meter = document.getElementById("meter");
    meter.setAttribute("aria-valuenow", String(u.usedPercent));
    if (!meter.classList.contains("ready")) {
      requestAnimationFrame(function () { meter.classList.add("ready"); });
    }
  }

  function verdict(id, allowed, detail) {
    var el = document.getElementById(id);
    el.className = allowed ? "allowed" : "refused";
    el.textContent = allowed ? "allowed" : "refused" + (detail ? " (" + detail + ")" : "");
  }

  function follow() {
    return invoke("usage.follow").then(function (u) {
      show(u);
      verdict("grant-usage", true);
    }, function (err) {
      verdict("grant-usage", false, err && err.code);
    });
  }

  window.vitra.on("usage.update", show);
  follow();
  // Not granted: the gateway refuses it, and the refusal is audited.
  invoke("clipboard.read").then(function () {
    verdict("grant-clipboard", true);
  }, function (err) {
    verdict("grant-clipboard", false, err && err.code);
  });

  document.getElementById("refresh").addEventListener("click", follow);
  document.getElementById("close").addEventListener("click", function () {
    invoke("panel.close");
  });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      invoke("panel.close");
    }
  });
})();
