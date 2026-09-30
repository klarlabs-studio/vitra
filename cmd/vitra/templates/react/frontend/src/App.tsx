import { useState, type FormEvent } from "react";
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

export function App() {
  const [name, setName] = useState("");
  const [out, setOut] = useState({ text: "", error: false });

  async function greet(ev: FormEvent) {
    ev.preventDefault();
    try {
      setOut({ text: (await client.greet({ name })).message, error: false });
    } catch (err) {
      setOut({ text: describe(err), error: true });
    }
  }

  return (
    <main>
      <h1>Hello from Vitra</h1>
      <p>
        This page can call one Go command, <code>greet</code>, because main.go grants it. Everything else is refused.
      </p>
      <form onSubmit={greet}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" aria-label="Your name" autoComplete="off" />
        <button>Greet</button>
      </form>
      <output className={out.error ? "error" : ""} aria-live="polite">{out.text}</output>
    </main>
  );
}
