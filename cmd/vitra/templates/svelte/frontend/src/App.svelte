<script lang="ts">
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
  let name = $state("");
  let out = $state({ text: "", error: false });

  async function greet(ev: SubmitEvent) {
    ev.preventDefault();
    try {
      out = { text: (await client.greet({ name })).message, error: false };
    } catch (err) {
      out = { text: describe(err), error: true };
    }
  }
</script>

<main>
  <h1>Hello from Vitra</h1>
  <p>This page can call one Go command, <code>greet</code>, because main.go grants it. Everything else is refused.</p>
  <form onsubmit={greet}>
    <input bind:value={name} placeholder="Your name" aria-label="Your name" autocomplete="off" />
    <button>Greet</button>
  </form>
  <output class={out.error ? "error" : ""} aria-live="polite">{out.text}</output>
</main>
