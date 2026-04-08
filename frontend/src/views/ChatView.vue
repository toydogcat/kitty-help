<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, defineAsyncComponent } from 'vue';
import { apiService, socket } from '../services/api';
import { syncService } from '../services/syncService';
import { db } from '../services/localDb';
import { liveQuery } from 'dexie';
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

// Bot Sender Logic (B-side)
const botForm = ref({
  platform: 'line',
  content: '',
  targetId: '',
  files: [] as File[]
});
const localUploads = ref<any[]>([]);
const selectedLocalFiles = ref<Set<string>>(new Set());
const sendingBotMsg = ref(false);
const localUploadsLoading = ref(false);

const platforms = [
  { id: 'telegram', name: 'Telegram', icon: '✈️', color: '#0088cc' },
  { id: 'discord', name: 'Discord', icon: '💬', color: '#5865F2' },
  { id: 'line', name: 'LINE', icon: '🟢', color: '#00B900' }
];

const onFilesChange = (e: any) => {
  const files = Array.from(e.target.files) as File[];
  botForm.value.files = files;
};

const toggleLocalFile = (name: string) => {
  if (selectedLocalFiles.value.has(name)) {
    selectedLocalFiles.value.delete(name);
  } else {
    selectedLocalFiles.value.add(name);
  }
};

const fetchLocalUploads = async () => {
  localUploadsLoading.value = true;
  try {
    const data = await apiService.getBotUploads();
    localUploads.value = data || [];
  } catch (err) {
    console.error("Local uploads fetch failed");
  } finally {
    localUploadsLoading.value = false;
  }
};

const sendBotMessage = async () => {
  if (!botForm.value.content && botForm.value.files.length === 0 && selectedLocalFiles.value.size === 0) {
    alert("Please enter content or select files.");
    return;
  }
  
  sendingBotMsg.value = true;
  try {
    const formData = new FormData();
    formData.append('platform', botForm.value.platform);
    formData.append('content', botForm.value.content);
    formData.append('targetId', botForm.value.targetId);
    
    botForm.value.files.forEach(f => formData.append('files', f));
    formData.append('selectedFiles', Array.from(selectedLocalFiles.value).join(','));

    await apiService.sendBotMessageMulti(formData);
    
    botForm.value.content = '';
    botForm.value.files = [];
    selectedLocalFiles.value.clear();
    await Promise.all([fetchRecentMessages(), fetchLocalUploads()]);
  } catch (err: any) {
    alert("Send failed: " + (err.response?.data?.error || err.message));
  } finally {
    sendingBotMsg.value = false;
  }
};

// --- Standard Chat Logic ---
const remarkEditModes = ref<Record<string, 'preview' | 'edit'>>({});
const showRemarkModal = ref(false);
const editingRemark = ref<any>(null);
const remarkEditBuffer = ref({ title: '', content: '' });
const { toggleRemarkSidebarPin, pinToDesk } = usePin();
const zoomedImageUrl = ref('');
const dragOverRemarkId = ref<string | null>(null);
const remarkModalDetails = ref<any>(null);

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  return apiService.getStorehouseFileUrl(mediaId, platform || 'line');
};

const fetchRecentMessages = async () => {
  loading.value = true;
  try {
    const results = await apiService.getChatLogs(filters.value.platform, filters.value.q, filters.value.startDate, filters.value.endDate);
    recentMessages.value = results.slice(0, filters.value.limit);
  } catch (err) { 
    console.error("Fetch failed:", err); 
  } finally { 
    loading.value = false; 
  }
};

let remarksObservable: any = null;

onMounted(() => {
  fetchRecentMessages();
  fetchLocalUploads();
  socket.on('messagesUpdate', fetchRecentMessages);

  // Sync Remarks with offline DB
  syncService.refreshRemarks();
  remarksObservable = liveQuery(() => db.remarks.toArray()).subscribe({
    next: (data) => {
      remarkContainers.value = data;
      data.forEach(c => {
         if (!remarkEditModes.value[c.id]) remarkEditModes.value[c.id] = 'preview';
      });
    }
  });
});

onUnmounted(() => {
  if (remarksObservable) remarksObservable.unsubscribe();
  socket.off('messagesUpdate', fetchRecentMessages);
});

// Drag & Drop
const onDragStart = (e: DragEvent, item: any) => {
  e.dataTransfer?.setData('application/json', JSON.stringify({ type: 'media', data: item }));
};

const handleDropOnRemark = async (e: DragEvent, containerId: string) => {
  e.preventDefault();
  dragOverRemarkId.value = null;
  const raw = e.dataTransfer?.getData('application/json');
  if (!raw) return;
  const payload = JSON.parse(raw);
  if (payload.type === 'media') {
    try {
      await syncService.addRemarkItem({ containerId, logId: payload.data.id });
    } catch (err) { alert("Failed to add to remark"); }
  }
};

const openRemarkModal = async (c: any) => {
  editingRemark.value = c;
  const items = await db.remarkItems.where('containerId').equals(c.id).toArray();
  remarkModalDetails.value = { ...c, items };
  remarkEditBuffer.value = { title: c.name, content: c.content || '' };
  showRemarkModal.value = true;
};

const handleSaveRemark = async (data: any) => {
    try {
        if (editingRemark.value) {
            await syncService.updateRemark(editingRemark.value.id, {
                name: data.title,
                content: data.content
            });
        } else {
            await syncService.createRemark({
                name: data.title,
                content: data.content
            });
        }
        showRemarkModal.value = false;
    } catch (err) {
        alert("Save failed");
    }
};

const pinnedRemarks = computed(() => remarkContainers.value.filter(c => c.isPinned));
const otherRemarks = computed(() => remarkContainers.value.filter(c => !c.isPinned));

const togglePin = async (c: any) => {
  try {
    await toggleRemarkSidebarPin(c.id, c.isPinned);
  } catch (err) { alert("Pin toggle failed"); }
};

const addToDesk = async (c: any) => {
  try {
    await pinToDesk('remark', c.id);
    alert("Pinned to Desk! 📌");
  } catch (err) { alert("Failed to pin to desk"); }
};

const deleteRemark = async (id: string) => {
  if (!confirm("Delete this group?")) return;
  try {
    await syncService.deleteRemark(id);
  } catch (err) { alert("Delete failed"); }
};

const copyRemark = (c: any) => {
  const text = (c.content || "") + "\n\n--- Items ---\n" + 
               (c.items || []).map((i: any) => `[${i.log?.platform}] ${i.log?.senderName}: ${i.log?.content}`).join("\n");
  navigator.clipboard.writeText(text);
  alert("Copied to clipboard!");
};

const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};
</script>

<template>
  <div class="chat-container">
    <!-- LEFT: BOT DISPATCHER -->
    <aside class="left-sidebar shadow-xl">
      <div class="bot-card">
        <header class="card-header">
           <span class="pulse-icon">🤖</span>
           <h2>Bot Dispatcher</h2>
           <div class="badge" :class="{ busy: sendingBotMsg }">{{ sendingBotMsg ? 'WORKING' : 'LIVE' }}</div>
        </header>

        <section class="bot-form">
           <div class="control-group">
              <label>PLATFORM</label>
              <div class="platform-selector">
                <button v-for="p in platforms" :key="p.id" 
                  class="plat-btn" :class="{ active: botForm.platform === p.id }"
                  :style="{ '--c': p.color }" @click="botForm.platform = p.id">
                  {{ p.icon }} <span>{{ p.name }}</span>
                </button>
              </div>
           </div>

           <div class="control-group">
              <label>TARGET CHANNEL/ID</label>
              <input v-model="botForm.targetId" placeholder="Default linked ID" class="glow-input" />
           </div>

           <div class="control-group">
              <label>MESSAGE</label>
              <textarea v-model="botForm.content" placeholder="Type a message..." class="glow-input"></textarea>
           </div>

           <div class="control-group">
              <label>NEW MEDIA (B-SIDE)</label>
              <div class="upload-trigger">
                <input type="file" multiple @change="onFilesChange" id="file-up" />
                <label for="file-up" class="up-box">
                  <span v-if="botForm.files.length === 0">📤 Upload New</span>
                  <span v-else>✅ {{ botForm.files.length }} files staged</span>
                </label>
              </div>
           </div>

           <button class="dispatch-btn" :disabled="sendingBotMsg" @click="sendBotMessage">
              <span v-if="!sendingBotMsg">🚀 DISPATCH</span>
              <span v-else class="loader"></span>
           </button>
        </section>

        <!-- MEDIA LIBRARY (STAGING AREA) -->
        <section class="media-library">
          <header class="lib-header">
            <label>MEDIA LIBRARY (暫存區)</label>
            <button @click="fetchLocalUploads" class="refresh-btn">🔄</button>
          </header>
          <div class="lib-list custom-scrollbar">
            <div v-if="localUploadsLoading" class="lib-loading">Scanning workspace...</div>
            <div v-for="f in localUploads" :key="f.name" 
                 class="lib-item" :class="{ selected: selectedLocalFiles.has(f.name) }"
                 @click="toggleLocalFile(f.name)">
              <span class="file-icon">{{ f.type === '.jpg' || f.type === '.png' ? '🖼️' : '📄' }}</span>
              <div class="file-meta">
                <span class="file-name">{{ f.name }}</span>
                <span class="file-info">{{ formatSize(f.size) }}</span>
              </div>
              <div class="checkbox">{{ selectedLocalFiles.has(f.name) ? '✓' : '' }}</div>
            </div>
            <div v-if="localUploads.length === 0 && !localUploadsLoading" class="lib-empty">No files in uploads/</div>
          </div>
        </section>
      </div>
    </aside>

    <!-- CENTER: UNIFIED TERMINAL -->
    <main class="terminal-main">
      <header class="terminal-header">
        <div class="title-row">
          <h1> Unified Terminal</h1>
          <div class="count-tag">{{ recentMessages.length }} MESSAGES</div>
        </div>
        
        <div class="filter-row">
          <select v-model="filters.platform" @change="fetchRecentMessages" class="f-select">
            <option value="">All</option>
            <option value="line">LINE</option>
            <option value="telegram">Telegram</option>
            <option value="discord">Discord</option>
          </select>
          <input type="text" v-model="filters.q" placeholder="Search logs..." @keyup.enter="fetchRecentMessages" class="f-input" />
          <input type="date" v-model="filters.startDate" @change="fetchRecentMessages" class="f-date" />
          <select v-model="filters.limit" @change="fetchRecentMessages" class="f-limit">
            <option :value="50">50</option>
            <option :value="100">100</option>
          </select>
        </div>
      </header>

      <div class="messages-flow custom-scrollbar">
        <div v-for="m in recentMessages" :key="m.id" class="msg-wrapper" :class="m.platform">
          <div class="msg-card-plus" draggable="true" @dragstart="onDragStart($event, m)">
            <header class="msg-card-header">
              <span class="badge-mini" :style="{ background: m.platform === 'line' ? '#00B900' : m.platform === 'discord' ? '#5865F2' : '#0088cc' }">
                {{ m.platform }}
              </span>
              <span class="author">{{ m.senderName }}</span>
              <span class="ts">{{ m.createdAt ? new Date(m.createdAt).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'}) : '' }}</span>
            </header>

            <div v-if="m.mediaId" class="msg-attachment">
               <div v-if="m.msgType === 'media' || m.content.includes('(File:')" class="image-preview" @click="zoomedImageUrl = getStorehouseUrl(m.mediaId, m.platform)">
                 <img :src="getStorehouseUrl(m.mediaId, m.platform)" loading="lazy" />
                 <div class="hover-overlay">VIEW</div>
               </div>
               <div v-else class="file-dummy" @click="zoomedImageUrl = getStorehouseUrl(m.mediaId, m.platform)">
                 📎 Attached File: {{ m.mediaId }}
               </div>
            </div>

            <div class="msg-body" v-html="marked.parse(m.content || '')"></div>
          </div>
        </div>
      </div>
    </main>

    <!-- RIGHT: RESOURCE REPOSITORY -->
    <aside class="right-sidebar shadow-xl">
      <div class="repo-card">
        <header class="repo-header">
          <label>RESOURCE REPOSITORY (知識庫)</label>
          <div class="title-act">
            <h2>🔖 Remarks</h2>
            <button class="new-btn" @click="showRemarkModal = true">+ NEW</button>
          </div>
        </header>

        <div class="repo-list custom-scrollbar">
          <div v-if="pinnedRemarks.length" class="section-label">PINNED (釘選)</div>
          <div v-for="c in pinnedRemarks" :key="c.id" class="remark-box pinned" 
               @drop="handleDropOnRemark($event, c.id)" @dragover.prevent @click="openRemarkModal(c)">
            <div class="r-top">
              <span class="r-title">{{ c.name }}</span>
              <span class="r-pin">📌</span>
            </div>
            <div class="r-preview" v-html="marked.parse(c.content || '...')"></div>
            <footer class="r-foot">🔗 {{ c.items?.length || 0 }} items</footer>
          </div>

          <div class="section-label">COLLECTIONS (全部)</div>
          <div v-for="c in otherRemarks" :key="c.id" class="remark-box"
               @drop="handleDropOnRemark($event, c.id)" @dragover.prevent @click="openRemarkModal(c)">
            <div class="r-top">
              <span class="r-title">{{ c.name }}</span>
            </div>
            <div class="r-preview" v-html="marked.parse(c.content || '...')"></div>
            <footer class="r-foot">🔗 {{ c.items?.length || 0 }} items</footer>
          </div>
        </div>
      </div>
    </aside>

    <!-- MODALS -->
    <UnifiedRemarkModal 
      :show="showRemarkModal" 
      :item="{ ...editingRemark, type: 'remark' }"
      :details="remarkModalDetails"
      @close="showRemarkModal = false" 
      @save="fetchRecentMessages"
    />
    <Teleport to="body">
      <div v-if="zoomedImageUrl" class="zoom-portal" @click="zoomedImageUrl = ''">
        <img :src="zoomedImageUrl" />
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.chat-container {
  display: flex;
  height: 100vh;
  padding: 1rem;
  gap: 1.5rem;
  background: #08080c;
  color: #e0e0e0;
  font-family: 'Inter', sans-serif;
  overflow: hidden;
}

/* SIDEBARS */
.left-sidebar, .right-sidebar {
  width: 330px;
  display: flex;
  flex-direction: column;
  background: rgba(20, 20, 25, 0.4);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255,255,255,0.05);
  border-radius: 32px;
  overflow: hidden;
}
.right-sidebar { width: 400px; }

/* BOT CARD */
.bot-card, .repo-card {
  height: 100%;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
}

.card-header, .repo-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}
.card-header h2, .repo-header h2 {
  font-size: 1rem; font-weight: 800; color: #fff; margin: 0; text-transform: uppercase; letter-spacing: 1px;
}
.badge {
  margin-left: auto; font-size: 0.6rem; font-weight: 900;
  padding: 2px 8px; border-radius: 20px; color: #00B900; background: rgba(0,255,0,0.1);
}
.badge.busy { color: #ffae00; background: rgba(255,174,0,0.1); }

/* FORM */
.bot-form { display: flex; flex-direction: column; gap: 0.8rem; }
.control-group { display: flex; flex-direction: column; gap: 0.3rem; }
.control-group label { font-size: 0.55rem; font-weight: 800; opacity: 0.4; letter-spacing: 1px; }

.platform-selector { display: flex; gap: 0.4rem; }
.plat-btn {
  flex: 1; padding: 0.5rem; border-radius: 12px; background: rgba(255,255,255,0.03); 
  border: 1px solid rgba(255,255,255,0.05); color: #fff; cursor: pointer; transition: 0.2s;
  display: flex; align-items: center; justify-content: center; gap: 6px; font-size: 0.7rem;
}
.plat-btn.active { background: rgba(var(--c), 0.1); border-color: var(--c); color: var(--c); box-shadow: 0 0 10px var(--c); }

.glow-input {
  background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.08); border-radius: 12px;
  padding: 0.7rem; color: #fff; font-size: 0.8rem; outline: none; transition: 0.2s;
}
.glow-input:focus { border-color: #6366f1; box-shadow: 0 0 10px rgba(99,102,241,0.2); }
textarea.glow-input { min-height: 60px; resize: none; }

.up-box {
  display: block; padding: 0.8rem; border: 2px dashed rgba(255,255,255,0.1); border-radius: 12px;
  text-align: center; font-size: 0.7rem; font-weight: 700; cursor: pointer; transition: 0.2s;
}
.up-box:hover { background: rgba(255,255,255,0.02); border-color: rgba(255,255,255,0.3); }

.dispatch-btn {
  background: #6366f1; color: #fff; border: none;
  padding: 0.8rem; border-radius: 16px; font-weight: 900; letter-spacing: 1px; cursor: pointer;
  box-shadow: 0 6px 15px rgba(99,102,241,0.2); transition: 0.3s;
}
.dispatch-btn:hover { transform: translateY(-2px); box-shadow: 0 10px 20px rgba(99,102,241,0.3); }

/* MEDIA LIBRARY */
.media-library { flex: 1; display: flex; flex-direction: column; min-height: 0; background: rgba(0,0,0,0.15); border-radius: 20px; padding: 0.8rem; }
.lib-header { display: flex; align-items: center; justify-content: space-between; padding-bottom: 0.4rem; }
.lib-header label { font-size: 0.6rem; font-weight: 900; color: #6366f1; text-transform: uppercase; }
.refresh-btn { background: none; border: none; cursor: pointer; font-size: 0.7rem; opacity: 0.5; }
.lib-list { flex: 1; overflow-y: auto; padding-right: 4px; }
.lib-item {
  display: flex; align-items: center; gap: 0.6rem; padding: 0.5rem; border-radius: 10px;
  cursor: pointer; margin-bottom: 0.3rem; transition: 0.2s; border: 1px solid transparent;
}
.lib-item:hover { background: rgba(255,255,255,0.03); }
.lib-item.selected { background: rgba(99,102,241,0.1); border-color: rgba(99,102,241,0.3); }
.file-name { font-size: 0.7rem; font-weight: 600; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.2; }
.file-info { font-size: 0.55rem; opacity: 0.4; display: block; }
.checkbox { width: 14px; height: 14px; border: 1.5px solid rgba(255,255,255,0.1); border-radius: 3px; display: flex; align-items: center; justify-content: center; font-size: 9px; color: #6366f1; }

/* MAIN TERMINAL */
.terminal-main { flex: 1; display: flex; flex-direction: column; min-width: 0; background: rgba(0,0,0,0.2); border-radius: 32px; border: 1px solid rgba(255,255,255,0.03); }
.terminal-header { padding: 1.5rem 2rem; border-bottom: 1px solid rgba(255,255,255,0.03); }
.title-row h1 { font-size: 1.2rem; font-weight: 900; margin: 0; }
.count-tag { font-size: 0.55rem; font-weight: 900; padding: 3px 10px; background: rgba(255,255,255,0.04); border-radius: 8px; opacity: 0.4; }

.filter-row { display: flex; gap: 0.8rem; margin-top: 1rem; }
.f-input { flex: 1; }
.f-input, .f-select, .f-date, .f-limit {
  background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.05);
  border-radius: 10px; padding: 0.5rem 0.8rem; color: #fff; font-size: 0.75rem;
}

.messages-flow { flex: 1; overflow-y: auto; padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.msg-card-plus {
  background: rgba(255,255,255,0.02); border-radius: 20px; padding: 1rem;
  border: 1px solid rgba(255,255,255,0.04); display: flex; flex-direction: column; gap: 0.5rem;
}
.msg-card-plus:hover { background: rgba(255,255,255,0.03); }
.msg-card-header { display: flex; align-items: center; gap: 0.6rem; }
.badge-mini { font-size: 0.5rem; font-weight: 900; padding: 2px 6px; border-radius: 4px; text-transform: uppercase; }
.author { font-weight: 700; font-size: 0.85rem; color: #fff; }
.ts { font-size: 0.65rem; opacity: 0.3; margin-left: auto; }

.msg-attachment { margin-top: 0.5rem; }
.image-preview { border-radius: 12px; overflow: hidden; max-width: 320px; transition: 0.2s; cursor: zoom-in; position: relative; }
.image-preview:hover { transform: scale(1.02); }
.file-dummy { font-size: 0.75rem; color: #6366f1; background: rgba(99,102,241,0.1); padding: 0.5rem 1rem; border-radius: 10px; cursor: pointer; }

/* REPO */
.repo-header .title-act { display: flex; align-items: center; justify-content: space-between; width: 100%; margin-top: 0.3rem; }
.repo-list { flex: 1; overflow-y: auto; padding-right: 4px; }
.section-label { font-size: 0.55rem; font-weight: 900; color: #6366f1; margin: 1rem 0 0.5rem; letter-spacing: 1px; }
.remark-box {
  background: rgba(255,255,255,0.02); border-radius: 18px; padding: 1rem;
  border: 1px solid rgba(255,255,255,0.04); margin-bottom: 0.8rem; cursor: pointer;
}
.remark-box:hover { border-color: #6366f1; background: rgba(99,102,241,0.03); }
.r-title { font-weight: 800; font-size: 0.9rem; color: #fff; }
.r-preview { font-size: 0.75rem; opacity: 0.4; margin-top: 0.4rem; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.r-foot { font-size: 0.55rem; font-weight: 800; color: #6366f1; margin-top: 0.6rem; }

/* SHARED */
.custom-scrollbar::-webkit-scrollbar { width: 5px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.05); border-radius: 10px; }
.zoom-portal { position: fixed; inset: 0; background: rgba(0,0,0,0.9); z-index: 9999; display: flex; align-items: center; justify-content: center; backdrop-filter: blur(10px); }
.zoom-portal img { max-width: 90vw; max-height: 90vh; border-radius: 16px; }
</style>
