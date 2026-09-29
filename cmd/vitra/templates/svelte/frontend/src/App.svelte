<script lang="ts">
  import { createClient } from "../vitra-client";

  declare global {
    interface Window {
      vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
    }
  }

  const client = createClient(window.vitra.invoke);
  let out = $state("");

  async function run(fn: () => Promise<unknown>) {
    try {
      out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      out = String(e);
    }
  }
</script>

<div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh;">
  <h1>Vitra</h1>
  <p>Vite + Svelte starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
  <button onclick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>
  <button onclick={() => run(() => client.dialogOpen())}>dialog.open</button>
  <button onclick={() => run(() => client.clipboardRead())}>clipboard.read</button>
  <button onclick={() => run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>
  <button onclick={() => run(() => client.osInfo())}>os.info</button>
  <button onclick={() => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
  <pre>{out}</pre>
</div>
