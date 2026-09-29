import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);
const root = document.getElementById("app")!;
root.innerHTML = `
  <h1>Vitra</h1>
  <p>Vite + TypeScript starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
  <button id="greet">demo.greet</button>
  <button id="open">dialog.open</button>
  <button id="clip">clipboard.read</button>
  <button id="browser">browser.open</button>
  <button id="os">os.info</button>
  <button id="notify">notifications.show</button>
  <pre id="out"></pre>
`;

const out = document.getElementById("out")!;
document.getElementById("greet")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.demoGreet("Vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("open")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.dialogOpen(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("clip")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.clipboardRead(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("browser")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.browserOpen("https://go.klarlabs.de/vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("os")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.osInfo(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("notify")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
