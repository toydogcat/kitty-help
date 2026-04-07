<script setup lang="ts">
import { ref, onMounted, computed, defineAsyncComponent } from 'vue';
import { apiService, socket } from '../services/api';
const UnifiedRemarkModal = defineAsyncComponent(() => import('../components/UnifiedRemarkModal.vue'));
import { usePin } from '../composables/usePin';
import { marked } from 'marked';

// Search & Filters
const recentMessages = ref<any[]>([]);
const remarkContainers = ref<any[]>([]);
const loading = ref(true);
const filters = ref({
  platform: '',
  q: '',
  startDate: '',
  endDate: '',
  limit: 100
});

// Editor Toggle for Remarks in sidebar
const remarkEditModes = ref<Record<string, 'preview' | 'edit'>>({});

// Global Editor for Remarks (Unified with Desk)
const showRemarkModal = ref(false);
const editingRemark = ref<any>(null);
const remarkEditBuffer = ref({ title: '', content: '' });
const remarkModalEditMode = ref<'preview' | 'edit'>('preview');
const remarkModalFullScreen = ref(false);
const savingRemark = ref(false);
const { toggleRemarkSidebarPin, pinToDesk } = usePin();
const remarkModalDetails = ref<any>(null); // For Quoted Items
const zoomedImageUrl = ref('');

// Drag & Drop
const dragOverRemarkId = ref<string | null>(null);

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  return apiService.getStorehouseFileUrl(mediaId, platform || 'line');
};

const fetchRecentMessages = async () => {
  loading.value = true;
  try {
    const [msgData, remarkData] = await Promise.all([
      apiService.getChatLogs(
        filters.value.platform,
        filters.value.q,
        filters.value.startDate,
        filters.value.endDate
      ),
      apiService.getRemarks()
    ]);
    recentMessages.value = msgData.slice(0, filters.value.limit);
    remarkContainers.value = remarkData.containers || [];
    
    remarkData.containers?.forEach((c: any) => {
      if (!remarkEditModes.value[c.id]) {
        remarkEditModes.value[c.id] = 'preview';
      }
    });
  } catch (err) {
    console.error("Fetch error:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchRecentMessages();
  socket.on('messagesUpdate', fetchRecentMessages);
});

const onDragStart = (e: DragEvent, item: any, _type: string = 'media') => {
  e.dataTransfer?.setData('application/json', JSON.stringify({ type: 'media', data: item }));
};

const handleDragOver = (e: DragEvent, containerId: string) => {
  e.preventDefault();
  dragOverRemarkId.value = containerId;
};

const handleDropOnRemark = async (e: DragEvent, containerId: string) => {
  e.preventDefault();
  dragOverRemarkId.value = null;
  const raw = e.dataTransfer?.getData('application/json');
  if (!raw) return;
  const payload = JSON.parse(raw);
  
  if (payload.type === 'media') {
    try {
      await apiService.addRemarkItem({
        containerId: containerId,
        logId: payload.data.id
      });
      await fetchRecentMessages();
    } catch (err) {
      alert("Failed to add to remark");
    }
  }
};

const createNewRemark = async () => {
  const name = prompt("Enter Remark Group Name:");
  if (!name) return;
  try {
    await apiService.createRemark({ name, content: "" });
    await fetchRecentMessages();
  } catch (err) {
    alert("Creation failed");
  }
};

const togglePin = async (c: any) => {
  try {
    await toggleRemarkSidebarPin(c.id, c.isPinned);
    await fetchRecentMessages();
  } catch (err) {
    alert("Pin toggle failed");
  }
};

const deleteRemark = async (id: string) => {
  if (!confirm("Delete this group and all its links?")) return;
  try {
    await apiService.deleteRemark(id);
    await fetchRecentMessages();
  } catch (err) {
    alert("Delete failed");
  }
};

const addToDesk = async (c: any) => {
  try {
    await pinToDesk('remark', c.id);
    alert("Pinned to Desk! 📌");
  } catch (err) {
    alert("Failed to pin to desk");
  }
};

const copyRemark = (c: any) => {
  const text = (c.content || "") + "\n\n--- Items ---\n" + 
               (c.items || []).map((i: any) => `[${i.log.platform}] ${i.log.senderName}: ${i.log.content}`).join("\n");
  navigator.clipboard.writeText(text);
  alert("Copied to clipboard!");
};

// MODAL LOGIC (Unified)
const openRemarkModal = async (c: any) => {
  editingRemark.value = c;
  remarkEditBuffer.value = { title: c.name, content: c.content || '' };
  remarkModalEditMode.value = 'preview';
  remarkModalFullScreen.value = false;
  showRemarkModal.value = true;
  
  // Reload details to ensure we have latest items
  try {
    const data = await apiService.getRemarks();
    const container = data.containers?.find((x: any) => x.id === c.id);
    remarkModalDetails.value = container || null;
  } catch (err) {
    console.error("Detail reload failed");
  }
};

const saveRemarkEdit = async (updatedData: { title: string, content: string }) => {
  if (!editingRemark.value) return;
  savingRemark.value = true;
  try {
    await apiService.updateRemark(editingRemark.value.id, {
      name: updatedData.title,
      content: updatedData.content
    });
    showRemarkModal.value = false;
    await fetchRecentMessages();
  } catch (err) {
    alert("Save failed");
  } finally {
    savingRemark.value = false;
  }
};

const pinnedRemarks = computed(() => remarkContainers.value.filter(c => c.isPinned));
const otherRemarks = computed(() => remarkContainers.value.filter(c => !c.isPinned));
</script>

<template>
  <div class="chat-view">
    <!-- Center Panel: Unified Search Terminal -->
    <div class="center-panel">
      <div class="panel-header search-header">
        <div class="header-top">
          <h2>🔍 Unified Chat Terminal</h2>
          <div class="quick-stats">{{ recentMessages.length }} items found</div>
        </div>
        
        <div class="filter-bar">
          <div class="f-group">
            <label>Platform</label>
            <select v-model="filters.platform" @change="fetchRecentMessages">
              <option value="">All Platforms</option>
              <option value="discord">Discord</option>
              <option value="telegram">Telegram</option>
              <option value="line">LINE</option>
            </select>
          </div>
          <div class="f-group">
            <label>Start Date</label>
            <input type="date" v-model="filters.startDate" @change="fetchRecentMessages" />
          </div>
          <div class="f-group">
            <label>End Date</label>
            <input type="date" v-model="filters.endDate" @change="fetchRecentMessages" />
          </div>
          <div class="f-group flex-grow">
            <label>Search Query</label>
            <input type="text" v-model="filters.q" placeholder="Type to search..." @keyup.enter="fetchRecentMessages" />
          </div>
          <div class="f-group">
            <label>Limit</label>
            <select v-model="filters.limit" @change="fetchRecentMessages">
              <option :value="50">50 (Default)</option>
              <option :value="100">100</option>
              <option :value="200">200</option>
            </select>
          </div>
        </div>
      </div>

      <div class="messages-list custom-scrollbar">
         <div v-for="m in recentMessages" :key="m.id" class="msg-card" :class="m.platform">
            <div class="msg-bubble shadow-sm" draggable="true" @dragstart="onDragStart($event, m, 'log')">
              <div class="msg-meta">
                <span class="platform-indicator" :class="m.platform"></span>
                <span class="platform-name">{{ m.platform.toUpperCase() }}</span>
                <span class="sender-name">{{ m.senderName }}</span>
                <span class="time">{{ m.createdAt ? new Date(m.createdAt).toLocaleString([], {month: 'numeric', day: 'numeric', hour: '2-digit', minute:'2-digit'}) : '' }}</span>
              </div>

              <!-- Media Context -->
              <div v-if="m.mediaId" class="msg-media-snippet">
                <!-- If Image (Inclusive check for 'image', 'photo', 'attachment' OR ANY Discord with mediaId) -->
                <div v-if="['image', 'photo', 'attachment'].includes((m.msgType || '').toLowerCase()) || ['image', 'photo', 'attachment'].includes((m.mediaType || '').toLowerCase()) || (m.content && m.content.includes('[Image]')) || m.platform === 'discord'" class="inline-thumb" @click="zoomedImageUrl = getStorehouseUrl(m.mediaId, m.platform)">
                   <img :src="getStorehouseUrl(m.mediaId, m.platform)" loading="lazy" />
                   <div class="zoom-overlay"><span class="icon">🔍</span></div>
                </div>
                <!-- If Other File -->
                <div v-else class="file-tag">
                   <span class="file-icon">📄</span>
                   <span class="file-info">{{ m.mediaType || 'Attachment' }}</span>
                </div>
              </div>

              <div class="msg-text">{{ m.content }}</div>
            </div>
         </div>
      </div>
    </div>

    <!-- Right Panel: Resource Organization (資源整合) -->
    <div class="right-panel">
      <div class="remarks-section">
        <div class="remarks-header">
          <label class="remark-group-label">Resource Repository (知識庫)</label>
          <div class="header-main">
            <h2>🔖 Remarks</h2>
            <button class="add-btn" @click="createNewRemark">+ New Group</button>
          </div>
        </div>

        <div class="remarks-list custom-scrollbar">
          <div v-if="pinnedRemarks.length > 0" class="remark-group-label mini">✨ Pinned (釘選)</div>
          <div v-for="c in pinnedRemarks" :key="c.id" 
               class="remark-item-card" 
               :class="{ 'drag-over': dragOverRemarkId === c.id }"
               @dragover="handleDragOver($event, c.id)"
               @dragleave="dragOverRemarkId = null"
               @drop="handleDropOnRemark($event, c.id)">
            
            <div class="remark-card-header">
              <div class="remark-title" @click="openRemarkModal(c)">
                <span class="icon">⭐</span> {{ c.name }}
              </div>
              <div class="remark-actions">
                <button class="act-btn" @click="togglePin(c)" title="Unpin">📌</button>
                <button class="act-btn" @click="addToDesk(c)" title="Add to Desk">📋</button>
                <button class="act-btn" @click="copyRemark(c)" title="Copy">📄</button>
                <button class="act-btn del" @click="deleteRemark(c.id)" title="Delete">🗑️</button>
              </div>
            </div>

            <div class="remark-card-body">
              <div class="body-header">
                <span class="label">Preview</span>
                <div class="mini-mode-switch">
                  <button :class="{ active: remarkEditModes[c.id] === 'preview' }" @click="remarkEditModes[c.id] = 'preview'">MD</button>
                  <button :class="{ active: remarkEditModes[c.id] === 'edit' }" @click="remarkEditModes[c.id] = 'edit'">TXT</button>
                </div>
              </div>
              <div v-if="remarkEditModes[c.id] === 'preview'" class="sidebar-md-box custom-scrollbar" v-html="marked.parse(c.content || 'No description.')"></div>
              <div v-else class="sidebar-txt-box">{{ c.content || 'No description.' }}</div>
              <div class="items-count" @click="openRemarkModal(c)">🔗 {{ c.items?.length || 0 }} items linked</div>
            </div>
          </div>

          <!-- Other Section -->
          <div v-if="otherRemarks.length > 0" class="remark-group-label mini">📦 All Groups (全部)</div>
          <div v-for="c in otherRemarks" :key="c.id" 
               class="remark-item-card" 
               :class="{ 'drag-over': dragOverRemarkId === c.id }"
               @dragover="handleDragOver($event, c.id)"
               @dragleave="dragOverRemarkId = null"
               @drop="handleDropOnRemark($event, c.id)">
            
            <div class="remark-card-header">
              <div class="remark-title" @click="openRemarkModal(c)">{{ c.name }}</div>
              <div class="remark-actions">
                <button class="act-btn" @click="togglePin(c)" title="Pin">☆</button>
                <button class="act-btn" @click="addToDesk(c)" title="Add to Desk">📋</button>
                <button class="act-btn" @click="copyRemark(c)" title="Copy">📄</button>
                <button class="act-btn del" @click="deleteRemark(c.id)" title="Delete">🗑️</button>
              </div>
            </div>

            <div class="remark-card-body">
              <div class="body-header">
                <span class="label">Preview</span>
                <div class="mini-mode-switch">
                  <button :class="{ active: remarkEditModes[c.id] === 'preview' }" @click="remarkEditModes[c.id] = 'preview'">MD</button>
                  <button :class="{ active: remarkEditModes[c.id] === 'edit' }" @click="remarkEditModes[c.id] = 'edit'">TXT</button>
                </div>
              </div>
              <div v-if="remarkEditModes[c.id] === 'preview'" class="sidebar-md-box custom-scrollbar" v-html="marked.parse(c.content || 'No description.')"></div>
              <div v-else class="sidebar-txt-box">{{ c.content || 'No description.' }}</div>
              <div class="items-count" @click="openRemarkModal(c)">🔗 {{ c.items?.length || 0 }} items linked</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- UNIFIED REMARK EDITOR MODAL -->
    <UnifiedRemarkModal 
      :show="showRemarkModal"
      :item="{ ...editingRemark, type: 'remark' }"
      :details="remarkModalDetails"
      :loading="savingRemark"
      @close="showRemarkModal = false"
      @save="saveRemarkEdit"
      @zoom="zoomedImageUrl = $event"
    />

    <Teleport to="body">
      <div v-if="zoomedImageUrl" class="global-zoom" @click="zoomedImageUrl = ''">
         <img :src="zoomedImageUrl" />
         <span class="close-zoom">✕</span>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.chat-view { display: flex; height: calc(100vh - 100px); gap: 1rem; padding: 1rem; }

.center-panel { flex: 1.5; background: rgba(0,0,0,0.2); border-radius: 24px; border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; overflow: hidden; }
.search-header { 
  padding: 1.5rem 2rem; 
  background: rgba(255,255,255,0.02); 
  border-bottom: 1px solid rgba(255,255,255,0.05); 
  display: flex; 
  flex-direction: column; 
  gap: 1.2rem;
}
.header-top { display: flex; justify-content: space-between; align-items: center; }
.header-top h2 { font-size: 1.3rem; font-weight: 800; color: #fff; }
.quick-stats { font-size: 0.75rem; opacity: 0.4; font-weight: 800; color: var(--primary-color); text-transform: uppercase; }

.filter-bar { display: flex; gap: 0.8rem; flex-wrap: wrap; align-items: flex-end; }
.f-group { display: flex; flex-direction: column; gap: 0.4rem; min-width: 110px; }
.f-group label { font-size: 0.6rem; font-weight: 900; opacity: 0.3; text-transform: uppercase; letter-spacing: 1px; color: #fff; }
.f-group select, .f-group input { 
  background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 10px; 
  padding: 0.6rem 0.8rem; 
  color: #fff; 
  font-size: 0.85rem; 
  outline: none;
  transition: all 0.2s;
}
.f-group select:focus, .f-group input:focus { border-color: var(--primary-color); background: rgba(0,0,0,0.4); }
.flex-grow { flex: 1; min-width: 180px; }

.messages-list { flex: 1; padding: 2rem; overflow-y: auto; display: flex; flex-direction: column; gap: 1.2rem; }

.msg-card { display: flex; flex-direction: column; align-items: flex-start; width: 100%; }
.msg-bubble { 
  max-width: 90%; 
  background: rgba(255,255,255,0.04); 
  border-radius: 20px; 
  padding: 1rem 1.4rem; 
  border: 1px solid rgba(255,255,255,0.06); 
  display: flex; 
  flex-direction: column; 
  gap: 0.4rem;
  transition: all 0.2s;
}
.msg-bubble:hover { border-color: rgba(var(--primary-rgb), 0.3); background: rgba(255,255,255,0.06); }

.msg-meta { display: flex; align-items: center; gap: 0.8rem; margin-bottom: 2px; }
.platform-indicator { width: 8px; height: 8px; border-radius: 50%; }
.platform-indicator.discord { background: #5865F2; box-shadow: 0 0 10px #5865F2; }
.platform-indicator.telegram { background: #0088cc; box-shadow: 0 0 10px #0088cc; }
.platform-indicator.line { background: #00B900; box-shadow: 0 0 10px #00B900; }

.platform-name { font-size: 0.65rem; font-weight: 900; opacity: 0.4; letter-spacing: 1.5px; text-transform: uppercase; }
.sender-name { font-weight: 800; color: #fff; font-size: 0.9rem; }
.time { font-size: 0.75rem; opacity: 0.3; margin-left: auto; letter-spacing: 0.5px; }

.msg-media-snippet { margin: 0.6rem 0; border-radius: 14px; overflow: hidden; border: 1px solid rgba(255,255,255,0.05); background: #000; }
.inline-thumb { position: relative; height: 180px; cursor: zoom-in; overflow: hidden; }
.inline-thumb img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.4s; }
.inline-thumb:hover img { transform: scale(1.08); }
.zoom-overlay { position: absolute; inset: 0; background: rgba(0,0,0,0.3); display: flex; align-items: center; justify-content: center; opacity: 0; transition: 0.2s; }
.inline-thumb:hover .zoom-overlay { opacity: 1; }

.file-tag { padding: 1.2rem; background: rgba(255,255,255,0.03); display: flex; align-items: center; gap: 1rem; }
.file-info { font-weight: 800; color: var(--primary-color); font-size: 0.85rem; text-transform: uppercase; letter-spacing: 1px; }

.msg-text { font-size: 1rem; color: #eee; line-height: 1.6; word-break: break-word; }

/* Right Panel Refactored */
.right-panel { width: 450px; display: flex; flex-direction: column; }
.remarks-section { flex: 1; min-height: 0; background: rgba(0,0,0,0.2); border-radius: 24px; padding: 1.5rem; display: flex; flex-direction: column; border: 1px solid rgba(255,255,255,0.05); }
.remarks-list { flex: 1; overflow-y: auto; padding-right: 8px; display: flex; flex-direction: column; gap: 1.2rem; margin-top: 1rem; }

.remark-group-label { font-size: 0.75rem; font-weight: 900; opacity: 0.4; letter-spacing: 2px; color: var(--primary-color); text-transform: uppercase; margin-bottom: 0.5rem; }

.remark-item-card { 
  background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 20px; 
  padding: 1.4rem; 
  transition: all 0.3s; 
  position: relative;
}
.remark-item-card.drag-over { 
  background: rgba(var(--primary-rgb), 0.08); 
  border-color: var(--primary-color); 
  box-shadow: 0 0 30px rgba(var(--primary-rgb), 0.3); 
  transform: scale(1.02);
}

.remark-card-header { display: flex; justify-content: space-between; align-items: start; margin-bottom: 1rem; }
.remark-title { font-weight: 800; font-size: 1.1rem; color: #fff; cursor: pointer; }
.remark-title:hover { color: var(--primary-color); text-shadow: 0 0 10px rgba(var(--primary-rgb), 0.5); }

.remark-actions { display: flex; gap: 8px; }
.act-btn { background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.05); border-radius: 8px; width: 34px; height: 34px; font-size: 0.9rem; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s; color: #fff; }
.act-btn:hover { background: rgba(255,255,255,0.1); border-color: rgba(255,255,255,0.2); transform: translateY(-2px); }
.act-btn.del:hover { background: rgba(231, 76, 60, 0.2); border-color: #e74c3c; color: #e74c3c; }

.remark-card-body { background: rgba(0,0,0,0.3); border-radius: 14px; padding: 1.2rem; border: 1px solid rgba(255,255,255,0.03); }
.body-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.body-header .label { font-size: 0.65rem; opacity: 0.3; font-weight: 900; text-transform: uppercase; color: #fff; }

.mini-mode-switch { display: flex; gap: 2px; background: rgba(0,0,0,0.3); padding: 4px; border-radius: 10px; border: 1px solid rgba(255,255,255,0.05); }
.mini-mode-switch button { background: none; border: none; font-size: 0.65rem; color: #fff; padding: 4px 10px; border-radius: 6px; cursor: pointer; opacity: 0.4; font-weight: 800; }
.mini-mode-switch button.active { background: var(--primary-color); opacity: 1; box-shadow: 0 4px 10px rgba(var(--primary-rgb), 0.3); }

.sidebar-md-box { font-size: 0.95rem; color: #ddd; line-height: 1.6; max-height: 200px; overflow-y: auto; }
.sidebar-md-box :deep(h1), .sidebar-md-box :deep(h2) { font-size: 1.1rem; margin: 1rem 0 0.5rem; color: var(--primary-color); }
.sidebar-txt-box { font-size: 0.95rem; opacity: 0.7; color: #eee; white-space: pre-wrap; }

.items-count { margin-top: 15px; font-size: 0.75rem; font-weight: 900; color: var(--primary-color); cursor: pointer; opacity: 0.6; transition: 0.2s; text-transform: uppercase; letter-spacing: 0.5px; }
.items-count:hover { opacity: 1; text-decoration: underline; letter-spacing: 1px; }

/* SIDEBAR SPECIFIC STYLES */

.global-zoom { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.92); z-index: 6000; display: flex; align-items: center; justify-content: center; cursor: zoom-out; backdrop-filter: blur(20px); }
.global-zoom img { max-width: 92vw; max-height: 92vh; border-radius: 16px; box-shadow: 0 0 100px rgba(0,0,0,0.8); }
.close-zoom { position: absolute; top: 3rem; right: 3rem; font-size: 2.5rem; color: #fff; cursor: pointer; opacity: 0.5; transition: 0.2s; }
.close-zoom:hover { opacity: 1; transform: rotate(90deg); }

.custom-scrollbar::-webkit-scrollbar { width: 8px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.08); border-radius: 10px; }
</style>
