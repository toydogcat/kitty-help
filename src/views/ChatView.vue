<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue';
import { apiService } from '../services/api';

const activePlatform = ref('remarks');
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
const remarkSearchQuery = ref('');
const startDate = ref('');
const endDate = ref('');

// Pagination
const currentPage = ref(1);
const pageSize = 20;

// Zoom Overlay for Remarks
const zoomedMediaUrl = ref('');

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
const deskItems = ref<any[]>([]); // To track which remarks are on desk

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
    const [remarksData, deskData] = await Promise.all([
      apiService.getRemarks(),
      apiService.getDeskItems('null') // Fetch desktop top-level items
    ]);
    remarkContainers.value = remarksData.containers || [];
    stagedItems.value = remarksData.staged || [];
    deskItems.value = deskData || [];
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
  currentPage.value = 1;
  fetchMessages();
});

// Debounce search
let searchTimeout: any;
watch([searchQuery, remarkSearchQuery], () => {
  clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    if (activePlatform.value !== 'remarks') {
      fetchMessages();
    }
  }, 500);
});

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  const p = platform || activePlatform.value;
  return apiService.getStorehouseFileUrl(mediaId, p);
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

const openRemarkZoom = (url: string) => {
  zoomedMediaUrl.value = url;
};

const closeRemarkZoom = () => {
  zoomedMediaUrl.value = '';
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
  await apiService.updateRemark(container.id, { 
    name: container.name, 
    content: container.content,
    isPinned: container.isPinned
  });
};

const togglePin = async (container: any) => {
  container.isPinned = !container.isPinned;
  await updateRemarkContent(container);
  await fetchRemarks(); // Re-fetch to sort
};

// Toggle Citation to Desk
const isItemOnDesk = (containerId: string) => {
  return deskItems.value.some(item => item.type === 'remark' && item.refId === containerId);
};

const toggleDeskPin = async (container: any) => {
  const existing = deskItems.value.find(item => item.type === 'remark' && item.refId === container.id);
  if (existing) {
    await apiService.deleteDeskItem(existing.id);
  } else {
    await apiService.addDeskItem({
      type: 'remark',
      refId: container.id,
      shelfId: null,
      sortOrder: 0
    });
  }
  await fetchRemarks(); // Refresh desk items state
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

// Computed Filters
const filteredRemarks = computed(() => {
  if (!remarkSearchQuery.value) return remarkContainers.value;
  const q = remarkSearchQuery.value.toLowerCase();
  return remarkContainers.value.filter(c => 
    c.name.toLowerCase().includes(q) || (c.content && c.content.toLowerCase().includes(q))
  );
});

const pinnedRemarks = computed(() => filteredRemarks.value.filter(c => c.isPinned));
const unpinnedRemarks = computed(() => filteredRemarks.value.filter(c => !c.isPinned));

const paginatedUnpinned = computed(() => {
  const start = (currentPage.value - 1) * pageSize;
  const end = start + pageSize;
  return unpinnedRemarks.value.slice(start, end);
});

const totalPages = computed(() => Math.ceil(unpinnedRemarks.value.length / pageSize));

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

    <div class="search-toolbar card">
      <template v-if="activePlatform !== 'remarks'">
        <div class="search-input-group">
          <span class="search-icon">🔍</span>
          <input v-model="searchQuery" type="text" placeholder="Search messages..." class="text-input" />
        </div>
        <div class="date-filters">
           <div class="date-field"><label>From</label><input v-model="startDate" type="date" /></div>
           <div class="date-field"><label>To</label><input v-model="endDate" type="date" /></div>
           <button @click="searchQuery = ''; startDate = ''; endDate = '';" class="clear-btn">🧹</button>
        </div>
      </template>
      <template v-else>
        <div class="search-input-group">
          <span class="search-icon">🔍</span>
          <input v-model="remarkSearchQuery" type="text" placeholder="Search within remarks..." class="text-input" />
        </div>
        <button @click="createNewRemark" class="primary-btn-mini">+ New Group</button>
      </template>
    </div>

    <div class="chat-container">
      <template v-if="activePlatform !== 'remarks'">
        <div v-if="loading" class="chat-loading"><div class="spinner"></div><p>Scanning archives...</p></div>
        <div v-else-if="messages.length === 0" class="empty-chat"><p>No messages found.</p></div>
        <div v-else class="message-feed grid-layout">
          <div v-for="m in messages" :key="m.id" class="message-card" :style="{ background: cardBackgrounds[m.bgIndex] }">
            <div class="card-controls">
              <button @click="toggleIntegrate(m)" class="control-btn" :class="{ integrated: m.isIntegrated }">{{ m.isIntegrated ? '🌟' : '📁' }}</button>
              <button @click="cycleBg(m)" class="control-btn">🎨</button>
            </div>
            <div class="msg-header">
              <span class="sender">{{ m.senderName }}</span>
              <span class="time">{{ formatDate(m.createdAt) }}</span>
            </div>
            <div class="msg-content">
              <template v-if="m.mediaId && (m.msgType === 'image' || m.content.includes('[Image]'))">
                <div class="media-container" :class="{ zoomed: m.isZoomed }" @click="toggleZoom(m)">
                  <img :src="getStorehouseUrl(m.mediaId)" loading="lazy" />
                  <div v-if="!m.isZoomed" class="zoom-hint">🔍 Expand</div>
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
          <div class="staging-section card" @dragover.prevent @drop="handleDropToContainer($event, null)">
            <div class="section-header">
              <h3>📥 Staging Area (暫存區)</h3>
              <p>Drag items here to unassign or click to remove.</p>
            </div>
            <div class="staged-grid">
              <div v-for="item in stagedItems" :key="item.id" class="staged-card" draggable="true" @dragstart="handleDragStart($event, 'item', item.id)">
                <div class="staged-content">
                  <header class="mini-tag-line">
                    <span class="platform-indicator">{{ item.log.platform }}</span>
                    <button @click="removeItem(item.id)" class="remove-item">✕</button>
                  </header>
                  <div v-if="item.log?.mediaId && (item.log?.msgType === 'image' || item.log?.content.includes('[Image]'))" class="staged-thumb" @click="openRemarkZoom(getStorehouseUrl(item.log.mediaId, item.log.platform))">
                    <img :src="getStorehouseUrl(item.log.mediaId, item.log.platform)" />
                  </div>
                  <p v-else>{{ item.log.content.substring(0, 100) }}</p>
                </div>
              </div>
              <div v-if="(stagedItems || []).length === 0" class="empty-staged">No items staged.</div>
            </div>
          </div>

          <!-- PINNED SECTION -->
          <div v-if="pinnedRemarks.length > 0" class="pinned-section">
            <h3 class="section-title">✨ Pinned (釘選)</h3>
            <div class="remark-grid">
              <div v-for="c in pinnedRemarks" :key="c.id" class="remark-container card pinned" @dragover.prevent @drop="handleDropToContainer($event, c.id)">
                <div class="container-header">
                  <input v-model="c.name" @blur="updateRemarkContent(c)" class="title-input" />
                  <div class="container-actions">
                    <!-- Internal Pin uses Star -->
                    <button @click="togglePin(c)" title="Pin/Unpin internally" class="pin-btn active">⭐</button>
                    <!-- Desk Pin uses Pushpin -->
                    <button @click="toggleDeskPin(c)" :title="isItemOnDesk(c.id) ? 'Remove from Desk' : 'Pin to Desk'" class="desk-pin-btn" :class="{ onDesk: isItemOnDesk(c.id) }">📌</button>
                    <button @click="copyRemark(c)" title="Copy Group">📋</button>
                    <button @click="deleteRemark(c.id)" title="Delete">🗑️</button>
                  </div>
                </div>
                <div class="container-items custom-scrollbar">
                  <div v-for="item in (c.items || [])" :key="item.id" class="mini-item-card" draggable="true" @dragstart="handleDragStart($event, 'item', item.id)">
                    <div class="mini-item-content">
                       <div v-if="item.log?.mediaId && (item.log?.msgType === 'image' || item.log?.content.includes('[Image]'))" class="mini-thumb" @click="openRemarkZoom(getStorehouseUrl(item.log.mediaId, item.log.platform))">
                          <img :src="getStorehouseUrl(item.log.mediaId, item.log.platform)" loading="lazy" />
                       </div>
                       <p v-else>{{ item.log?.content ? (item.log.content.substring(0, 80)) : 'No Content' }}</p>
                    </div>
                    <button @click="removeItem(item.id)" class="mini-remove">✕</button>
                  </div>
                  <div v-if="!(c.items && c.items.length > 0)" class="drop-hint">Drop items here</div>
                </div>
                <div class="container-footer">
                  <label>Notes & Summary</label>
                  <textarea v-model="c.content" @blur="updateRemarkContent(c)" placeholder="Summary..."></textarea>
                </div>
              </div>
            </div>
          </div>

          <!-- ALL REMARKS -->
          <div class="all-remarks-section">
            <h3 class="section-title">📚 All Remarks (共 {{ unpinnedRemarks.length }} 個)</h3>
            <div class="remark-grid">
              <div v-for="c in paginatedUnpinned" :key="c.id" class="remark-container card" @dragover.prevent @drop="handleDropToContainer($event, c.id)">
                <div class="container-header">
                   <input v-model="c.name" @blur="updateRemarkContent(c)" class="title-input" />
                   <div class="container-actions">
                     <!-- Internal Pin uses Star -->
                     <button @click="togglePin(c)" title="Pin/Unpin internally" class="pin-btn">⭐</button>
                     <!-- Desk Pin uses Pushpin -->
                     <button @click="toggleDeskPin(c)" :title="isItemOnDesk(c.id) ? 'Remove from Desk' : 'Pin to Desk'" class="desk-pin-btn" :class="{ onDesk: isItemOnDesk(c.id) }">📌</button>
                     <button @click="copyRemark(c)" title="Copy Group">📋</button>
                     <button @click="deleteRemark(c.id)" title="Delete">🗑️</button>
                   </div>
                </div>
                <div class="container-items custom-scrollbar">
                  <div v-for="item in (c.items || [])" :key="item.id" class="mini-item-card" draggable="true" @dragstart="handleDragStart($event, 'item', item.id)">
                    <div class="mini-item-content">
                       <div v-if="item.log?.mediaId && (item.log?.msgType === 'image' || item.log?.content.includes('[Image]'))" class="mini-thumb" @click="openRemarkZoom(getStorehouseUrl(item.log.mediaId, item.log.platform))">
                          <img :src="getStorehouseUrl(item.log.mediaId, item.log.platform)" loading="lazy" />
                       </div>
                       <p v-else>{{ item.log?.content }}</p>
                    </div>
                    <button @click="removeItem(item.id)" class="mini-remove">✕</button>
                  </div>
                  <div v-if="!(c.items && c.items.length > 0)" class="drop-hint">Drop items here</div>
                </div>
                <div class="container-footer">
                   <label>Notes & Summary</label>
                  <textarea v-model="c.content" @blur="updateRemarkContent(c)" placeholder="Summary..."></textarea>
                </div>
              </div>
            </div>
            <div v-if="totalPages > 1" class="pagination-footer card">
              <button :disabled="currentPage === 1" @click="currentPage--">Previous</button>
              <span>Page {{ currentPage }} of {{ totalPages }}</span>
              <button :disabled="currentPage === totalPages" @click="currentPage++">Next</button>
            </div>
          </div>
        </div>
      </template>
    </div>

    <Transition name="fade">
      <div v-if="messages.some(m => m.isZoomed) || zoomedMediaUrl" class="zoom-overlay" @click="messages.forEach(m => m.isZoomed = false); closeRemarkZoom()">
        <template v-if="zoomedMediaUrl"><img :src="zoomedMediaUrl" class="zoomed-image-remark" /></template>
        <span class="close-overlay">✕</span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.chat-view { display: flex; flex-direction: column; gap: 1.2rem; max-width: 1600px; margin: 0 auto; padding: 1rem; }
.view-header { display: flex; justify-content: space-between; align-items: flex-end; border-bottom: 2px solid rgba(var(--primary-rgb), 0.2); padding-bottom: 1.5rem; }
.platform-tabs { display: flex; gap: 0.8rem; }
.platform-btn { display: flex; align-items: center; gap: 0.6rem; padding: 0.6rem 1.2rem; background: rgba(255, 255, 255, 0.05); border: 1px solid var(--border-color); border-radius: 20px; color: var(--text-color); cursor: pointer; font-weight: 600; }
.platform-btn.active { background: var(--primary-color); color: white; box-shadow: 0 4px 12px rgba(var(--primary-rgb), 0.3); }

.search-toolbar { display: flex; justify-content: space-between; align-items: center; gap: 1.5rem; padding: 0.8rem 1.5rem; background: rgba(var(--primary-rgb), 0.03); backdrop-filter: blur(10px); }
.search-input-group { flex: 1; display: flex; align-items: center; gap: 0.8rem; background: rgba(0,0,0,0.2); padding: 0.6rem 1rem; border-radius: 10px; border: 1px solid var(--border-color); }
.search-input-group input { background: transparent; border: none; color: white; width: 100%; outline: none; }

.grid-layout { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 1.5rem; padding: 1rem 0; }
.message-card { position: relative; display: flex; flex-direction: column; padding: 1.2rem; border-radius: 16px; border: 1px solid var(--border-color); background: rgba(255,255,255,0.05); }

.media-container { cursor: zoom-in; margin-top: 0.5rem; border-radius: 12px; overflow: hidden; max-height: 200px; position: relative; }
.media-container img { width: 100%; height: 100%; object-fit: cover; }

.remarks-view { display: flex; flex-direction: column; gap: 2rem; }
.staging-section { padding: 1.5rem; background: rgba(var(--primary-rgb), 0.05); min-height: 150px; }
.staged-grid { display: flex; flex-wrap: wrap; gap: 1rem; margin-top: 1rem; }
.staged-card { background: rgba(255,255,255,0.07); padding: 0.8rem; border-radius: 12px; width: 280px; border: 1px dashed var(--border-color); }
.staged-thumb { width: 100%; height: 120px; border-radius: 8px; overflow: hidden; cursor: zoom-in; margin-bottom: 5px; }
.staged-thumb img { width: 100%; height: 100%; object-fit: cover; }

.remark-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(380px, 1fr)); gap: 1.5rem; }
.remark-container { display: flex; flex-direction: column; background: rgba(255,255,255,0.03); padding: 1.5rem; border: 1px solid var(--border-color); border-radius: 16px; min-height: 520px; max-height: 600px; }
.remark-container.pinned { border-color: rgba(241, 196, 15, 0.5); background: linear-gradient(135deg, rgba(241, 196, 15, 0.08), rgba(241, 196, 15, 0.02)); }

.container-actions { display: flex; gap: 5px; }
.pin-btn, .desk-pin-btn { background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); border-radius: 4px; padding: 2px 6px; cursor: pointer; transition: all 0.2s; }
.pin-btn:hover, .desk-pin-btn:hover { background: rgba(255,255,255,0.15); border-color: var(--primary-color); }
.pin-btn.active { background: rgba(241, 196, 15, 0.2); border-color: #f1c40f; }
.desk-pin-btn.onDesk { background: rgba(var(--primary-rgb), 0.2); border-color: var(--primary-color); }

.container-items { flex: 1; overflow-y: auto; background: rgba(0,0,0,0.1); padding: 1rem; border-radius: 12px; margin: 1rem 0; border: 1px solid rgba(255,255,255,0.05); }
.mini-item-card { background: rgba(255,255,255,0.05); padding: 0.8rem; border-radius: 8px; margin-bottom: 0.8rem; position: relative; }
.mini-thumb { width: 100%; height: 100px; border-radius: 6px; overflow: hidden; cursor: zoom-in; margin-top: 5px; }
.mini-thumb img { width: 100%; height: 100%; object-fit: cover; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; }
.custom-scrollbar::-webkit-scrollbar-track { background: rgba(0,0,0,0.1); }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(var(--primary-rgb), 0.3); border-radius: 10px; }
.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: var(--primary-color); }

.container-footer { border-top: 1px solid rgba(255,255,255,0.05); padding-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; }
.container-footer label { font-size: 0.7rem; text-transform: uppercase; opacity: 0.5; font-weight: 700; }
.container-footer textarea { width: 100%; min-height: 100px; background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: white; padding: 0.8rem; resize: none; outline: none; transition: border-color 0.2s; }
.container-footer textarea:focus { border-color: var(--primary-color); }

.zoom-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.9); z-index: 2000; display: flex; align-items: center; justify-content: center; cursor: zoom-out; }
.zoomed-image-remark { max-width: 90vw; max-height: 90vh; border-radius: 12px; box-shadow: 0 0 50px rgba(0,0,0,0.5); }
.close-overlay { position: absolute; top: 20px; right: 20px; color: white; font-size: 2rem; cursor: pointer; }

.pagination-footer { display: flex; justify-content: center; align-items: center; gap: 2rem; padding: 1.5rem; margin-top: 2rem; background: rgba(var(--primary-rgb), 0.05); }

.spinner { width: 40px; height: 40px; border: 3px solid rgba(255,255,255,0.1); border-top-color: var(--primary-color); border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
