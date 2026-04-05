<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService } from '../services/api';
import SnippetExplorer from './SnippetExplorer.vue';

const props = defineProps<{
  deviceId: string;
  isToby?: boolean;
}>();

const currentUser = ref<any>(null);

onMounted(async () => {
  // 1. Get device/user info
  try {
    const users = await apiService.getUsers();
    
    if (props.isToby) {
      // If recognized as Toby via login, find the Toby profile directly
      currentUser.value = users.find((u: any) => u.email === 'toby@family.local' || u.role === 'toby');
    } else {
      // Legacy device identity mapping
      const devices = await apiService.getDevices();
      const currentDevice = devices.find((d: any) => d.id === props.deviceId);
      if (currentDevice && currentDevice.user_id) {
        currentUser.value = users.find((u: any) => u.id === currentDevice.user_id);
      }
    }
  } catch (err) {
    console.error("Init error:", err);
  }
});


</script>

<template>
  <div class="common-clipboard">
    <!-- Phase 3: Personal Snippets Section -->
    <div v-if="currentUser || props.isToby" class="snippets-section">
      <h3>📚 個人剪貼簿 (Personal Board)</h3>
      <SnippetExplorer :user-id="currentUser?.id || ''" />
    </div>
    <div v-else class="snippets-placeholder card">
      <p>💡 <strong>提示：</strong> 目前裝置尚未連結至使用者，因此隱藏了個人剪貼簿。</p>
      <p class="hint">管理員請至上方「Admin Dashboard」中的「Approved Devices」將此裝置分配給使用者（例如：Toby），即可看到個人 Board。</p>
    </div>
  </div>
</template>

<style scoped>
/* ... (existing styles) ... */
.common-clipboard {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}


.tools-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 2rem;
  align-items: start;
}

.timer-section {
  flex: 1;
  min-width: 320px;
}

.calculator-section {
  flex: 0 0 320px;
}

@media (max-width: 768px) {
  .tools-grid {
    flex-direction: column;
  }
  .calculator-section {
    flex: 1;
    width: 100%;
  }
}

.snippets-section {
  text-align: left;
}

.calendar-section {
  width: 100%;
}

.snippets-section h3 {
  margin-bottom: 1rem;
  color: var(--secondary-color);
}


.snippets-placeholder {
  text-align: left;
  padding: 1.5rem;
  background: rgba(255, 255, 255, 0.05);
  border: 1px dashed var(--border-color);
  opacity: 0.8;
}

.snippets-placeholder .hint {
  margin-top: 0.5rem;
  font-size: 0.85rem;
  color: var(--secondary-color);
}

.main-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
}

@media (max-width: 768px) {
  .main-layout {
    grid-template-columns: 1fr;
  }
}

.shared-card {
  display: flex;
  flex-direction: column;
  height: 650px; /* Increased height to accommodate history */
  background: var(--card-bg);
  border: 2px solid var(--border-color);
  overflow: hidden;
  position: relative;
}

.shared-card h3 {
  padding: 1rem;
  margin: 0;
  background: rgba(0,0,0,0.1);
  font-size: 1.1rem;
  border-bottom: 1px solid var(--border-color);
}

.content-box {
  flex: 0 0 auto; /* Don't grow, keep fixed if possible */
  min-height: 150px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  position: relative;
  overflow: auto;
}

.image-wrapper {
  max-width: 100%;
  max-height: 100%;
  display: flex;
  justify-content: center;
  position: relative;
}

.image-wrapper img {
  max-width: 100%;
  max-height: 350px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.2);
}

.copy-actions {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}


.copy-float {
  background: rgba(0,0,0,0.6);
  color: white;
  border: 1px solid rgba(255,255,255,0.3);
  padding: 0.4rem 0.8rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  backdrop-filter: blur(4px);
  z-index: 10;
}

.copy-float.primary {
  background: var(--primary-color);
  border-color: transparent;
}

.text-card pre {
  width: 100%;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 1.25rem;
  line-height: 1.6;
  text-align: left;
  margin: 0;
  font-family: inherit;
}

.empty-placeholder {
  opacity: 0.3;
  font-style: italic;
}

.input-container {
  padding: 1rem;
  border-top: 1px solid var(--border-color);
  background: rgba(0,0,0,0.05);
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

.input-wrapper {
  position: relative;
  display: flex;
}

textarea {
  flex: 1;
  height: 60px;
  padding: 0.8rem;
  padding-right: 3rem;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  background: var(--card-bg);
  color: var(--text-color);
  font-size: 0.95rem;
  resize: none;
}

.mic-btn {
  position: absolute;
  right: 0.75rem;
  bottom: 0.75rem;
  background: transparent;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  filter: grayscale(1);
}

.mic-btn.recording {
  filter: none;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.7; }
  100% { transform: scale(1); opacity: 1; }
}

.action-btn {
  width: 100%;
  padding: 0.8rem;
  border-radius: 8px;
  font-weight: bold;
  cursor: pointer;
  border: none;
}

.upload { background: var(--accent-color); color: white; display: inline-block; text-align: center; }
.send { background: var(--primary-color); color: white; }
.send:disabled { opacity: 0.5; cursor: not-allowed; }
.hidden { display: none; }

/* NEW: History Styles */
.history-section {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 0.5rem 1rem 1rem;
  background: rgba(0,0,0,0.2);
  border-top: 1px solid var(--border-color);
}

.history-section h4 {
  margin: 0.5rem 0;
  font-size: 0.9rem;
  opacity: 0.7;
}

.history-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding-right: 0.5rem;
}

.history-item {
  background: rgba(255,255,255,0.05);
  padding: 0.75rem;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.history-item:hover {
  background: rgba(255,255,255,0.1);
  border-color: var(--primary-color);
  transform: translateY(-2px);
}

.history-content {
  font-size: 0.95rem;
  margin-bottom: 0.25rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-align: left;
}

.history-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  opacity: 0.5;
}

.history-user {
  color: var(--secondary-color);
}
</style>
