// A small Markdown renderer for the preview: headings, emphasis, inline code,
// code blocks, lists, and paragraphs. Text is escaped before any markup is
// added, so note content can never inject HTML.
(function () {
  "use strict";

  function escape(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function inline(s) {
    return escape(s)
      .replace(/`([^`]+)`/g, "<code>$1</code>")
      .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
      .replace(/(^|[^*])\*([^*\s][^*]*)\*/g, "$1<em>$2</em>");
  }

  function render(md) {
    const out = [];
    let list = null;
    let para = [];
    let code = null;
    const flushPara = () => {
      if (para.length) out.push("<p>" + inline(para.join(" ")) + "</p>");
      para = [];
    };
    const flushList = () => {
      if (list) out.push("<" + list.tag + ">" + list.items.join("") + "</" + list.tag + ">");
      list = null;
    };
    for (const line of md.split(/\r?\n/)) {
      if (code !== null) {
        if (/^```/.test(line)) {
          out.push("<pre><code>" + escape(code.join("\n")) + "</code></pre>");
          code = null;
        } else {
          code.push(line);
        }
        continue;
      }
      if (/^```/.test(line)) { flushPara(); flushList(); code = []; continue; }
      const h = /^(#{1,4})\s+(.*)$/.exec(line);
      if (h) { flushPara(); flushList(); out.push("<h" + h[1].length + ">" + inline(h[2]) + "</h" + h[1].length + ">"); continue; }
      const li = /^\s*([-*]|\d+\.)\s+(.*)$/.exec(line);
      if (li) {
        flushPara();
        const tag = /\d/.test(li[1]) ? "ol" : "ul";
        if (!list || list.tag !== tag) { flushList(); list = { tag: tag, items: [] }; }
        list.items.push("<li>" + inline(li[2]) + "</li>");
        continue;
      }
      if (line.trim() === "") { flushPara(); flushList(); continue; }
      flushList();
      para.push(line.trim());
    }
    if (code !== null) out.push("<pre><code>" + escape(code.join("\n")) + "</code></pre>");
    flushPara();
    flushList();
    return out.join("\n");
  }

  window.renderMarkdown = render;
})();
