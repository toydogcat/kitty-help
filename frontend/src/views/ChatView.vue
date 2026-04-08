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

// Bot Sender Logic
const botForm = ref({
  platform: 'line',
  content: '',
  targetId: '',
  file: null as File | null
});
const sendingBotMsg = ref(false);
const filePreviewUrl = ref('');
const platforms = [
  { id: 'telegram', name: 'Telegram', icon: '✈️', color: '#0088cc' },
  { id: 'discord', name: 'Discord', icon: '💬', color: '#5865F2' },
  { id: 'line', name: 'LINE', icon: '🟢', color: '#00B900' }
];

const onFileChange = (e: any) => {
  const file = e.target.files[0];
  if (file) {
    botForm.value.file = file;
    filePreviewUrl.value = URL.createObjectURL(file);
  } else {
    botForm.value.file = null;
    filePreviewUrl.value = '';
  }
};

const sendBotMessage = async () => {
  if (!botForm.value.content && !botForm.value.file) {
    alert("Please enter content or select a file.");
    return;
  }
  
  sendingBotMsg.value = true;
  try {
    await apiService.sendBotMessage(
      botForm.value.platform,
      botForm.value.content,
      botForm.value.file,
      botForm.value.targetId
    );
    // Success!
    botForm.value.content = '';
    botForm.value.file = null;
    filePreviewUrl.value = '';
    await fetchRecentMessages();
  } catch (err: any) {
    alert("Send failed: " + (err.response?.data?.error || err.message));
  } finally {
    sendingBotMsg.value = false;
  }
};

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
    <!-- Left Panel: Bot Sender Control -->
    <div class="left-panel">
      <div class="bot-sender-card shadow-lg">
        <div class="card-header">
           <span class="icon">🤖</span>
           <h3>Bot Dispatcher</h3>
           <div class="live-indicator" :class="{ sending: sendingBotMsg }">
              <span class="dot"></span> {{ sendingBotMsg ? 'DISPATCHING...' : 'READY' }}
           </div>
        </div>

        <div class="card-body">
           <div class="form-group">
              <label>SELECT PLATFORM</label>
              <div class="platform-chips">
                <button 
                  v-for="p in platforms" 
                  :key="p.id"
                  class="p-chip"
                  :class="{ active: botForm.platform === p.id }"
                  :style="{ '--p-color': p.color }"
                  @click="botForm.platform = p.id"
                >
                  <span class="p-icon">{{ p.icon }}</span>
                  <span class="p-name">{{ p.name }}</span>
                </button>
              </div>
           </div>

           <div class="form-group">
              <label>TARGET ID (OPTIONAL)</label>
              <input 
                type="text" 
                v-model="botForm.targetId" 
                placeholder="Default to Admin/Storehouse"
                class="compact-input"
              />
           </div>

           <div class="form-group flex-grow">
              <label>MESSAGE CONTENT</label>
              <textarea 
                v-model="botForm.content" 
                placeholder="Type once-off message here..."
                @keyup.ctrl.enter="sendBotMessage"
              ></textarea>
           </div>

           <div class="form-group media-upload">
              <label>ATTACH MEDIA</label>
              <div class="upload-area" :class="{ has_file: !!filePreviewUrl }">
                <input type="file" @change="onFileChange" accept="image/*,video/*,application/pdf" id="bot-file-input" />
                <label for="bot-file-input" class="upload-label">
                  <div v-if="filePreviewUrl" class="preview-wrap">
                    <img :src="filePreviewUrl" />
                    <div class="remove-btn" @click.stop.prevent="botForm.file = null; filePreviewUrl = ''">✕</div>
                  </div>
                  <div v-else class="placeholder">
                    <span class="icon">📁</span>
                    <span>Browse or Drop</span>
                  </div>
                </label>
              </div>
           </div>

           <button class="send-btn" :disabled="sendingBotMsg" @click="sendBotMessage">
              <span v-if="!sendingBotMsg">🚀 DISPATCH MESSAGE</span>
              <span v-else class="loading-spin"></span>
           </button>
        </div>
      </div>
      
      <!-- Quick Info / Status -->
      <div class="bot-status-mini">
         <div class="status-row">
            <span class="lab">Auto-Sync</span>
            <span class="val enabled">ACTIVE</span>
         </div>
         <div class="status-row">
            <span class="lab">Worker Status</span>
            <span class="val enabled">ONLINE</span>
         </div>
      </div>
    </div>

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
.chat-view { display: flex; height: calc(100vh - 100px); gap: 1rem; padding: 1rem; background: var(--bg-darker); }

/* Left Panel: Bot Sender */
.left-panel { width: 320px; display: flex; flex-direction: column; gap: 1rem; }
.bot-sender-card { 
  background: rgba(255,255,255,0.04); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 28px; 
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  flex: 1;
}
.bot-sender-card .card-header { display: flex; align-items: center; gap: 0.8rem; }
.bot-sender-card .card-header h3 { font-size: 1rem; font-weight: 800; color: #fff; text-transform: uppercase; letter-spacing: 1px; }
.live-indicator { margin-left: auto; font-size: 0.6rem; font-weight: 900; background: rgba(0,0,0,0.3); padding: 4px 10px; border-radius: 20px; display: flex; align-items: center; gap: 6px; color: #00B900; }
.live-indicator.sending { color: var(--primary-color); }
.live-indicator .dot { width: 6px; height: 6px; background: currentColor; border-radius: 50%; box-shadow: 0 0 10px currentColor; }
.live-indicator.sending .dot { animation: blink 0.6s infinite; }

@keyframes blink { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }

.bot-sender-card .card-body { flex: 1; display: flex; flex-direction: column; gap: 1.2rem; }
.form-group { display: flex; flex-direction: column; gap: 0.5rem; }
.form-group label { font-size: 0.65rem; font-weight: 900; opacity: 0.3; letter-spacing: 1px; color: #fff; }

.platform-chips { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.p-chip { 
  flex: 1; 
  min-width: 80px; 
  background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.05); 
  padding: 0.5rem; 
  border-radius: 12px; 
  cursor: pointer; 
  display: flex; 
  flex-direction: column; 
  align-items: center; 
  gap: 4px;
  transition: all 0.2s;
}
.p-chip .p-icon { font-size: 1.2rem; }
.p-chip .p-name { font-size: 0.6rem; font-weight: 900; color: #fff; opacity: 0.5; }
.p-chip.active { background: rgba(var(--primary-rgb), 0.1); border-color: var(--p-color); box-shadow: 0 4px 15px rgba(0,0,0,0.2); }
.p-chip.active .p-name { opacity: 1; color: var(--p-color); }

.compact-input { background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.06); border-radius: 10px; padding: 0.6rem 0.8rem; color: #fff; font-size: 0.8rem; outline: none; }
textarea { 
  background: rgba(0,0,0,0.2); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 14px; 
  padding: 1rem; 
  color: #fff; 
  font-size: 0.9rem; 
  outline: none; 
  resize: none; 
  flex: 1; 
  min-height: 120px;
  line-height: 1.5;
}
textarea:focus { border-color: var(--primary-color); background: rgba(0,0,0,0.3); }

.upload-area { border: 2px dashed rgba(255,255,255,0.08); border-radius: 14px; position: relative; transition: 0.2s; }
.upload-area:hover { border-color: rgba(255,255,255,0.2); background: rgba(255,255,255,0.02); }
#bot-file-input { display: none; }
.upload-label { cursor: pointer; display: flex; align-items: center; justify-content: center; min-height: 100px; width: 100%; }
.placeholder { display: flex; flex-direction: column; align-items: center; gap: 0.4rem; font-size: 0.75rem; opacity: 0.4; font-weight: 800; }
.preview-wrap { position: relative; width: 100%; height: 100%; padding: 10px; }
.preview-wrap img { width: 100%; height: 80px; object-fit: cover; border-radius: 8px; }
.remove-btn { position: absolute; top: 0; right: 0; background: #e74c3c; color: #fff; width: 18px; height: 18px; border-radius: 50%; font-size: 0.6rem; display: flex; align-items: center; justify-content: center; cursor: pointer; }

.send-btn { 
  background: var(--primary-color); 
  color: #000; 
  border: none; 
  border-radius: 14px; 
  padding: 1rem; 
  font-weight: 800; 
  font-size: 0.85rem; 
  cursor: pointer; 
  transition: all 0.3s;
  box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.3);
}
.send-btn:hover:not(:disabled) { transform: translateY(-2px); box-shadow: 0 8px 25px rgba(var(--primary-rgb), 0.4); filter: brightness(1.1); }
.send-btn:disabled { opacity: 0.5; cursor: not-allowed; filter: grayscale(1); }

.bot-status-mini { background: rgba(0,0,0,0.1); border-radius: 14px; padding: 1rem; border: 1px solid rgba(255,255,255,0.03); }
.status-row { display: flex; justify-content: space-between; font-size: 0.65rem; font-weight: 900; margin-bottom: 4px; }
.status-row .lab { opacity: 0.3; }
.status-row .val.enabled { color: #00B900; }

.center-panel { flex: 2; background: rgba(0,0,0,0.2); border-radius: 28px; border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; overflow: hidden; }
/* ... Rest remains similar ... */
.search-header { 
  padding: 1.5rem 2rem; 
  background: rgba(255,255,255,0.02); 
  border-bottom: 1px solid rgba(255,255,255,0.05); 
  display: flex; 
  flex-direction: column; 
  gap: 1.2rem;
}
.header-top { display: flex; justify-content: space-between; align-items: center; }
.header-top h2 { font-size: 1.1rem; font-weight: 800; color: #fff; }
.quick-stats { font-size: 0.65rem; opacity: 0.4; font-weight: 800; color: var(--primary-color); text-transform: uppercase; }

.filter-bar { display: flex; gap: 0.6rem; flex-wrap: wrap; align-items: flex-end; }
.f-group { display: flex; flex-direction: column; gap: 0.4rem; min-width: 90px; }
.f-group label { font-size: 0.55rem; font-weight: 900; opacity: 0.3; text-transform: uppercase; letter-spacing: 1px; color: #fff; }
.f-group select, .f-group input { 
  background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 10px; 
  padding: 0.5rem 0.7rem; 
  color: #fff; 
  font-size: 0.75rem; 
  outline: none;
  transition: all 0.2s;
}

.messages-list { flex: 1; padding: 1.5rem; overflow-y: auto; display: flex; flex-direction: column; gap: 1rem; }

.msg-bubble { 
  max-width: 95%; 
  background: rgba(255,255,255,0.04); 
  border-radius: 20px; 
  padding: 1rem 1.4rem; 
  border: 1px solid rgba(255,255,255,0.06); 
  display: flex; 
  flex-direction: column; 
  gap: 0.3rem;
}

/* Right Panel Refactored */
.right-panel { width: 380px; display: flex; flex-direction: column; }
.remarks-section { flex: 1; min-height: 0; background: rgba(0,0,0,0.2); border-radius: 28px; padding: 1.2rem; display: flex; flex-direction: column; border: 1px solid rgba(255,255,255,0.05); }
.remarks-list { flex: 1; overflow-y: auto; padding-right: 4px; display: flex; flex-direction: column; gap: 1rem; margin-top: 1rem; }

.remark-group-label { font-size: 0.65rem; font-weight: 900; opacity: 0.4; letter-spacing: 2px; color: var(--primary-color); text-transform: uppercase; margin-bottom: 0.3rem; }

.remark-item-card { 
  background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.06); 
  border-radius: 18px; 
  padding: 1.2rem; 
  transition: all 0.3s; 
}

.remark-title { font-weight: 800; font-size: 0.95rem; color: #fff; cursor: pointer; }

.act-btn { width: 30px; height: 30px; font-size: 0.8rem; }

.remark-card-body { padding: 1rem; }
.sidebar-md-box { font-size: 0.85rem; }
.sidebar-txt-box { font-size: 0.85rem; }

.items-count { font-size: 0.65rem; }

.global-zoom { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.92); z-index: 6000; display: flex; align-items: center; justify-content: center; cursor: zoom-out; backdrop-filter: blur(20px); }
.global-zoom img { max-width: 92vw; max-height: 92vh; border-radius: 16px; box-shadow: 0 0 100px rgba(0,0,0,0.8); }
.close-zoom { position: absolute; top: 3rem; right: 3rem; font-size: 2.5rem; color: #fff; cursor: pointer; opacity: 0.5; transition: 0.2s; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.05); border-radius: 10px; }
</style>
