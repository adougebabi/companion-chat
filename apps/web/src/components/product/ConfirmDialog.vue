<script setup lang="ts">
import { Dialog, DialogTrigger, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
withDefaults(defineProps<{ open: boolean; title: string; description: string; busy?: boolean; confirmLabel?: string; destructive?: boolean }>(), { confirmLabel: "确认" });
const emit = defineEmits<{ "update:open": [value: boolean]; confirm: [] }>();
</script>
<template>
  <Dialog :open="open" @update:open="value => { if (!busy) emit('update:open', value); }">
    <DialogTrigger v-if="$slots.trigger" as-child><slot name="trigger" /></DialogTrigger>
    <DialogContent class="confirmation-dialog" :show-close-button="false" role="alertdialog">
      <DialogHeader><DialogTitle>{{ title }}</DialogTitle><DialogDescription>{{ description }}</DialogDescription></DialogHeader>
      <slot />
      <DialogFooter><Button variant="outline" type="button" :disabled="busy" @click="emit('update:open', false)">取消</Button><Button :variant="destructive ? 'destructive' : 'default'" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? '正在处理…' : confirmLabel }}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
</template>
