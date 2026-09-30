<script setup lang="ts">
import { ref } from "vue";
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
const name = ref("");
const out = ref({ text: "", error: false });

async function greet() {
  try {
    out.value = { text: (await client.greet({ name: name.value })).message, error: false };
  } catch (err) {
    out.value = { text: describe(err), error: true };
  }
}
</script>

<template>
  <main>
    <h1>Hello from Vitra</h1>
    <p>This page can call one Go command, <code>greet</code>, because main.go grants it. Everything else is refused.</p>
    <form @submit.prevent="greet">
      <input v-model="name" placeholder="Your name" aria-label="Your name" autocomplete="off" />
      <button>Greet</button>
    </form>
    <output :class="{ error: out.error }" aria-live="polite">{{ out.text }}</output>
  </main>
</template>
