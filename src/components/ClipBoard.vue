<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService, socket } from '../services/api';
import SnippetExplorer from './SnippetExplorer.vue';
import CountdownTimer from './CountdownTimer.vue';
import Calculator from './Calculator.vue';

const props = defineProps<{
  deviceId: string;
}>();

const currentUser = ref<any>(null);
const commonText = ref<any>(null);
const commonImage = ref<any>(null);
const commonHistory = ref<any[]>([]);
const inputText = ref('');
const isUploading = ref(false);
const isRecording = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

// Speech Recognition Setup
const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
const recognition = SpeechRecognition ? new SpeechRecognition() : null;
if (recognition) {
  recognition.lang = 'zh-TW';
  recognition.continuous = false;
  recognition.interimResults = false;
  
  recognition.onresult = (event: any) => {
    const transcript = event.results[0][0].transcript;
    inputText.value += transcript;
    isRecording.value = false;
  };
  
  recognition.onend = () => {
    isRecording.value = false;
  };
}

onMounted(async () => {
  // 1. Get device/user info
  try {
    const devices = await apiService.getDevices();
    const currentDevice = devices.find((d: any) => d.id === props.deviceId);
    if (currentDevice && currentDevice.user_id) {
      const users = await apiService.getUsers();
      currentUser.value = users.find((u: any) => u.id === currentDevice.user_id);
    }
  } catch (err) {
    console.error("Init error:", err);
  }

  // 2. Fetch Common State & History
  try {
    const [state, history] = await Promise.all([
      apiService.getCommonState(),
      apiService.getCommonHistory()
    ]);
    commonText.value = state.text;
    commonImage.value = state.image;
    commonHistory.value = history;
  } catch (err) {
    console.error("Fetch common state/history failed:", err);
  }

  // 3. Socket Listeners
  socket.on('commonUpdate', (updated) => {
    if (updated.key === 'text') commonText.value = updated;
    if (updated.key === 'image') commonImage.value = updated;
  });

  socket.on('commonHistoryUpdate', (history) => {
    commonHistory.value = history;
  });
});

const updateText = async () => {
  if (!inputText.value.trim()) return;
  try {
    await apiService.updateCommonState('text', {
      content: inputText.value,
      userId: currentUser.value?.id
    });
    inputText.value = '';
  } catch (err) {
    alert("Update text failed");
  }
};

const handleImageUpload = async (e: Event) => {
  const target = e.target as HTMLInputElement;
  if (target.files && target.files[0]) {
    isUploading.value = true;
    try {
      const uploadRes = await apiService.uploadFile(target.files[0]);
      await apiService.updateCommonState('image', {
        fileUrl: uploadRes.url,
        fileName: target.files[0].name,
        userId: currentUser.value?.id
      });
    } catch (err) {
      alert("Upload image failed");
    } finally {
      isUploading.value = false;
      if (fileInput.value) fileInput.value.value = '';
    }
  }
};

const toggleRecording = () => {
  if (!recognition) {
    alert("Speech recognition not supported in this browser.");
    return;
  }
  if (isRecording.value) {
    recognition.stop();
  } else {
    isRecording.value = true;
    recognition.start();
  }
};

const copyToClipboard = (text: string) => {
  if (!text) return;
  navigator.clipboard.writeText(text);
};

const convertToPng = (blob: Blob): Promise<Blob> => {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext('2d');
      ctx?.drawImage(img, 0, 0);
      canvas.toBlob((result) => {
        if (result) resolve(result);
        else reject(new Error("Canvas conversion failed"));
      }, 'image/png');
      URL.revokeObjectURL(img.src);
    };
    img.onerror = reject;
    img.src = URL.createObjectURL(blob);
  });
};

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

const formatUrl = (url: string) => {
  if (!url) return '';
  if (url.startsWith('http')) return url;
  return `${API_URL}${url}`;
};

const copyImageToClipboard = async (url: string) => {
  const fullUrl = formatUrl(url);
  if (!fullUrl) return;
  try {
    // Add ngrok-skip-browser-warning to bypass interstitial page
    const response = await fetch(fullUrl, {
      headers: { 'ngrok-skip-browser-warning': 'any' }
    });
    let blob = await response.blob();
    
    // Clipboard API usually only supports PNG for writing
    if (blob.type !== 'image/png') {
      try {
        blob = await convertToPng(blob);
      } catch (e) {
        console.warn("PNG conversion failed, trying direct write", e);
      }
    }
    
    const item = new ClipboardItem({ [blob.type]: blob });
    await navigator.clipboard.write([item]);
    alert("圖片已複製到剪貼簿！(Copied as PNG)");
  } catch (err) {
    console.error("Failed to copy image:", err);
    navigator.clipboard.writeText(url);
    alert("複製圖片失敗，改為複製連結。");
  }
};
</script>

<template>
  <div class="common-clipboard">
    <!-- User Info -->
    <div v-if="currentUser" class="user-banner">
      👋 Hi, <strong>{{ currentUser.name }}</strong> 
      <code class="user-id" title="Current Device ID">Device: {{ deviceId.substring(0, 8) }}</code>
    </div>

    <!-- New: Tools Section (Timer & Calculator) -->
    <div v-if="currentUser" class="tools-grid">
      <div class="tool-item timer-section">
        <CountdownTimer />
      </div>
      <div class="tool-item calculator-section">
        <Calculator />
      </div>
    </div>

    <!-- Phase 3: Personal Snippets Section -->
    <div v-if="currentUser" class="snippets-section">
      <h3>📚 個人剪貼簿 (Personal Hierarchical Board)</h3>
      <SnippetExplorer :user-id="currentUser.id" />
    </div>
    <div v-else class="snippets-placeholder card">
      <p>💡 <strong>提示：</strong> 目前裝置尚未連結至使用者，因此隱藏了個人剪貼簿。</p>
      <p class="hint">管理員請至上方「Admin Dashboard」中的「Approved Devices」將此裝置分配給使用者（例如：Toby），即可看到個人 Board。</p>
    </div>

    <!-- Main Layout -->
    <div class="main-layout">
      <!-- Left Column: Image -->
      <div class="card shared-card image-card">
        <h3>🖼️ 共同圖 (Common Image)</h3>
        <div class="content-box">
          <div v-if="commonImage?.file_url" class="image-wrapper">
            <img :src="formatUrl(commonImage.file_url)" alt="Shared" />
            <div class="copy-actions">
              <button @click="copyImageToClipboard(commonImage.file_url)" class="copy-float primary">🖼️ 複製圖片</button>
              <button @click="copyToClipboard(commonImage.file_url)" class="copy-float">🔗 複製連結</button>
            </div>
          </div>
          <div v-else class="empty-placeholder">尚未上傳圖片</div>
        </div>
        <div class="card-footer">
          <input type="file" ref="fileInput" @change="handleImageUpload" accept="image/*" id="img-up" class="hidden" />
          <label for="img-up" class="action-btn upload">
            {{ isUploading ? '上傳中...' : '📤 更換圖片' }}
          </label>
        </div>
      </div>

      <!-- Right Column: Text -->
      <div class="card shared-card text-card">
        <h3>📝 共同字 (Common Text)</h3>
        <div class="content-box">
          <pre v-if="commonText?.content">{{ commonText.content }}</pre>
          <div v-else class="empty-placeholder">尚無共用文字</div>
          <button v-if="commonText?.content" @click="copyToClipboard(commonText.content)" class="copy-float">📋 複製文字</button>
        </div>
        
        <div class="input-container">
          <div class="input-wrapper">
            <textarea v-model="inputText" placeholder="輸入新文字以取代現有內容..." @keydown.enter.ctrl="updateText"></textarea>
            <button @click="toggleRecording" class="mic-btn" :class="{ recording: isRecording }" title="語音輸入">
              🎤
            </button>
          </div>
          <button @click="updateText" :disabled="!inputText.trim()" class="action-btn send">
            🚀 更新內容
          </button>
        </div>

        <!-- NEW: Text History Section -->
        <div class="history-section" v-if="commonHistory.length > 0">
          <h4>🕒 歷史紀錄 (最近 10 筆)</h4>
          <div class="history-list">
            <div v-for="item in commonHistory" :key="item.id" class="history-item" @click="copyToClipboard(item.content)">
              <div class="history-content">{{ item.content }}</div>
              <div class="history-meta">
                <span class="history-user">{{ item.user_name || '系統' }}</span>
                <span class="history-time">{{ new Date(item.created_at).toLocaleTimeString() }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
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

.user-banner {
  text-align: left;
  font-size: 1.1rem;
  opacity: 0.8;
  margin-bottom: -0.5rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.user-id {
  font-family: monospace;
  background: rgba(255, 255, 255, 0.1);
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  color: var(--secondary-color);
  border: 1px solid rgba(255, 255, 255, 0.05);
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
