<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue';
import base from '../styles/v3-proto.css?inline';
import life from '../styles/v3-life-review.css?inline';
import world from '../styles/v3-world-review.css?inline';
import lifeRegression from '../styles/v3-life-regression.css?inline';
import reading from '../styles/v3-reading-regression.css?inline';
import media from '../styles/v3-media-regression.css?inline';
import settings from '../styles/v3-settings-regression.css?inline';
import uxLife from '../styles/v3-ux-report-life.css?inline';
import creation from '../styles/v3-ux-report-creation.css?inline';
import uxSettings from '../styles/v3-ux-report-settings.css?inline';
import chat from '../styles/v3-chat-summary-review.css?inline';
const host=ref<HTMLElement|null>(null);let dispose:undefined|(()=>void),active=true,style:HTMLStyleElement;
onMounted(async()=>{style=document.createElement('style');style.dataset.legacySurface='';style.textContent=[base,life,world,lifeRegression,reading,media,settings,uxLife,creation,uxSettings,chat].join('\n');document.head.append(style);const {mountProduct}=await import('../v3/proto/app');if(!active)return;const cleanup=await mountProduct(host.value);if(active)dispose=cleanup;else cleanup?.();});
onBeforeUnmount(()=>{active=false;dispose?.();style?.remove();});
</script>
<template><div id="v3-root" ref="host" /></template>
