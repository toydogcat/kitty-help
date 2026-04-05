<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import axios from 'axios';

const activePlatform = ref('telegram');
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

const fetchMyStatus = async () => {
  try {
    const res = await axios.get('/api/bot/my-status');
    myStatus.value = res.data;
  } catch (err) {
    console.error('Failed to fetch bot status:', err);
  }
};

const fetchMessages = async () => {
  loading.value = true;
  try {
    const params = new URLSearchParams({
      platform: activePlatform.value,
      q: searchQuery.value,
      startDate: startDate.value,
      endDate: endDate.value
    });
    const res = await axios.get(`/api/chat/logs?${params.toString()}`);
    messages.value = res.data;
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
  return `/api/storehouse/file/${mediaId}`;
};

const formatDate = (dateStr: string) => {
  const d = new Date(dateStr);
  return d.toLocaleString();
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
        <div class="date-field">
          <label>From</label>
          <input v-model="startDate" type="date" />
        </div>
        <div class="date-field">
          <label>To</label>
          <input v-model="endDate" type="date" />
        </div>
        <button @click="searchQuery = ''; startDate = ''; endDate = '';" class="clear-btn" title="Clear Filters">
          🧹
        </button>
      </div>
    </div>

    <div class="chat-container card">
      <div v-if="!myStatus[activePlatform as keyof typeof myStatus]" class="unlinked-notice">
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

      <div v-else class="message-feed">
        <div v-for="m in messages" :key="m.id" class="message-wrapper">
          <div class="message-bubble">
            <div class="msg-header">
              <span class="sender">{{ m.senderName }}</span>
              <span class="time">{{ formatDate(m.createdAt) }}</span>
            </div>
            <div class="msg-content">
              <template v-if="m.msgType === 'media' && m.mediaId">
                <div class="media-container" v-if="m.content.includes('[Image]') || m.content.includes('[photo]') ">
                  <img :src="getStorehouseUrl(m.mediaId)" loading="lazy" />
                </div>
                <div v-else class="file-card">
                  <span class="file-icon">📎</span>
                  <div class="file-info">
                    <span class="file-name">Media Backup</span>
                    <a :href="getStorehouseUrl(m.mediaId)" target="_blank" class="download-link">View File</a>
                  </div>
                </div>
              </template>
              <p v-else class="text-content">{{ m.content }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-view {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
  max-width: 1100px;
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
.status-dot.linked { background: #2ecc71; box-shadow: 0 0 5px #2ecc71; }
.status-dot.unlinked { background: #95a5a6; }

/* Search Toolbar */
.search-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.5rem;
  padding: 1rem 1.5rem;
  background: rgba(var(--primary-rgb), 0.05);
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
}

.date-field label { font-size: 0.75rem; opacity: 0.6; text-transform: uppercase; }

.date-field input {
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-color);
  color: white;
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  font-size: 0.85rem;
  outline: none;
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

.message-feed {
  flex: 1;
  padding: 2rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  overflow-y: auto;
  max-height: 70vh;
  background: radial-gradient(circle at top right, rgba(var(--primary-rgb), 0.05), transparent 400px);
}

.message-wrapper {
  display: flex;
  width: 100%;
  justify-content: center; /* Center the bubbles */
}

.message-bubble {
  background: rgba(255, 255, 255, 0.03);
  padding: 1.2rem;
  border-radius: 20px;
  border: 1px solid var(--border-color);
  width: 100%;
  max-width: 800px; /* Better width for reading */
  box-shadow: 0 4px 20px rgba(0,0,0,0.15);
  transition: transform 0.2s ease;
}

.message-bubble:hover {
  transform: translateY(-2px);
  background: rgba(255, 255, 255, 0.05);
}

.msg-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.8rem;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  padding-bottom: 0.5rem;
}

.sender { font-weight: 800; color: var(--primary-color); font-size: 0.9rem; }
.time { opacity: 0.5; font-size: 0.8rem; }

.text-content {
  line-height: 1.6;
  white-space: pre-wrap;
  color: rgba(255,255,255,0.9);
}

.media-container img {
  max-width: 100%;
  border-radius: 12px;
  margin-top: 0.5rem;
  border: 1px solid rgba(255,255,255,0.1);
}

.file-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  background: rgba(0,0,0,0.3);
  padding: 1rem;
  border-radius: 12px;
  border: 1px dashed rgba(var(--primary-rgb), 0.3);
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

.chat-loading, .empty-chat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4rem;
  text-align: center;
}

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

/* Custom Scrollbar */
.message-feed::-webkit-scrollbar { width: 6px; }
.message-feed::-webkit-scrollbar-track { background: transparent; }
.message-feed::-webkit-scrollbar-thumb {
  background: rgba(var(--primary-rgb), 0.2);
  border-radius: 10px;
}
.message-feed::-webkit-scrollbar-thumb:hover {
  background: rgba(var(--primary-rgb), 0.4);
}
</style>
