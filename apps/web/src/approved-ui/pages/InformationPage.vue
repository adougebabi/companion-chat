<script setup>
import {useWorkspace} from '../workspace.mjs';
defineProps({information:{type:Object,required:true}});
const w=useWorkspace();
</script>
<template><section class="card stack"><h2>{{information.title}}</h2><p>{{information.body}}</p><button class="button" @click="w.navigate(information.page)">{{information.label}}</button></section></template>
