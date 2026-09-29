<script setup lang="ts">
import { ref } from "vue";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);
const out = ref("");

async function run(fn: () => Promise<unknown>) {
  try {
    out.value = JSON.stringify(await fn(), null, 2);
  } catch (e) {
    out.value = String(e);
  }
}
</script>

<template>
  <div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh;">
    <h1>Vitra</h1>
    <p>Vite + Vue starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button @click="run(() => client.demoGreet('Vitra'))">demo.greet</button>
    <button @click="run(() => client.dialogOpen())">dialog.open</button>
    <button @click="run(() => client.clipboardRead())">clipboard.read</button>
    <button @click="run(() => client.browserOpen('https://go.klarlabs.de/vitra'))">browser.open</button>
    <button @click="run(() => client.osInfo())">os.info</button>
    <button @click="run(() => client.notificationsShow({ title: 'Vitra', body: 'Hello from scaffold' }))">notifications.show</button>
    <pre>{{ out }}</pre>
  </div>
</template>
