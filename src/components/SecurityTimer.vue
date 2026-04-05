<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
  userId: string;
  deviceId: string;
}>();

const emit = defineEmits(['expired', 'refresh']);

const remainingSeconds = ref(0);
let timerInterval: any = null;
let syncInterval: any = null;

const formatTime = computed(() => {
  const m = Math.floor(remainingSeconds.value / 60);
  const s = remainingSeconds.value % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
});

const isUrgent = computed(() => remainingSeconds.value < 60);

const fetchStatus = async () => {
  try {
    const res = await apiService.getSecurityStatus(props.userId, props.deviceId);
    if (res.status === 'granted') {
      remainingSeconds.value = res.remainingSeconds;
      if (!timerInterval) startCountdown();
    } else {
      remainingSeconds.value = 0;
      stopCountdown();
      emit('expired');
    }
  } catch (err) {
    console.error('Failed to sync security timer:', err);
  }
};

const startCountdown = () => {
  if (timerInterval) clearInterval(timerInterval);
  timerInterval = setInterval(() => {
    if (remainingSeconds.value > 0) {
      remainingSeconds.value--;
    } else {
      stopCountdown();
      emit('expired');
    }
  }, 1000);
};

const stopCountdown = () => {
  if (timerInterval) clearInterval(timerInterval);
  timerInterval = null;
};

onMounted(() => {
  fetchStatus();
  // Sync with server every 30 seconds to stay accurate
  syncInterval = setInterval(fetchStatus, 30000);
});

onUnmounted(() => {
  stopCountdown();
  if (syncInterval) clearInterval(syncInterval);
});

defineExpose({
  refresh: fetchStatus
});
</script>

<template>
  <div v-if="remainingSeconds > 0" class="security-timer-pill" :class="{ urgent: isUrgent }">
    <div class="shield-icon">🛡️</div>
    <div class="timer-content">
      <span class="label">安全授權中</span>
      <span class="countdown">{{ formatTime }}</span>
    </div>
    <button class="refresh-btn" @click="fetchStatus" title="同步剩餘時間">
      <span class="sync-icon">🔄</span>
    </button>
  </div>
</template>

<style scoped>
.security-timer-pill {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.2);
  padding: 6px 16px;
  border-radius: 100px;
  backdrop-filter: blur(10px);
  color: #10b981;
  font-family: 'Inter', sans-serif;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
  animation: slide-in 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}

.security-timer-pill.urgent {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.3);
  color: #ef4444;
  animation: pulse 1s infinite alternate;
}

.timer-content {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}

.label {
  font-size: 0.65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.8;
}

.countdown {
  font-size: 1.1rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.5px;
}

.refresh-btn {
  background: transparent;
  border: none;
  color: inherit;
  padding: 4px;
  cursor: pointer;
  opacity: 0.5;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.refresh-btn:hover {
  opacity: 1;
  transform: rotate(180deg);
}

.sync-icon {
  font-size: 0.8rem;
}

@keyframes slide-in {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes pulse {
  from { box-shadow: 0 0 0 rgba(239, 68, 68, 0); }
  to { box-shadow: 0 0 15px rgba(239, 68, 68, 0.4); }
}
</style>
