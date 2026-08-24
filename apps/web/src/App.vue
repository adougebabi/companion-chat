<script setup lang="ts">
import { browserOperations } from "@fluctlight/browser-client";
</script>

<template>
  <main>
    <h1>Fluctlight</h1>
    <p data-testid="platform-contract">{{ browserOperations.length }} browser platform operations available.</p>
  </main>
</template>
