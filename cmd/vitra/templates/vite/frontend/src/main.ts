import "./style.css";
import { createClient, type VitraInvoker } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: VitraInvoker };
  }
}

function describe(err: unknown): string {
  const e = err as { code?: string; message?: string };
  return (e.code ? e.code + ": " : "") + (e.message ?? String(err));
}

const client = createClient(window.vitra.invoke);

document.getElementById("app")!.innerHTML = `
  <main>
    <h1>Hello from Vitra</h1>
    <p>This page can call one Go command, <code>greet</code>, because main.go grants it. Everything else is refused.</p>
    <form id="form">
      <input id="name" placeholder="Your name" aria-label="Your name" autocomplete="off">
      <button>Greet</button>
    </form>
    <output id="out" aria-live="polite"></output>
  </main>
`;

const out = document.getElementById("out")!;
document.getElementById("form")!.addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const name = (document.getElementById("name") as HTMLInputElement).value;
  try {
    out.className = "";
    out.textContent = (await client.greet({ name })).message;
  } catch (err) {
    out.className = "error";
    out.textContent = describe(err);
  }
});
