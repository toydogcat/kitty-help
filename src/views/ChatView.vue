<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { apiService, socket } from '../services/api';
import { marked } from 'marked';

// ... (existing state)
const recentMessages = ref<any[]>([]);
const recentPhotos = ref<any[]>([]);
const photoPage = ref(1);
const totalPhotos = ref(0);
const remarkContainers = ref<any[]>([]);
const loading = ref(true);

// Editor Toggle for Remarks in sidebar
const remarkEditModes = ref<Record<string, 'preview' | 'edit'>>({});

// Global Editor for Remarks (Unified with Desk)
const showRemarkModal = ref(false);
const editingRemark = ref<any>(null);
const remarkEditBuffer = ref({ title: '', content: '' });
const remarkModalEditMode = ref<'preview' | 'edit'>('preview');
const remarkModalFullScreen = ref(false);
const savingRemark = ref(false);
const remarkModalDetails = ref<any>(null); // For Quoted Items
const zoomedImageUrl = ref('');

// Drag & Drop
const dragOverRemarkId = ref<string | null>(null);

const fetchData = async () => {
  loading.value = true;
  try {
    const [msgData, photoData, remarkData] = await Promise.all([
      apiService.getRecentMessages(),
      apiService.getRecentPhotos(photoPage.value),
      apiService.getRemarks()
    ]);
    recentMessages.value = msgData;
    recentPhotos.value = photoData.photos || [];
    totalPhotos.value = photoData.total || 0;
    remarkContainers.value = remarkData.containers || [];
    
    // Init edit modes
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
  fetchData();
  socket.on('messagesUpdate', fetchData);
});

const handleDragStart = (e: DragEvent, photo: any) => {
  e.dataTransfer?.setData('application/json', JSON.stringify({ type: 'media', data: photo }));
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
      await fetchData();
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
    await fetchData();
  } catch (err) {
    alert("Creation failed");
  }
};

const togglePin = async (c: any) => {
  try {
    await apiService.updateRemark(c.id, { isPinned: !c.isPinned });
    await fetchData();
  } catch (err) {
    alert("Pin toggle failed");
  }
};

const deleteRemark = async (id: string) => {
  if (!confirm("Delete this group and all its links?")) return;
  try {
    await apiService.deleteRemark(id);
    await fetchData();
  } catch (err) {
    alert("Delete failed");
  }
};

const addToDesk = async (c: any) => {
  try {
    await apiService.addDeskItem({ type: 'remark', refId: c.id, shelfId: null });
    alert("Pinned to Desk! 📌");
  } catch (err) {
    console.error("Failed to pin to desk:", err);
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

const saveRemarkEdit = async () => {
  if (!editingRemark.value) return;
  savingRemark.value = true;
  try {
    await apiService.updateRemark(editingRemark.value.id, {
      name: remarkEditBuffer.value.title,
      content: remarkEditBuffer.value.content
    });
    showRemarkModal.value = false;
    await fetchData();
  } catch (err) {
    alert("Save failed");
  } finally {
    savingRemark.value = false;
  }
};

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  return apiService.getStorehouseFileUrl(mediaId, platform || 'line');
};

const pinnedRemarks = computed(() => remarkContainers.value.filter(c => c.isPinned));
const otherRemarks = computed(() => remarkContainers.value.filter(c => !c.isPinned));
</script>

<template>
  <div class="chat-view">
    <!-- Center Panel: Recent Messages -->
    <div class="center-panel">
      <div class="panel-header">
        <h2>💬 Unified Chat (Recent)</h2>
      </div>
      <div class="messages-list custom-scrollbar">
         <div v-for="m in recentMessages" :key="m.id" class="msg-card" :class="m.platform">
            <div class="msg-bubble shadow-sm">
              <div class="msg-meta">
                <span class="platform-indicator" :class="m.platform"></span>
                <span class="platform-name">{{ m.platform.toUpperCase() }}</span>
                <span class="sender-name">{{ m.senderName }}</span>
                <span class="time">{{ m.createdAt ? new Date(m.createdAt).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'}) : '' }}</span>
              </div>
              <div class="msg-text">{{ m.content }}</div>
            </div>
         </div>
      </div>
    </div>

    <!-- Right Panel: Remarks & Photos -->
    <div class="right-panel">
      <!-- Photos Bucket (Drag source) -->
      <div class="photos-bucket">
        <h3>🖼️ Recent Photos <span class="badge">{{ totalPhotos }}</span></h3>
        <div class="photos-grid custom-scrollbar">
          <div v-for="p in recentPhotos" :key="p.id" class="photo-item" draggable="true" @dragstart="handleDragStart($event, p)">
            <img :src="getStorehouseUrl(p.mediaId, p.platform)" loading="lazy" />
          </div>
        </div>
      </div>

      <!-- Remarks Section -->
      <div class="remarks-section">
        <div class="section-header">
           <h3>📚 Integrated Remarks</h3>
           <button @click="createNewRemark" class="new-remark-btn">+ New Group</button>
        </div>

        <div class="remarks-list custom-scrollbar">
          <!-- Pinned Section -->
          <div v-if="pinnedRemarks.length > 0" class="remark-group-label">✨ Pinned (釘選)</div>
          <div 
            v-for="c in pinnedRemarks" :key="c.id" 
            class="remark-item-card"
            :class="{ 'drag-over': dragOverRemarkId === c.id }"
            @dragover="handleDragOver($event, c.id)" @dragleave="dragOverRemarkId = null" @drop="handleDropOnRemark($event, c.id)"
          >
            <div class="remark-card-header">
               <span class="remark-title" @click="openRemarkModal(c)">{{ c.name }}</span>
               <div class="remark-actions">
                  <button @click="togglePin(c)" class="act-btn">{{ c.isPinned ? '⭐' : '☆' }}</button>
                  <button @click="addToDesk(c)" class="act-btn">📌</button>
                  <button @click="copyRemark(c)" class="act-btn">📋</button>
                  <button @click="deleteRemark(c.id)" class="act-btn del">🗑️</button>
               </div>
            </div>
            <!-- SIDEBAR REMARK CONTENT: Default to MD, with txt toggle -->
            <div class="remark-card-body">
               <div class="body-header">
                  <span class="label">Content</span>
                  <div class="mini-mode-switch">
                    <button :class="{ active: remarkEditModes[c.id] === 'preview' }" @click="remarkEditModes[c.id] = 'preview'">MD</button>
                    <button :class="{ active: remarkEditModes[c.id] === 'edit' }" @click="remarkEditModes[c.id] = 'edit'">TXT</button>
                  </div>
               </div>
               <div v-if="remarkEditModes[c.id] === 'preview'" class="sidebar-md-box" v-html="marked.parse(c.content || 'No description.')"></div>
               <div v-else class="sidebar-txt-box">{{ c.content || 'Empty...' }}</div>
               <div class="items-count" @click="openRemarkModal(c)">🔗 {{ c.items?.length || 0 }} items linked</div>
            </div>
          </div>

          <!-- Other Section -->
          <div v-if="otherRemarks.length > 0" class="remark-group-label">All Remarks</div>
          <div 
            v-for="c in otherRemarks" :key="c.id" 
            class="remark-item-card"
            :class="{ 'drag-over': dragOverRemarkId === c.id }"
            @dragover="handleDragOver($event, c.id)" @dragleave="dragOverRemarkId = null" @drop="handleDropOnRemark($event, c.id)"
          >
            <div class="remark-card-header">
               <span class="remark-title" @click="openRemarkModal(c)">{{ c.name }}</span>
               <div class="remark-actions">
                  <button @click="togglePin(c)" class="act-btn">☆</button>
                  <button @click="addToDesk(c)" class="act-btn">📌</button>
                  <button @click="copyRemark(c)" class="act-btn">📋</button>
                  <button @click="deleteRemark(c.id)" class="act-btn del">🗑️</button>
               </div>
            </div>
            <div class="remark-card-body">
               <div class="body-header">
                  <span class="label">Content</span>
                  <div class="mini-mode-switch">
                    <button :class="{ active: remarkEditModes[c.id] === 'preview' }" @click="remarkEditModes[c.id] = 'preview'">MD</button>
                    <button :class="{ active: remarkEditModes[c.id] === 'edit' }" @click="remarkEditModes[c.id] = 'edit'">TXT</button>
                  </div>
               </div>
               <div v-if="remarkEditModes[c.id] === 'preview'" class="sidebar-md-box" v-html="marked.parse(c.content || '')"></div>
               <div v-else class="sidebar-txt-box">{{ c.content || 'Empty...' }}</div>
               <div class="items-count" @click="openRemarkModal(c)">🔗 {{ c.items?.length || 0 }} items linked</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- UNIFIED REMARK EDITOR MODAL (Same as Desk) -->
    <Teleport to="body">
      <div v-if="showRemarkModal" class="modal-overlay remark-editor-overlay" @click.self="showRemarkModal = false">
        <div class="modal-card wide-editor" :class="{ 'is-full': remarkModalFullScreen }">
          <div class="modal-header">
            <h3>📖 REMARK EDITOR</h3>
            <div class="unified-controls">
               <div class="mode-capsule">
                  <button :class="{ active: remarkModalEditMode === 'preview' }" @click="remarkModalEditMode = 'preview'">MD PREVIEW</button>
                  <button :class="{ active: remarkModalEditMode === 'edit' }" @click="remarkModalEditMode = 'edit'">TXT / EDIT</button>
               </div>
               <div class="action-set">
                  <button @click="remarkModalFullScreen = !remarkModalFullScreen" class="action-item">
                    {{ remarkModalFullScreen ? '❐' : '⛶' }}
                  </button>
                  <button @click="showRemarkModal = false" class="action-item close">✕</button>
               </div>
            </div>
          </div>
          
          <div class="modal-body custom-scrollbar">
            <div class="form-group">
              <label>Title / Category Name</label>
              <input v-model="remarkEditBuffer.title" placeholder="e.g., My Project Notes" />
            </div>

            <div class="form-group fill">
              <label>Notes & Summary (Markdown Supported)</label>
              <div v-if="remarkModalEditMode === 'preview'" class="md-preview-box" v-html="marked.parse(remarkEditBuffer.content || '')"></div>
              <textarea v-else v-model="remarkEditBuffer.content" placeholder="Paste or type details here..."></textarea>
            </div>

            <!-- UNIFIED QUOTED ITEMS GRID -->
            <div class="quoted-section">
              <label class="section-label">📚 Quoted Items (引用項目)</label>
              <div class="quoted-items-grid">
                <div v-for="item in (remarkModalDetails?.items || [])" :key="item.id" class="quoted-item-card">
                  <div class="item-meta-top">
                    <span class="p-slug">{{ item.log?.platform }}</span>
                    <span class="p-user">{{ item.log?.senderName }}</span>
                  </div>
                  <div v-if="item.log?.mediaId && (item.log?.msgType === 'image' || item.log?.content.includes('[Image]'))" class="item-img-box" @click="zoomedImageUrl = getStorehouseUrl(item.log.mediaId, item.log.platform)">
                    <img :src="getStorehouseUrl(item.log.mediaId, item.log.platform)" />
                  </div>
                  <div v-else class="item-text-box">
                    <p>{{ item.log?.content }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button @click="showRemarkModal = false" class="cancel-btn">Discard</button>
            <button @click="saveRemarkEdit" class="save-btn" :disabled="savingRemark">
              {{ savingRemark ? 'Saving...' : '✅ Save Changes' }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="zoomedImageUrl" class="global-zoom" @click="zoomedImageUrl = ''">
         <img :src="zoomedImageUrl" />
         <span class="close-zoom">✕</span>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.chat-view { display: flex; height: calc(100vh - 100px); gap: 1rem; padding: 1rem; }
.right-panel { width: 450px; display: flex; flex-direction: column; gap: 1rem; }
.center-panel { flex: 1; background: rgba(0,0,0,0.2); border-radius: 20px; border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; overflow: hidden; }
.panel-header { padding: 1.5rem; background: rgba(255,255,255,0.02); border-bottom: 1px solid rgba(255,255,255,0.05); }
.messages-list { flex: 1; padding: 1.5rem; overflow-y: auto; display: flex; flex-direction: column; gap: 1rem; }

.msg-card { display: flex; flex-direction: column; align-items: flex-start; width: 100%; transition: all 0.2s; }
.msg-bubble { 
  max-width: 85%; 
  background: rgba(255,255,255,0.05); 
  border-radius: 18px; 
  padding: 1rem 1.4rem; 
  border: 1px solid rgba(255,255,255,0.08); 
  display: flex; 
  flex-direction: column; 
  gap: 0.5rem;
}

.msg-meta { display: flex; align-items: center; gap: 0.8rem; margin-bottom: 2px; }
.platform-indicator { width: 8px; height: 8px; border-radius: 50%; }
.platform-indicator.discord { background: #5865F2; }
.platform-indicator.telegram { background: #0088cc; }
.platform-indicator.line { background: #00B900; }

.platform-name { font-size: 0.7rem; font-weight: 800; opacity: 0.4; letter-spacing: 1px; }
.sender-name { font-weight: 700; color: #fff; font-size: 0.9rem; }
.time { font-size: 0.75rem; opacity: 0.4; margin-left: auto; }

.msg-text { font-size: 1rem; color: #eee; line-height: 1.6; word-break: break-word; }

.foto-badge { background: var(--primary-color); color: #fff; padding: 2px 6px; border-radius: 4px; font-size: 0.7rem; }

/* Photos Bucket */
.photos-bucket { height: 280px; background: rgba(255,255,255,0.03); border-radius: 20px; padding: 1.2rem; display: flex; flex-direction: column; }
.photos-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); gap: 8px; overflow-y: auto; flex: 1; padding-top: 10px; }
.photo-item { border-radius: 8px; overflow: hidden; aspect-ratio: 1; cursor: grab; background: rgba(0,0,0,0.2); transition: transform 0.2s; }
.photo-item:hover { transform: scale(1.05); }
.photo-item img { width: 100%; height: 100%; object-fit: cover; }

/* Remarks Section */
.remarks-section { flex: 1; min-height: 0; background: rgba(255,255,255,0.03); border-radius: 20px; padding: 1.5rem; display: flex; flex-direction: column; }
.remarks-list { flex: 1; overflow-y: auto; padding-right: 8px; display: flex; flex-direction: column; gap: 1.2rem; margin-top: 1rem; }

.remark-group-label { font-size: 0.75rem; font-weight: 800; opacity: 0.5; letter-spacing: 1px; color: var(--primary-color); }

.remark-item-card { 
  background: rgba(255,255,255,0.04); 
  border: 1px solid rgba(255,255,255,0.08); 
  border-radius: 16px; 
  padding: 1.2rem; 
  transition: all 0.3s; 
  position: relative;
}
.remark-item-card.drag-over { 
  background: rgba(var(--primary-rgb), 0.1); 
  border-color: var(--primary-color); 
  box-shadow: 0 0 20px rgba(var(--primary-rgb), 0.4); 
  transform: scale(1.02);
}

.remark-card-header { display: flex; justify-content: space-between; align-items: start; margin-bottom: 0.8rem; }
.remark-title { font-weight: 800; font-size: 1.05rem; color: #fff; cursor: pointer; }
.remark-title:hover { color: var(--primary-color); }

.remark-actions { display: flex; gap: 4px; }
.act-btn { background: rgba(255,255,255,0.05); border: none; border-radius: 6px; width: 28px; height: 28px; font-size: 0.85rem; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: all 0.2s; }
.act-btn:hover { background: rgba(255,255,255,0.15); transform: translateY(-2px); }
.act-btn.del:hover { background: #e74c3c; color: white; }

/* Sidebar MD Box */
.remark-card-body { background: rgba(0,0,0,0.2); border-radius: 12px; padding: 1rem; border: 1px solid rgba(255,255,255,0.03); }
.body-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.body-header .label { font-size: 0.65rem; opacity: 0.5; font-weight: 800; }

.mini-mode-switch { display: flex; gap: 2px; background: rgba(255,255,255,0.05); padding: 2px; border-radius: 6px; }
.mini-mode-switch button { background: none; border: none; font-size: 0.6rem; color: #fff; padding: 2px 6px; border-radius: 4px; cursor: pointer; opacity: 0.5; }
.mini-mode-switch button.active { background: var(--primary-color); opacity: 1; }

.sidebar-md-box { font-size: 0.9rem; color: #ccc; line-height: 1.6; max-height: 150px; overflow: hidden; }
.sidebar-md-box :deep(h1), .sidebar-md-box :deep(h2) { font-size: 1rem; margin: 0.5rem 0; }
.sidebar-txt-box { font-size: 0.9rem; opacity: 0.7; color: #eee; }

.items-count { margin-top: 10px; font-size: 0.7rem; font-weight: 800; color: var(--primary-color); cursor: pointer; opacity: 0.7; }
.items-count:hover { opacity: 1; text-decoration: underline; }

/* Unified Editor Styles (Same as Desk) */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); backdrop-filter: blur(12px); display: flex; align-items: center; justify-content: center; z-index: 3001; }
.modal-card.wide-editor { width: 950px; max-width: 95vw; background: var(--card-bg); border-radius: 28px; border: 1px solid rgba(var(--primary-rgb), 0.3); display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 25px 60px rgba(0,0,0,0.6); }
.modal-card.is-full { width: 100vw; height: 100vh; border-radius: 0; }

.unified-controls { display: flex; gap: 0.8rem; align-items: center; }
.mode-capsule { display: flex; background: rgba(0,0,0,0.4); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.mode-capsule button { background: none; border: none; color: #fff; padding: 6px 14px; border-radius: 9px; font-size: 0.75rem; font-weight: 800; cursor: pointer; opacity: 0.4; }
.mode-capsule button.active { background: var(--primary-color); opacity: 1; }
.action-set { display: flex; background: rgba(255,255,255,0.05); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.action-item { background: none; border: none; color: #fff; width: 34px; height: 34px; border-radius: 9px; font-size: 1rem; cursor: pointer; opacity: 0.6; display: flex; align-items: center; justify-content: center; }
.action-item:hover { background: rgba(255,255,255,0.1); opacity: 1; }
.action-item.close:hover { background: #e74c3c; }

.editor-body { flex: 1; overflow-y: auto; padding: 2.5rem; display: flex; flex-direction: column; gap: 1.8rem; }
.form-group.fill { flex: 1; }
input, textarea { background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 14px; padding: 1.2rem; color: #fff; width: 100%; outline: none; }
textarea { height: 350px; resize: none; font-size: 1rem; }

.md-preview-box { background: rgba(0,0,0,0.4); padding: 2rem; border-radius: 14px; border: 1px solid rgba(255,255,255,0.05); min-height: 350px; color: #eee; line-height: 1.7; }
.md-preview-box :deep(h1) { color: var(--primary-color); border-bottom: 1px solid rgba(255,255,255,0.1); margin: 1.5rem 0 1rem; }

/* UNIFIED QUOTED ITEMS GRID (Shared by Chat/Desk) */
.quoted-items-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.2rem; margin-top: 1.2rem; padding-bottom: 2rem; }
.quoted-item-card { background: rgba(255,255,255,0.03); border-radius: 16px; border: 1px solid rgba(255,255,255,0.06); overflow: hidden; display: flex; flex-direction: column; transition: all 0.2s; }
.quoted-item-card:hover { transform: scale(1.02); border-color: var(--primary-color); background: rgba(var(--primary-rgb), 0.05); }
.item-meta-top { background: rgba(0,0,0,0.3); padding: 0.7rem 1rem; display: flex; justify-content: space-between; font-size: 0.65rem; font-weight: 800; align-items: center; }
.p-slug { opacity: 0.5; letter-spacing: 1px; text-transform: uppercase; }
.p-user { color: var(--primary-color); }
.item-img-box { height: 180px; cursor: zoom-in; overflow: hidden; background: #000; }
.item-img-box img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.4s; }
.quoted-item-card:hover .item-img-box img { transform: scale(1.1); }
.item-text-box { padding: 1.2rem; font-size: 0.95rem; color: #ddd; line-height: 1.6; max-height: 180px; overflow-y: auto; }

.modal-footer { padding: 1.5rem 2.5rem; display: flex; justify-content: flex-end; gap: 1.2rem; background: rgba(0,0,0,0.2); }
.save-btn { background: var(--primary-color); color: #fff; padding: 0.8rem 2.8rem; border-radius: 12px; font-weight: 800; cursor: pointer; border: none; }

.global-zoom { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.9); z-index: 4000; display: flex; align-items: center; justify-content: center; cursor: zoom-out; }
.global-zoom img { max-width: 90vw; max-height: 90vh; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
