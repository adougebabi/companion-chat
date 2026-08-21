import { computed, ref } from 'vue';
import type { ViewName } from '../components/types';

const currentView = ref<ViewName>('contacts');

export function useAppRouter() {
  function navigate(view: ViewName) {
    currentView.value = view;
  }
  return { currentView: computed(() => currentView.value), navigate };
}

