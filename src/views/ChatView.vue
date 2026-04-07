<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { apiService } from '../services/api';

const activePlatform = ref('line');
const platforms = [
  { id: 'telegram', name: 'Telegram', icon: '✈️' },
  { id: 'discord', name: 'Discord', icon: '🎮' },
  { id: 'line', name: 'Line', icon: '🟢' },
  { id: 'remarks', name: 'Integrated', icon: '📚' }
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

// --- Integrated Remarks Logic ---
const remarkContainers = ref<any[]>([]);
const stagedItems = ref<any[]>([]);

const fetchMyStatus = async () => {
  try {
    const data = await apiService.getMyBotStatus();
    myStatus.value = data;
  } catch (err) {
    console.error('Failed to fetch bot status:', err);
  }
};

const fetchMessages = async () => {
  if (activePlatform.value === 'remarks') {
    await fetchRemarks();
    return;
  }
  loading.value = true;
  try {
    const data = await apiService.getChatLogs(
      activePlatform.value,
      searchQuery.value,
      startDate.value,
      endDate.value
    );
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

const fetchRemarks = async () => {
  loading.value = true;
  try {
    const data = await apiService.getRemarks();
    remarkContainers.value = data.containers;
    stagedItems.value = data.staged;
  } catch (err) {
    console.error('Failed to fetch remarks:', err);
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

const resetBg = (m: any) => {
  m.bgIndex = 0;
};

const toggleZoom = (m: any) => {
  m.isZoomed = !m.isZoomed;
};

const toggleIntegrate = async (m: any) => {
  try {
    const res = await apiService.toggleIntegration(m.id);
    m.isIntegrated = (res.status === 'added');
  } catch (err) {
    console.error('Integration toggle failed:', err);
  }
};

// --- Remarks Management Logic ---
const createNewRemark = async () => {
  const name = prompt('Enter Remark Group Name:');
  if (name) {
    await apiService.createRemark({ name });
    await fetchRemarks();
  }
};

const updateRemarkContent = async (container: any) => {
  await apiService.updateRemark(container.id, { name: container.name, content: container.content });
};

const copyRemark = async (container: any) => {
  await apiService.createRemark({ name: container.name + ' (Copy)', content: container.content });
  await fetchRemarks();
};

const deleteRemark = async (id: string) => {
  if (confirm('Delete this remark group?')) {
    await apiService.deleteRemark(id);
    await fetchRemarks();
  }
};

// Drag & Drop Handlers
const handleDragStart = (e: DragEvent, type: string, id: string) => {
  e.dataTransfer?.setData('type', type);
  e.dataTransfer?.setData('id', id);
};

const handleDropToContainer = async (e: DragEvent, containerId: string | null) => {
  e.preventDefault();
  const type = e.dataTransfer?.getData('type');
  const id = e.dataTransfer?.getData('id');

  if (type === 'item') {
    await apiService.moveRemarkItem(id!, containerId);
    await fetchRemarks();
  }
};

const removeItem = async (itemId: string) => {
  await apiService.removeRemarkItem(itemId);
  await fetchRemarks();
};

</script>

<template>
  <div class="chat-view">
    <header class="view-header">
      <div class="header-content">
        <h2>💬 Discovery & Knowledge</h2>
        <p>Browse archives and integrate insights into permanent remarks.</p>
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
          <template v-if="p.id !== 'remarks'">
            <span v-if="myStatus[p.id as keyof typeof myStatus]" class="status-dot linked" title="Linked"></span>
            <span v-else class="status-dot unlinked" title="Not Linked"></span>
          </template>
        </button>
      </div>
    </header>

    <!-- Search Tool (Hidden for Remarks) -->
    <div v-if="activePlatform !== 'remarks'" class="search-toolbar card">
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

    <!-- Main Content Container -->
    <div class="chat-container">
      <!-- 1. Standard Platform Feed -->
      <template v-if="activePlatform !== 'remarks'">
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
              <button @click="toggleIntegrate(m)" class="control-btn" :class="{ integrated: m.isIntegrated }" title="Integrate into Remarks">
                {{ m.isIntegrated ? '🌟' : '📁' }}
              </button>
              <button @click="resetBg(m)" class="control-btn" title="Reset Color">🔄</button>
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
                  <div v-if="!m.isZoomed" class="zoom-hint">🔍 Expand</div>
                </div>
                <div v-else class="file-card">
                  <span class="file-icon">{{ m.mediaType === 'video' ? '🎬' : '📎' }}</span>
                  <div class="file-info">
                    <span class="file-name">{{ m.mediaType === 'video' ? 'Video' : 'File' }}</span>
                    <a :href="getStorehouseUrl(m.mediaId)" target="_blank" class="download-link">View</a>
                  </div>
                </div>
              </template>
              <p v-else class="text-content">{{ m.content }}</p>
            </div>
          </div>
        </div>
      </template>

      <!-- 2. Integrated Remarks View -->
      <template v-else>
        <div class="remarks-view">
          <!-- STAGING AREA -->
          <div 
            class="staging-section card"
            @dragover.prevent
            @drop="handleDropToContainer($event, null)"
          >
            <div class="section-header">
              <h3>📥 Staging Area (暫存區)</h3>
              <p>Drag items here to unassign or click to remove.</p>
            </div>
            <div class="staged-grid">
              <div 
                v-for="item in stagedItems" 
                :key="item.id" 
                class="staged-card"
                draggable="true"
                @dragstart="handleDragStart($event, 'item', item.id)"
              >
                <div class="staged-content">
                  <span class="platform-indicator">{{ item.log.platform }}</span>
                  <p>{{ item.log.content.substring(0, 50) }}{{ item.log.content.length > 50 ? '...' : '' }}</p>
                </div>
                <button @click="removeItem(item.id)" class="remove-item">✕</button>
              </div>
              <div v-if="stagedItems.length === 0" class="empty-staged">
                No items staged. Start integrating cards from other tabs!
              </div>
            </div>
          </div>

          <!-- REMARK CONTAINERS -->
          <div class="remark-list">
            <div class="list-actions">
              <button @click="createNewRemark" class="primary-btn">+ New Remark Group</button>
            </div>

            <div class="remark-grid">
              <div 
                v-for="c in remarkContainers" 
                :key="c.id" 
                class="remark-container card"
                @dragover.prevent
                @drop="handleDropToContainer($event, c.id)"
              >
                <div class="container-header">
                  <input v-model="c.name" @blur="updateRemarkContent(c)" class="title-input" />
                  <div class="container-actions">
                    <button @click="copyRemark(c)" title="Duplicate">📋</button>
                    <button @click="deleteRemark(c.id)" title="Delete">🗑️</button>
                  </div>
                </div>

                <div class="container-items">
                  <div 
                    v-for="item in c.items" 
                    :key="item.id" 
                    class="mini-item-card"
                    draggable="true"
                    @dragstart="handleDragStart($event, 'item', item.id)"
                  >
                    <p>{{ item.log.content.substring(0, 60) }}...</p>
                    <button @click="removeItem(item.id)" class="mini-remove">✕</button>
                  </div>
                  <div v-if="c.items.length === 0" class="drop-hint">Drop items here</div>
                </div>

                <div class="container-footer">
                  <textarea 
                    v-model="c.content" 
                    @blur="updateRemarkContent(c)" 
                    placeholder="Enter your summary/note here..."
                  ></textarea>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Zoom Overlay (Global) -->
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
  max-width: 1500px;
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

.platform-tabs { display: flex; gap: 0.8rem; }
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

.status-dot { width: 6px; height: 6px; border-radius: 50%; position: absolute; top: 5px; right: 10px; }
.status-dot.linked { background: #2ecc71; box-shadow: 0 0 5px #2ecc71; }
.status-dot.unlinked { background: #95a5a6; }

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
.search-input-group input { background: transparent; border: none; color: white; width: 100%; outline: none; }

.date-filters { display: flex; align-items: center; gap: 1rem; }
.date-field { display: flex; align-items: center; gap: 0.5rem; }
.date-field.active { background: rgba(var(--primary-rgb), 0.1); padding: 2px 8px; border-radius: 8px; }
.date-field input { background: rgba(0,0,0,0.3); border: 1px solid var(--border-color); color: white; padding: 0.4rem 0.6rem; border-radius: 6px; outline: none; }
.date-field input::-webkit-calendar-picker-indicator {
  filter: invert(1); background-color: var(--primary-color); border-radius: 3px; padding: 2px;
}

/* Page Layout */
.grid-layout {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
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
  transition: all 0.3s ease;
  height: fit-content;
}

.message-card:hover { transform: translateY(-3px); box-shadow: 0 8px 25px rgba(0,0,0,0.2); }

.card-controls {
  position: absolute;
  top: 10px;
  right: 10px;
  display: flex;
  gap: 5px;
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
  backdrop-filter: blur(5px);
}
.control-btn:hover { background: rgba(255,255,255,0.3); transform: scale(1.1); }
.control-btn.integrated { background: rgba(241, 196, 15, 0.4); border: 1px solid #f1c40f; }

.msg-header { display: flex; flex-direction: column; margin-bottom: 0.8rem; border-bottom: 1px solid rgba(255,255,255,0.05); padding-bottom: 0.5rem; }
.sender { font-weight: 800; color: var(--primary-color); font-size: 0.9rem; }
.time { opacity: 0.4; font-size: 0.75rem; }

.media-container {
  cursor: zoom-in; margin: 0.5rem -0.5rem 0; border-radius: 12px; overflow: hidden; max-height: 200px;
}
.media-container img { width: 100%; height: 100%; object-fit: cover; }
.media-container.zoomed {
  position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); width: 90vw; max-height: 90vh; z-index: 1001; cursor: zoom-out; box-shadow: 0 0 50px rgba(0,0,0,0.8);
}

/* Remarks View Styles */
.remarks-view {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.staging-section {
  padding: 1.5rem;
  background: rgba(var(--primary-rgb), 0.05);
}

.section-header h3 { margin-bottom: 0.5rem; color: var(--primary-color); }
.staged-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-top: 1rem;
  min-height: 80px;
}

.staged-card {
  background: rgba(255,255,255,0.07);
  padding: 0.8rem;
  border-radius: 12px;
  width: 250px;
  position: relative;
  cursor: grab;
  border: 1px dashed var(--border-color);
}
.staged-card:active { cursor: grabbing; }

.platform-indicator { font-size: 0.6rem; text-transform: uppercase; background: var(--primary-color); color: white; padding: 2px 6px; border-radius: 4px; display: inline-block; margin-bottom: 5px; }
.remove-item { position: absolute; top: 5px; right: 5px; background: transparent; border: none; color: #e74c3c; cursor: pointer; }

.remark-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 2rem;
}

.remark-container {
  display: flex;
  flex-direction: column;
  min-height: 400px;
  background: rgba(255,255,255,0.03);
  padding: 1.5rem;
  border: 1px solid var(--border-color);
}

.container-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}
.title-input { background: transparent; border: none; font-size: 1.2rem; font-weight: 800; color: white; border-bottom: 2px solid transparent; width: 100%; outline: none; }
.title-input:focus { border-bottom-color: var(--primary-color); }

.container-actions button { background: transparent; border: none; cursor: pointer; font-size: 1.1rem; padding: 5px; opacity: 0.6; }
.container-actions button:hover { opacity: 1; }

.container-items {
  flex: 1;
  background: rgba(0,0,0,0.2);
  border-radius: 12px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  margin-bottom: 1rem;
  border: 2px dashed rgba(255,255,255,0.05);
}

.mini-item-card {
  background: rgba(255,255,255,0.05);
  padding: 0.8rem;
  border-radius: 8px;
  font-size: 0.85rem;
  position: relative;
}

.mini-remove { position: absolute; top: 2px; right: 2px; border: none; background: transparent; color: #888; cursor: pointer; font-size: 0.7rem; }

.container-footer textarea {
  width: 100%;
  height: 100px;
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  color: white;
  padding: 1rem;
  resize: vertical;
  outline: none;
}
.container-footer textarea:focus { border-color: var(--primary-color); }

.primary-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 0.8rem 1.5rem;
  border-radius: 8px;
  font-weight: 700;
  cursor: pointer;
  margin-bottom: 1rem;
}

.zoom-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.9); z-index: 1000; }
.close-overlay { position: absolute; top: 20px; right: 20px; color: white; font-size: 2rem; cursor: pointer; }

@keyframes spin { to { transform: rotate(360deg); } }
.spinner { width: 50px; height: 50px; border: 4px solid rgba(var(--primary-rgb), 0.1); border-top-color: var(--primary-color); border-radius: 50%; animation: spin 1s linear infinite; margin-bottom: 1.5rem; }
</style>
