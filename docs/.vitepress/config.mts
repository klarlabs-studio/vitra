import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Vitra",
  description: "Secure desktop apps with Go and a web frontend. The page gets only the authority you grant.",
  base: "/vitra/",
  cleanUrls: true,
  lastUpdated: true,
  // Phase spike notes are working documents, not user docs.
  srcExclude: ["spikes/**", "README.md"],
  head: [["meta", { name: "theme-color", content: "#2f5bd3" }]],
  themeConfig: {
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Security", link: "/security" },
      { text: "Reference", link: "/reference/cli" },
      { text: "API", link: "https://pkg.go.dev/go.klarlabs.de/vitra" },
      { text: "Changelog", link: "https://github.com/klarlabs-studio/vitra/blob/main/CHANGELOG.md" },
    ],
    sidebar: [
      {
        text: "Introduction",
        items: [
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "How Vitra works", link: "/guide/concepts" },
          { text: "Examples", link: "/guide/examples" },
        ],
      },
      {
        text: "Building apps",
        items: [
          { text: "Commands", link: "/guide/commands" },
          { text: "Grants and path scopes", link: "/guide/grants" },
          { text: "The frontend", link: "/guide/frontend" },
          { text: "Events", link: "/guide/events" },
          { text: "Desktop features", link: "/guide/desktop" },
        ],
      },
      {
        text: "Shipping",
        items: [
          { text: "Packaging", link: "/guide/packaging" },
          { text: "Signed updates", link: "/guide/updates" },
          { text: "Audit and policy", link: "/guide/audit-policy" },
        ],
      },
      {
        text: "Background",
        items: [
          { text: "Security model", link: "/security" },
          { text: "Architecture", link: "/architecture-ddd" },
          { text: "Product intent", link: "/intent" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI", link: "/reference/cli" },
          { text: "Official plugins", link: "/reference/plugins" },
          { text: "Denial codes", link: "/reference/denials" },
        ],
      },
    ],
    search: { provider: "local" },
    socialLinks: [{ icon: "github", link: "https://github.com/klarlabs-studio/vitra" }],
    editLink: {
      pattern: "https://github.com/klarlabs-studio/vitra/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },
    footer: {
      message: "Apache-2.0 licensed.",
      copyright: "© klarlabs",
    },
  },
});
