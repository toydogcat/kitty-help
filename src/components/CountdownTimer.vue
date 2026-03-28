<script setup lang="ts">
import { ref, onUnmounted, computed } from 'vue';

const hours = ref(0);
const minutes = ref(0);
const seconds = ref(0);
const totalSeconds = ref(0);
const remainingSeconds = ref(0);
const isActive = ref(false);
const timer = ref<any>(null);

const notificationPermission = ref(Notification.permission);

const requestPermission = async () => {
  const permission = await Notification.requestPermission();
  notificationPermission.value = permission;
};

const playBeep = () => {
  try {
    const ctx = new (window.AudioContext || (window as any).webkitAudioContext)();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.type = 'sine';
    osc.frequency.setValueAtTime(880, ctx.currentTime);
    osc.start();
    gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 1);
    osc.stop(ctx.currentTime + 1);
  } catch (e) {
    console.error("Audio failed", e);
  }
};

const notify = () => {
  if (notificationPermission.value === 'granted') {
    new Notification("⏰ Time's Up!", {
      body: "Your countdown has finished.",
      icon: "/favicon.ico"
    });
  }
  playBeep();
};

const startTimer = () => {
  if (isActive.value) {
    clearInterval(timer.value);
    isActive.value = false;
    return;
  }

  if (remainingSeconds.value === 0) {
    totalSeconds.value = hours.value * 3600 + minutes.value * 60 + seconds.value;
    remainingSeconds.value = totalSeconds.value;
  }

  if (remainingSeconds.value <= 0) return;

  isActive.value = true;
  timer.value = setInterval(() => {
    remainingSeconds.value--;
    if (remainingSeconds.value <= 0) {
      clearInterval(timer.value);
      isActive.value = false;
      notify();
    }
  }, 1000);
};

const resetTimer = () => {
  clearInterval(timer.value);
  isActive.value = false;
  remainingSeconds.value = 0;
  hours.value = 0;
  minutes.value = 0;
  seconds.value = 0;
};

onUnmounted(() => {
  clearInterval(timer.value);
});

const formatTime = (s: number) => {
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`;
};

const progress = computed(() => {
  if (totalSeconds.value === 0) return 0;
  return (remainingSeconds.value / totalSeconds.value) * 100;
});
</script>

<template>
  <div class="countdown-timer card">
    <h3>⏰ 倒數計時 (Countdown)</h3>
    
    <div v-if="!isActive && remainingSeconds === 0" class="timer-inputs">
      <div class="input-group">
        <input type="number" v-model="hours" min="0" max="99" />
        <label>時 (Hr)</label>
      </div>
      <div class="input-group">
        <input type="number" v-model="minutes" min="0" max="59" />
        <label>分 (Min)</label>
      </div>
      <div class="input-group">
        <input type="number" v-model="seconds" min="0" max="59" />
        <label>秒 (Sec)</label>
      </div>
    </div>

    <div v-else class="timer-display">
      <div class="time-text">{{ formatTime(remainingSeconds) }}</div>
      <div class="progress-bar">
        <div class="progress-fill" :style="{ width: progress + '%' }"></div>
      </div>
    </div>

    <div class="timer-actions">
      <button @click="startTimer" :class="{ 'stop-btn': isActive, 'start-btn': !isActive }">
        {{ isActive ? '⏹️ 停止 (Stop)' : '▶️ 開始 (Start)' }}
      </button>
      <button @click="resetTimer" class="reset-btn">🔄 重置 (Reset)</button>
    </div>

    <div v-if="notificationPermission !== 'granted'" class="perm-notice">
      <button @click="requestPermission" class="text-btn">🔔 開啟桌面通知提醒</button>
    </div>
  </div>
</template>

<style scoped>
.countdown-timer {
  text-align: center;
  padding: 1.5rem;
  background: rgba(255, 255, 255, 0.05);
  border: 2px solid var(--border-color);
  border-radius: 20px;
  backdrop-filter: blur(10px);
}

.timer-inputs {
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin: 1.5rem 0;
}

.input-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.input-group input {
  width: 60px;
  height: 60px;
  text-align: center;
  font-size: 1.5rem;
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  color: var(--text-color);
  font-weight: bold;
}

.input-group label {
  font-size: 0.75rem;
  opacity: 0.6;
}

.timer-display {
  margin: 2rem 0;
}

.time-text {
  font-size: 3.5rem;
  font-family: monospace;
  font-weight: bold;
  color: var(--primary-color);
  text-shadow: 0 0 20px rgba(var(--primary-rgb), 0.5);
}

.progress-bar {
  width: 100%;
  height: 6px;
  background: rgba(255,255,255,0.1);
  border-radius: 3px;
  margin-top: 1rem;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(to right, var(--primary-color), var(--accent-color));
  transition: width 1s linear;
}

.timer-actions {
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin-top: 1rem;
}

.timer-actions button {
  flex: 1;
  max-width: 150px;
  padding: 0.8rem;
  border-radius: 10px;
  border: none;
  font-weight: bold;
  cursor: pointer;
  transition: all 0.2s ease;
}

.start-btn {
  background: var(--primary-color);
  color: white;
}

.stop-btn {
  background: #ef4444;
  color: white;
}

.reset-btn {
  background: rgba(255,255,255,0.1);
  color: var(--text-color);
  border: 1px solid var(--border-color) !important;
}

.perm-notice {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
}

.text-btn {
  background: none;
  border: none;
  color: var(--secondary-color);
  text-decoration: underline;
  cursor: pointer;
  font-size: 0.85rem;
}
</style>
