# Vitra Menu Bar

A menu bar (tray) app: the free space on a disk next to the tray icon, and
the details in an HTML panel that drops down when you click the icon. It has
no Dock icon, no taskbar entry, and no other window.

```bash
make menubar
# or
CGO_ENABLED=1 go run -tags vitra_native ./example/menubar -path /Volumes/Data
```

- The icon is drawn at run time: a ring with the used share filled in, as a
  macOS template image.
- A left click shows or hides the panel; the menu (right click) has
  **Show Details**, **Refresh**, and **Quit**.
- The status refreshes every 30 seconds and the panel follows it live.

The panel's grant names exactly two permissions:

| Permission | Command |
|---|---|
| `usage.read` | `usage.follow`: the current usage, then updates as events |
| `panel.close` | `panel.close`: hide the panel (also bound to Escape) |

It also tries `clipboard.read`, which no grant names, and shows the refusal.

See [Menu bar apps](https://klarlabs-studio.github.io/vitra/guide/menubar)
for the building blocks.
