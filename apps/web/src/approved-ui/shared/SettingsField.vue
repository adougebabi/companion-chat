<script setup>
import { computed } from 'vue';
import { autonomyActionChoices } from '../../v3/proto/components/configuration-fields.js';
const props=defineProps({field:{type:Object,required:true}});
const actionChoices=computed(()=>{
  const known=new Map(autonomyActionChoices);
  for(const value of Array.isArray(props.field.value)?props.field.value:[])if(!known.has(value))known.set(value,value);
  return [...known];
});
function toggle(value,checked){const selected=new Set(Array.isArray(props.field.value)?props.field.value:[]);if(checked)selected.add(value);else selected.delete(value);props.field.value=[...selected];}
</script>
<template>
  <fieldset v-if="field.type==='action-types'" class="action-type-field">
    <legend>{{field.label}}</legend>
    <label v-for="[value,label] in actionChoices" :key="value" class="check-field"><input type="checkbox" :checked="Array.isArray(field.value)&&field.value.includes(value)" @change="toggle(value,$event.target.checked)"/>{{label}}</label>
  </fieldset>
  <label v-else :class="{'boolean-field':field.type==='checkbox'}">{{field.label}}
    <select :aria-label="field.label" v-if="field.type==='select'" v-model="field.value"><option v-for="option in field.options" :key="option">{{option}}</option></select>
    <textarea :aria-label="field.label" v-else-if="field.type==='json'" :value="String(field.value)" rows="3" @input="field.value=$event.target.value"/>
    <input :aria-label="field.label" v-else-if="field.type==='checkbox'" v-model="field.value" class="switch" type="checkbox"/>
    <input :aria-label="field.label" v-else-if="field.type==='number'" v-model.number="field.value" type="number" :min="field.min" :max="field.max" :step="field._definition?.step||'any'"/>
    <input :aria-label="field.label" v-else v-model="field.value" :type="field.type||'text'"/>
  </label>
</template>
