<script setup lang="ts">
import {defineAsyncComponent} from 'vue';
// Fixture code is selected only by an explicit build-time environment flag.
const Product=defineAsyncComponent(import.meta.env.VITE_PRODUCT_MODE==='mock'
 ?()=>import('./approved-ui/reference-fixture.mjs')
 :()=>import('./approved-ui/ApprovedApp.mjs'));
</script>
<template><Product /></template>
