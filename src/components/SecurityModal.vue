<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
  userId: string;
  deviceId: string;
}>();

const emit = defineEmits(['verified', 'close']);

const token = ref('');
const expiresAt = ref<string | null>(null);
const lineVerified = ref(false);
const discordVerified = ref(false);
const loading = ref(true);
const error = ref<string | null>(null);

let pollInterval: any = null;

const startChallenge = async () => {
  try {
    loading.value = true;
    const res = await apiService.requestSecurityChallenge(props.userId, props.deviceId);
    token.value = res.token;
    expiresAt.value = res.expiresAt;
    loading.value = false;
    startPolling();
  } catch (err) {
    error.value = '無法起始驗證，請稍後再試。';
    loading.value = false;
  }
};

const checkStatus = async () => {
  try {
    const res = await apiService.getSecurityStatus(props.userId, props.deviceId, token.value);
    lineVerified.value = res.lineVerified;
    discordVerified.value = res.discordVerified;
    
    if (res.status === 'granted') {
      clearInterval(pollInterval);
      emit('verified');
    }
  } catch (err) {
    console.error('Polling status failed:', err);
  }
};

const startPolling = () => {
  if (pollInterval) clearInterval(pollInterval);
  pollInterval = setInterval(checkStatus, 2500);
};

onMounted(() => {
  startChallenge();
});

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval);
});
</script>

<template>
  <div class="security-modal-overlay" @click.self="emit('close')">
    <div class="security-modal-content">
      <div class="header">
        <span class="icon">🛡️</span>
        <h3>高強度安全性驗證</h3>
        <p>此書籤受到密碼保護，請完成跨平台雙重驗證</p>
      </div>

      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
        <p>正在生成挑戰碼...</p>
      </div>

      <div v-else-if="error" class="error-state">
        <p>{{ error }}</p>
        <button @click="startChallenge" class="retry-btn">重試</button>
      </div>

      <div v-else class="challenge-body">
        <div class="token-section">
          <label>請在通訊軟體輸入：</label>
          <div class="token-display">
            <span v-for="(char, index) in token" :key="index" class="token-char">{{ char }}</span>
          </div>
        </div>

        <div class="steps-section">
          <div class="step" :class="{ active: !lineVerified, completed: lineVerified }">
            <div class="step-icon">{{ lineVerified ? '✅' : '1' }}</div>
            <div class="step-text">
              <h4>Line 機器人驗證</h4>
              <p>{{ lineVerified ? '已完成驗證' : '請輸入 /verify ' + token }}</p>
            </div>
          </div>

          <div class="step-connector"></div>

          <div class="step" :class="{ active: lineVerified && !discordVerified, completed: discordVerified, locked: !lineVerified }">
            <div class="step-icon">{{ discordVerified ? '✅' : '2' }}</div>
            <div class="step-text">
              <h4>Discord 機器人驗證</h4>
              <p>{{ discordVerified ? '已完成驗證' : (lineVerified ? '請輸入 /verify ' + token : '等待 Line 驗證完成...') }}</p>
            </div>
          </div>
        </div>

        <div class="footer-note">
          <span class="clock">⏳</span> 驗證碼將在 10 分鐘後過期
        </div>
      </div>

      <button class="close-btn" @click="emit('close')">取消</button>
    </div>
  </div>
</template>

<style scoped>
.security-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.85);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  padding: 20px;
}

.security-modal-content {
  background: rgba(30, 30, 35, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  width: 100%;
  max-width: 450px;
  padding: 32px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  color: white;
  text-align: center;
  position: relative;
  overflow: hidden;
}

.security-modal-content::before {
  content: '';
  position: absolute;
  top: -50%;
  left: -50%;
  width: 200%;
  height: 200%;
  background: radial-gradient(circle at center, rgba(100, 100, 255, 0.1) 0%, transparent 70%);
  pointer-events: none;
}

.header h3 {
  font-size: 1.5rem;
  margin: 12px 0 8px;
  background: linear-gradient(135deg, #fff 0%, #aaa 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.header p {
  color: #888;
  font-size: 0.9rem;
}

.token-section {
  margin: 32px 0;
}

.token-section label {
  display: block;
  font-size: 0.85rem;
  color: #6366f1;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin-bottom: 12px;
}

.token-display {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.token-char {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  width: 45px;
  height: 55px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.8rem;
  font-weight: 700;
  font-family: 'JetBrains Mono', monospace;
  color: #fff;
  text-shadow: 0 0 15px rgba(99, 102, 241, 0.5);
}

.steps-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 32px 0;
  text-align: left;
}

.step {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid transparent;
  transition: all 0.3s ease;
}

.step.active {
  background: rgba(99, 102, 241, 0.08);
  border-color: rgba(99, 102, 241, 0.3);
  transform: translateX(5px);
}

.step.completed {
  border-color: rgba(34, 197, 94, 0.3);
}

.step.locked {
  opacity: 0.4;
  filter: grayscale(1);
}

.step-icon {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
}

.active .step-icon {
  background: #6366f1;
  box-shadow: 0 0 15px rgba(99, 102, 241, 0.4);
}

.completed .step-icon {
  background: #22c55e;
}

.step-text h4 {
  margin: 0;
  font-size: 0.95rem;
}

.step-text p {
  margin: 4px 0 0;
  font-size: 0.8rem;
  color: #666;
}

.active .step-text p {
  color: #a5b4fc;
}

.footer-note {
  font-size: 0.85rem;
  color: #666;
  margin-top: 24px;
}

.close-btn {
  margin-top: 24px;
  background: transparent;
  border: none;
  color: #666;
  cursor: pointer;
  font-size: 0.9rem;
  transition: color 0.2s;
}

.close-btn:hover {
  color: #ff4757;
}

.spinner {
  width: 30px;
  height: 30px;
  border: 3px solid rgba(255,255,255,0.1);
  border-top-color: #6366f1;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 40px auto;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
