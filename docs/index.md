---
layout: home

hero:
  name: Vitra
  text: Desktop apps in Go with a web frontend
  tagline: The page is untrusted. It gets exactly the authority you grant, for the window it runs in, at the origin it runs at. Nothing more.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: Security model
      link: /security
    - theme: alt
      text: GitHub
      link: https://github.com/klarlabs-studio/vitra

features:
  - title: Explicit commands
    details: Only Go functions you register are callable. Inputs are decoded strictly into your types, and a TypeScript client is generated from them.
  - title: Narrow grants
    details: A grant names windows, origins, and permissions. Path-scoped permissions take allow and deny patterns. A new window has no authority at all.
  - title: Identity from the host
    details: The native bridge stamps every call with the window it came from and that window's origin. A per-window sender token keeps frames from forging calls.
  - title: Every refusal is audited
    details: Denials carry a deterministic code and reason. Forged bridge messages and blocked navigations are recorded too.
  - title: Native on three platforms
    details: WebKitGTK on Linux, WKWebView on macOS, WebView2 on Windows. The kernel is standard library only.
  - title: Ship it
    details: Packages for deb, rpm, snap, flatpak, AppImage, MSI, NSIS, .app and .dmg, plus ed25519-signed updates with expiry.
---

## See it work

![The notes demo: a note is edited and saved, then nine attacks from the page are each refused and shown in the live audit log](./assets/notes-demo.gif)

[`example/notes`](/guide/examples#notes) is a Markdown notes app whose window may read one folder and write Markdown files in it. Its **Try to break it** panel attacks the app from the page, and the audit log shows each attempt refused, with the rule that refused it.

```bash
git clone https://github.com/klarlabs-studio/vitra && cd vitra
make notes
```
