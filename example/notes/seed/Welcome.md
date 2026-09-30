# Welcome to Vitra Notes

This app is a normal web page running in a native window. What makes it different is what the page is **not** allowed to do.

The Go side granted this window exactly three things:

- **Read** anything in this vault, except the `.private` folder.
- **Write** Markdown files (`*.md`) in this vault, except in `.private`.
- **Read the audit log**, which is the panel on the right.

Everything else is refused before any Go code runs: other folders, other file types, other windows, other origins, remote pages.

## Try it

Open **Try to break it** at the bottom of the audit panel. Each button makes the page attempt something an attacker would, with real calls through the real bridge. Watch the audit log: every attempt shows up with the reason it was refused.

Edits you make here are saved to the vault folder on disk. 
