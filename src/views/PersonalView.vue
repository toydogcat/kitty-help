<script setup lang="ts">
import ClipBoard from '../components/ClipBoard.vue';
const props = defineProps<{ deviceId: string }>();
</script>

<template>
  <div class="personal-view">
    <ClipBoard :device-id="props.deviceId" />
  </div>
</template>

<style scoped>
.personal-view {
  animation: fadeIn 0.5s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
