import { useState } from "react";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

export function App() {
  const [out, setOut] = useState("");
  const run = async (label: string, fn: () => Promise<unknown>) => {
    try {
      setOut(JSON.stringify(await fn(), null, 2));
    } catch (e) {
      setOut(String(e));
    }
  };
  return (
    <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
      <h1>Vitra</h1>
      <p>Vite + React starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button onClick={() => run("greet", () => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run("open", () => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run("clip", () => client.clipboardRead())}>clipboard.read</button>{" "}
      <button onClick={() => run("browser", () => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
      <button onClick={() => run("os", () => client.osInfo())}>os.info</button>{" "}
      <button onClick={() => run("notify", () => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>{out}</pre>
    </div>
  );
}
