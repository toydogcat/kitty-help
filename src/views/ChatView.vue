<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { apiService } from '../services/api';

const activePlatform = ref('line');
const platforms = [
  { id: 'telegram', name: 'Telegram', icon: '✈️' },
  { id: 'discord', name: 'Discord', icon: '🎮' },
  { id: 'line', name: 'Line', icon: '🟢' }
];

const messages = ref<any[]>([]);
const myStatus = ref({
  telegram: false,
  discord: false,
  line: false
});
const loading = ref(true);

// Search Filters
const searchQuery = ref('');
const startDate = ref('');
const endDate = ref('');

// Card Styling state
const cardBackgrounds = [
  'rgba(255, 255, 255, 0.05)', // Default
  'linear-gradient(135deg, rgba(170, 59, 255, 0.2), rgba(170, 59, 255, 0.05))', // Purple
  'linear-gradient(135deg, rgba(46, 204, 113, 0.2), rgba(46, 204, 113, 0.05))', // Green
  'linear-gradient(135deg, rgba(52, 152, 219, 0.2), rgba(52, 152, 219, 0.05))', // Blue
  'linear-gradient(135deg, rgba(241, 196, 15, 0.2), rgba(241, 196, 15, 0.05))', // Yellow
  'linear-gradient(135deg, rgba(231, 76, 60, 0.2), rgba(231, 76, 60, 0.05))', // Red
];

const fetchMyStatus = async () => {
  try {
    const data = await apiService.getMyBotStatus();
    myStatus.value = data;
  } catch (err) {
    console.error('Failed to fetch bot status:', err);
  }
};

const fetchMessages = async () => {
  loading.value = true;
  try {
    const data = await apiService.getChatLogs(
      activePlatform.value,
      searchQuery.value,
      startDate.value,
      endDate.value
    );
    // Initialize background colors for new messages
    messages.value = data.map((m: any) => ({
      ...m,
      bgIndex: 0,
      isZoomed: false
    }));
  } catch (err) {
    console.error('Failed to fetch messages:', err);
  } finally {
    loading.value = false;
  }
};

onMounted(async () => {
  await fetchMyStatus();
  await fetchMessages();
});

watch([activePlatform, startDate, endDate], () => {
  fetchMessages();
});

// Debounce search
let searchTimeout: any;
watch(searchQuery, () => {
  clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    fetchMessages();
  }, 500);
});

const getStorehouseUrl = (mediaId: string) => {
  return apiService.getStorehouseFileUrl(mediaId, activePlatform.value);
};

const formatDate = (dateStr: string) => {
  const d = new Date(dateStr);
  return d.toLocaleString();
};

const cycleBg = (m: any) => {
  m.bgIndex = (m.bgIndex + 1) % cardBackgrounds.length;
};

const toggleZoom = (m: any) => {
  m.isZoomed = !m.isZoomed;
};
</script>

<template>
  <div class="chat-view">
    <header class="view-header">
      <div class="header-content">
        <h2>💬 Discovery History</h2>
        <p>Browse your cross-platform conversation archives from AI assistants.</p>
      </div>
      
      <div class="platform-tabs">
        <button 
          v-for="p in platforms" 
          :key="p.id"
          @click="activePlatform = p.id"
          :class="['platform-btn', { active: activePlatform === p.id }]"
        >
          <span class="p-icon">{{ p.icon }}</span>
          {{ p.name }}
          <span v-if="myStatus[p.id as keyof typeof myStatus]" class="status-dot linked" title="Linked"></span>
          <span v-else class="status-dot unlinked" title="Not Linked"></span>
        </button>
      </div>
    </header>

    <div class="search-toolbar card">
      <div class="search-input-group">
        <span class="search-icon">🔍</span>
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Search messages..." 
          class="text-input"
        />
      </div>
      
      <div class="date-filters">
        <div class="date-field" :class="{ active: startDate }">
          <label>From</label>
          <input v-model="startDate" type="date" />
        </div>
        <div class="date-field" :class="{ active: endDate }">
          <label>To</label>
          <input v-model="endDate" type="date" />
        </div>
        <button @click="searchQuery = ''; startDate = ''; endDate = '';" class="clear-btn" title="Clear Filters">
          🧹
        </button>
      </div>
    </div>

    <div class="chat-container">
      <div v-if="!myStatus[activePlatform as keyof typeof myStatus]" class="unlinked-notice card">
        <h3>🚫 Platform Not Linked</h3>
        <p>You haven't linked your <strong>{{ activePlatform }}</strong> account yet.</p>
        <div class="instruction">
          <p>To link your account:</p>
          <ol>
            <li v-if="activePlatform === 'telegram'">Find <strong>@super_kitty_help_bot</strong> on Telegram</li>
            <li v-if="activePlatform === 'discord'">Invite <strong>KittyHelp</strong> to your Discord server</li>
            <li v-if="activePlatform === 'line'">Add <strong>KittyHelp</strong> as a friend on LINE</li>
            <li>Send the message: <code>我請求加入</code></li>
            <li>Enter the 8-digit code in the <strong>Home</strong> page's verification portal.</li>
            <li>Wait for AdminToby approval in the dashboard to complete binding.</li>
          </ol>
        </div>
      </div>

      <div v-else-if="loading" class="chat-loading">
        <div class="spinner"></div>
        <p>Scanning archives...</p>
      </div>

      <div v-else-if="messages.length === 0" class="empty-chat">
        <p v-if="searchQuery || startDate || endDate">No results match your filters. Try widening your search!</p>
        <p v-else>No messages found on this platform yet. Start talking to your bot!</p>
      </div>

      <div v-else class="message-feed grid-layout">
        <div 
          v-for="m in messages" 
          :key="m.id" 
          class="message-card" 
          :style="{ background: cardBackgrounds[m.bgIndex] }"
        >
          <div class="card-controls">
            <button @click="cycleBg(m)" class="control-btn" title="Change Background">🎨</button>
          </div>

          <div class="msg-header">
            <span class="sender">{{ m.senderName }}</span>
            <span class="time">{{ formatDate(m.createdAt) }}</span>
          </div>

          <div class="msg-content">
            <template v-if="m.msgType === 'media' && m.mediaId">
              <div 
                class="media-container" 
                :class="{ zoomed: m.isZoomed }"
                @click="toggleZoom(m)"
                v-if="m.mediaType === 'image' || m.mediaType === 'photo' || m.content.includes('[Image]') || m.content.includes('[photo]')"
              >
                <img :src="getStorehouseUrl(m.mediaId)" loading="lazy" />
                <div v-if="!m.isZoomed" class="zoom-hint">🔍 Click to Expand</div>
              </div>
              <div v-else class="file-card">
                <span class="file-icon">{{ m.mediaType === 'video' ? '🎬' : '📎' }}</span>
                <div class="file-info">
                  <span class="file-name">{{ m.mediaType === 'video' ? 'Video Memory' : 'Media Backup' }}</span>
                  <a :href="getStorehouseUrl(m.mediaId)" target="_blank" class="download-link">{{ m.mediaType === 'video' ? 'Watch' : 'View File' }}</a>
                </div>
              </div>
            </template>
            <p v-else class="text-content">{{ m.content }}</p>
          </div>

          <div class="msg-footer" v-if="m.mediaType === 'image' || m.mediaType === 'photo'">
            <span class="type-tag">📸 Image</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Zoom Overlay -->
    <Transition name="fade">
      <div v-if="messages.some(m => m.isZoomed)" class="zoom-overlay" @click="messages.forEach(m => m.isZoomed = false)">
        <span class="close-overlay">✕</span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.chat-view {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
  max-width: 1400px;
  margin: 0 auto;
  padding: 1rem;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  border-bottom: 2px solid rgba(var(--primary-rgb), 0.2);
  padding-bottom: 1.5rem;
}

.header-content h2 { font-size: 1.8rem; margin-bottom: 0.2rem; }
.header-content p { opacity: 0.6; font-size: 0.9rem; }

.platform-tabs {
  display: flex;
  gap: 0.8rem;
}

.platform-btn {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.6rem 1.2rem;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color);
  border-radius: 20px;
  color: var(--text-color);
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s ease;
  position: relative;
}

.platform-btn.active {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
  box-shadow: 0 4px 12px rgba(var(--primary-rgb), 0.3);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  position: absolute;
  top: 5px;
  right: 10px;
}
.status-dot.linked { border-radius: 50%; background: #2ecc71; box-shadow: 0 0 5px #2ecc71; }
.status-dot.unlinked { border-radius: 50%; background: #95a5a6; }

/* Search Toolbar */
.search-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.5rem;
  padding: 1rem 1.5rem;
  background: rgba(var(--primary-rgb), 0.03);
  backdrop-filter: blur(10px);
}

.search-input-group {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.8rem;
  background: rgba(0,0,0,0.2);
  padding: 0.6rem 1rem;
  border-radius: 10px;
  border: 1px solid var(--border-color);
}

.search-input-group input {
  background: transparent;
  border: none;
  color: white;
  width: 100%;
  outline: none;
}

.date-filters {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.date-field {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.3s ease;
}

.date-field.active {
  background: rgba(var(--primary-rgb), 0.1);
  padding: 2px 8px;
  border-radius: 8px;
  box-shadow: 0 0 10px rgba(var(--primary-rgb), 0.3);
}

.date-field label { font-size: 0.75rem; opacity: 0.6; text-transform: uppercase; font-weight: 800; }

.date-field input {
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-color);
  color: white;
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  font-size: 0.85rem;
  outline: none;
  transition: all 0.2s;
}

.date-field input:focus {
  border-color: var(--primary-color);
  box-shadow: 0 0 15px rgba(var(--primary-rgb), 0.5);
  background: rgba(var(--primary-rgb), 0.1);
}

/* Chrome/Safari Calendar Icon glow */
.date-field input::-webkit-calendar-picker-indicator {
  filter: invert(1);
  cursor: pointer;
  background-color: var(--primary-color);
  border-radius: 3px;
  padding: 2px;
  box-shadow: 0 0 8px var(--primary-color);
}

.clear-btn {
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 0.4rem 0.6rem;
  cursor: pointer;
  transition: background 0.2s;
}
.clear-btn:hover { background: rgba(255,255,255,0.1); }

/* Chat Container */
.chat-container {
  min-height: 60vh;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.grid-layout {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  grid-auto-rows: min-content;
  gap: 1.5rem;
  padding: 1rem 0;
}

.message-card {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 1.2rem;
  border-radius: 16px;
  border: 1px solid var(--border-color);
  box-shadow: 0 4px 15px rgba(0,0,0,0.1);
  transition: transform 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275), box-shadow 0.3s;
  height: fit-content;
  overflow: hidden;
}

.message-card:hover {
  transform: translateY(-5px) scale(1.01);
  box-shadow: 0 10px 25px rgba(0,0,0,0.2);
  border-color: rgba(var(--primary-rgb), 0.3);
}

.card-controls {
  position: absolute;
  top: 10px;
  right: 10px;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 5;
}

.message-card:hover .card-controls { opacity: 1; }

.control-btn {
  background: rgba(255,255,255,0.1);
  border: none;
  border-radius: 50%;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-size: 1rem;
  backdrop-filter: blur(5px);
}
.control-btn:hover { background: rgba(255,255,255,0.2); transform: scale(1.1); }

.msg-header {
  display: flex;
  flex-direction: column;
  margin-bottom: 0.8rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid rgba(255,255,255,0.05);
}

.sender { font-weight: 800; color: var(--primary-color); font-size: 0.9rem; }
.time { opacity: 0.4; font-size: 0.75rem; }

.text-content {
  line-height: 1.6;
  white-space: pre-wrap;
  color: rgba(255,255,255,0.9);
  font-size: 0.95rem;
}

.media-container {
  position: relative;
  cursor: zoom-in;
  margin: 0.5rem -0.5rem 0;
  border-radius: 12px;
  overflow: hidden;
  max-height: 200px;
  transition: all 0.4s ease;
  border: 1px solid rgba(255,255,255,0.1);
}

.media-container img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s;
}

.media-container:hover img { transform: scale(1.05); }

.media-container.zoomed {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 90vw;
  max-height: 90vh;
  z-index: 1001;
  cursor: zoom-out;
  box-shadow: 0 0 50px rgba(0,0,0,0.8);
  border: 2px solid var(--primary-color);
  max-width: 1200px;
}

.media-container.zoomed img {
  object-fit: contain;
}

.zoom-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.9);
  z-index: 1000;
  cursor: zoom-out;
}

.close-overlay {
  position: absolute;
  top: 20px;
  right: 20px;
  color: white;
  font-size: 2rem;
  cursor: pointer;
}

.zoom-hint {
  position: absolute;
  bottom: 10px;
  right: 10px;
  background: rgba(0,0,0,0.6);
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.7rem;
  opacity: 0;
  transition: opacity 0.2s;
}

.media-container:hover .zoom-hint { opacity: 1; }

.file-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  background: rgba(0,0,0,0.3);
  padding: 1rem;
  border-radius: 12px;
  border: 1px dashed rgba(var(--primary-rgb), 0.3);
  margin-top: 0.5rem;
}

.file-icon { font-size: 1.5rem; }
.file-info { display: flex; flex-direction: column; gap: 0.2rem; }
.file-name { font-size: 0.9rem; font-weight: 600; }

.download-link {
  color: var(--primary-color);
  text-decoration: none;
  font-size: 0.85rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 0.3rem;
}
.download-link:hover { text-decoration: underline; }

.msg-footer {
  margin-top: auto;
  padding-top: 1rem;
  display: flex;
  justify-content: flex-end;
}

.type-tag {
  font-size: 0.7rem;
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
  padding: 2px 8px;
  border-radius: 20px;
  font-weight: 800;
}

.fade-enter-active, .fade-leave-active { transition: opacity 0.3s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.spinner {
  width: 50px;
  height: 50px;
  border: 4px solid rgba(var(--primary-rgb), 0.1);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 1.5rem;
}

@keyframes spin { to { transform: rotate(360deg); } }

.unlinked-notice {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem;
}

.instruction {
  background: rgba(0,0,0,0.25);
  padding: 2rem;
  border-radius: 16px;
  margin-top: 2rem;
  max-width: 500px;
  text-align: left;
}

.instruction li { margin-bottom: 0.8rem; color: rgba(255,255,255,0.8); }
</style>
