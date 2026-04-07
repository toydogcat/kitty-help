<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import { apiService } from '../services/api';
import { marked } from 'marked';

const props = defineProps<{
  userRole: string;
  isAdmin: boolean;
  isToby: boolean;
}>();

// State
const shelves = ref<any[]>([]);
const desktopItems = ref<any[]>([]);
const activeShelfId = ref<string | null>(null);
const loading = ref(true);
const modalLoading = ref(false);
const draggingItem = ref<any>(null);
const dragOverShelfId = ref<string | null | 'desktop'>(null);

// Modals
const showAddShelfModal = ref(false);
const showRenameModal = ref(false);
const newShelfName = ref('');
const renamingShelfId = ref<string | null>(null);

// Item Editor Modal
const showEditModal = ref(false);
const isFullScreen = ref(false);
const editingItem = ref<any>(null);
const editBuffer = ref({ title: '', content: '' });
const editMode = ref<'edit' | 'preview'>('preview'); 
const saving = ref(false);
const remarkDetails = ref<any>(null); 

// Zoom Overlay for Remark items inside modal
const zoomedImageUrl = ref('');

const route = useRoute();

onMounted(() => {
  const shelf = route.query.shelfId as string;
  if (shelf) activeShelfId.value = shelf;
  fetchData();
});

const fetchData = async () => {
  loading.value = true;
  try {
    const [sData, iData] = await Promise.all([
      apiService.getShelves(),
      apiService.getDeskItems(activeShelfId.value || 'null')
    ]);
    shelves.value = sData;
    desktopItems.value = iData;
  } catch (err) {
    console.error("Failed to load desk data:", err);
  } finally {
    loading.value = false;
  }
};

const switchShelf = async (id: string | null) => {
  activeShelfId.value = id;
  await fetchData();
};

const handleAddShelf = async () => {
  if (!newShelfName.value) return;
  try {
    await apiService.createShelf({ name: newShelfName.value, sortOrder: shelves.value.length });
    newShelfName.value = '';
    showAddShelfModal.value = false;
    await fetchData();
  } catch (err) {
    alert("Failed to create shelf");
  }
};

const openRenameModal = (shelf: any) => {
  renamingShelfId.value = shelf.id;
  newShelfName.value = shelf.name;
  showRenameModal.value = true;
};

const handleRenameShelf = async () => {
  if (!renamingShelfId.value || !newShelfName.value) return;
  try {
    await apiService.updateShelf(renamingShelfId.value, { name: newShelfName.value });
    showRenameModal.value = false;
    await fetchData();
  } catch (err) {
    alert("Rename failed");
  }
};

const duplicateShelf = async (id: string) => {
  try {
    await apiService.duplicateShelf(id);
    await fetchData();
  } catch (err) {
    alert("Duplicate failed");
  }
};

const deleteShelf = async (id: string) => {
  if (!confirm("Delete this shelf? Items will be moved to desktop.")) return;
  try {
    await apiService.deleteShelf(id);
    if (activeShelfId.value === id) activeShelfId.value = null;
    await fetchData();
  } catch (err) {
    alert("Delete failed");
  }
};

const onDragStart = (item: any) => {
  draggingItem.value = item;
};

const onDragOverShelf = (id: string | null | 'desktop') => {
  dragOverShelfId.value = id;
};

const onDropOnShelf = async (shelfId: string | null) => {
  if (!draggingItem.value) return;
  try {
    await apiService.updateDeskItem(draggingItem.value.id, { shelfId });
    draggingItem.value = null;
    dragOverShelfId.value = null;
    await fetchData();
  } catch (err) {
    console.error("Move failed:", err);
  } finally {
    dragOverShelfId.value = null;
  }
};

const removeItem = async (id: string) => {
  try {
    await apiService.deleteDeskItem(id);
    await fetchData();
  } catch (err) {
    alert("Remove failed");
  }
};

const activeShelfName = computed(() => {
  if (!activeShelfId.value) return 'Main Desktop';
  const s = shelves.value.find(x => x.id === activeShelfId.value);
  return s ? s.name : 'Unknown Shelf';
});

const getIcon = (type: string) => {
  switch (type) {
    case 'bookmark': return '🔖';
    case 'snippet': return '📄';
    case 'media': return '🖼️';
    case 'remark': return '📚';
    default: return '📦';
  }
};

const getThumbnail = (item: any, large = false) => {
  if (item.type === 'media' && item.fileId) {
    const baseUrl = apiService.getStorehouseFileUrl(item.fileId, item.source);
    return `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}${large ? 'w=1024' : 'w=256'}`;
  }
  return null;
};

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  return apiService.getStorehouseFileUrl(mediaId, platform || 'line');
};

const openOriginal = async (item: any) => {
  if (item.type === 'bookmark' && item.url) {
    window.open(item.url, '_blank');
    return;
  }
  
  editingItem.value = item;
  editBuffer.value = { 
    title: item.title, 
    content: item.content || '' 
  };
  editMode.value = 'preview'; 
  showEditModal.value = true;
  isFullScreen.value = false;
  remarkDetails.value = null;

  if (item.type === 'remark') {
    modalLoading.value = true;
    try {
      const data = await apiService.getRemarks();
      const container = data.containers?.find((c: any) => c.id === item.refId);
      remarkDetails.value = container || null;
    } catch (err) {
      console.error("Failed to load remark details:", err);
    } finally {
      modalLoading.value = false;
    }
  }
};

const saveItemEdit = async () => {
  if (!editingItem.value) return;
  saving.value = true;
  try {
    if (editingItem.value.type === 'snippet') {
      await apiService.updateSnippet(editingItem.value.refId, {
        name: editBuffer.value.title,
        content: editBuffer.value.content
      });
    } else if (editingItem.value.type === 'media') {
      await apiService.updateStorehouseItem(editingItem.value.refId, {
        title: editBuffer.value.title,
        notes: editBuffer.value.content
      });
    } else if (editingItem.value.type === 'bookmark') {
      await apiService.updateBookmark(editingItem.value.refId, {
        title: editBuffer.value.title
      });
    } else if (editingItem.value.type === 'remark') {
      await apiService.updateRemark(editingItem.value.refId, {
        name: editBuffer.value.title,
        content: editBuffer.value.content,
        isPinned: remarkDetails.value?.isPinned || false
      });
    }
    
    showEditModal.value = false;
    await fetchData(); 
  } catch (err) {
    alert("Save failed");
  } finally {
    saving.value = false;
  }
};
</script>

<template>
  <div class="desk-view">
    <div class="desk-header">
      <div class="title-group">
        <h1>🖥️ Desk Explorer</h1>
        <p class="subtitle">Current Context: <strong>{{ activeShelfName }}</strong></p>
      </div>
      <div class="actions">
        <button v-if="activeShelfId" @click="switchShelf(null)" class="back-btn">⬅ Back to Desktop</button>
        <button @click="showAddShelfModal = true" class="add-shelf-btn">+ New Shelf</button>
      </div>
    </div>

    <div 
      class="desktop-canvas" 
      :class="{ 'drop-active': draggingItem && activeShelfId !== null }"
      @dragover.prevent
      @drop="onDropOnShelf(activeShelfId)"
    >
      <div v-if="loading" class="desk-loader">Loading workspace...</div>
      
      <div v-else-if="desktopItems.length === 0" class="empty-desk">
        <div class="empty-icon">📭</div>
        <h3>No items here</h3>
        <p>Drag items from other modules to pin them to your desk.</p>
      </div>

      <div v-else class="items-grid">
        <div 
          v-for="it in desktopItems" 
          :key="it.id"
          class="desk-tile"
          draggable="true"
          @dragstart="onDragStart(it)"
          @click="openOriginal(it)"
        >
          <div v-if="getThumbnail(it)" class="tile-preview">
            <img :src="getThumbnail(it) || ''" loading="lazy" />
          </div>
          <div v-else class="tile-icon">{{ getIcon(it.type) }}</div>
          
          <div class="tile-content">
            <span class="tile-title">{{ it.title }}</span>
            <span class="tile-meta">{{ it.type.toUpperCase() }}</span>
          </div>
          <button @click.stop="removeItem(it.id)" class="remove-btn" title="Unlink from desk">×</button>
        </div>
      </div>
    </div>

    <!-- Shelves Area -->
    <div class="shelves-rail shadow-lg">
      <div class="rail-header">
        <span class="rail-title">📚 My Shelves</span>
        <span class="rail-hint">Drag items below to store</span>
      </div>
      <div class="shelves-container custom-scrollbar">
        <div class="shelf-card desktop-link" :class="{ active: activeShelfId === null, 'drag-over': dragOverShelfId === 'desktop' }" @click="switchShelf(null)" @dragover.prevent="onDragOverShelf('desktop')" @dragleave="onDragOverShelf(null)" @drop="onDropOnShelf(null)">
          <span class="s-icon">{{ dragOverShelfId === 'desktop' ? '📥' : '🏠' }}</span>
          <span class="s-name">Desktop</span>
        </div>
        <div v-for="s in shelves" :key="s.id" class="shelf-card" :class="{ active: activeShelfId === s.id, 'drag-over': dragOverShelfId === s.id }" @click="switchShelf(s.id)" @dragover.prevent="onDragOverShelf(s.id)" @dragleave="onDragOverShelf(null)" @drop="onDropOnShelf(s.id)">
          <div class="shelf-top">
            <span class="s-icon">{{ dragOverShelfId === s.id ? '📥' : '📁' }}</span>
            <div class="s-actions">
              <button @click.stop="duplicateShelf(s.id)" class="s-dup">👯</button>
              <button @click.stop="openRenameModal(s)" class="s-edit">✎</button>
              <button @click.stop="deleteShelf(s.id)" class="s-del">×</button>
            </div>
          </div>
          <span class="s-name">{{ s.name }}</span>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <div v-if="showAddShelfModal || showRenameModal" class="modal-overlay" @click.self="showAddShelfModal = showRenameModal = false">
      <div class="modal-card mini">
        <h3>{{ showRenameModal ? 'Rename Shelf' : 'Create New Shelf' }}</h3>
        <input v-model="newShelfName" placeholder="Shelf Name..." @keyup.enter="showRenameModal ? handleRenameShelf() : handleAddShelf()" autoFocus />
        <div class="modal-actions">
           <button @click="showAddShelfModal = showRenameModal = false">Cancel</button>
           <button @click="showRenameModal ? handleRenameShelf() : handleAddShelf()" class="confirm-btn">Confirm</button>
        </div>
      </div>
    </div>

    <!-- Universal Item Editor Modal -->
    <Teleport to="body">
      <div v-if="showEditModal" class="modal-overlay editor-modal-overlay" @click.self="showEditModal = false">
        <div class="editor-pane shadow-2xl" :class="{ 'is-full': isFullScreen }">
          <div class="editor-header">
            <div class="type-badge">{{ editingItem?.type.toUpperCase() }} EDITOR</div>
            
            <!-- UNIFIED CONTROL CAPSULE -->
            <div class="unified-controls">
               <div class="mode-capsule" v-if="editingItem?.type === 'remark' || editingItem?.type === 'snippet'">
                  <button @click="editMode = 'preview'" :class="{ active: editMode === 'preview' }">MD PREVIEW</button>
                  <button @click="editMode = 'edit'" :class="{ active: editMode === 'edit' }">TXT / EDIT</button>
               </div>
               <div class="action-set">
                  <button @click="isFullScreen = !isFullScreen" class="action-item" title="Fullscreen">
                    {{ isFullScreen ? '❐' : '⛶' }}
                  </button>
                  <button @click="showEditModal = false" class="action-item close">✕</button>
               </div>
            </div>
          </div>

          <div class="editor-body custom-scrollbar">
            <div v-if="editingItem?.type === 'media'" class="media-large-preview">
              <img :src="getThumbnail(editingItem, true) || ''" />
            </div>

            <div class="field">
              <label>Title / Category Name</label>
              <input v-model="editBuffer.title" placeholder="e.g., Project Workspace" />
            </div>

            <div class="field fill">
              <label>Notes & Summary (Markdown Supported)</label>
              <div v-if="editMode === 'preview'" class="md-preview-area" v-html="marked.parse(editBuffer.content || '')"></div>
              <textarea v-else v-model="editBuffer.content" placeholder="Paste or type details here..."></textarea>
            </div>

            <!-- UNIFIED QUOTED ITEMS GRID -->
            <div v-if="editingItem?.type === 'remark'" class="nested-remark-items">
              <label class="section-label">📚 Quoted Items (引用項目)</label>
              <div v-if="modalLoading" class="modal-item-loader"><span class="spinner"></span> Loading items...</div>
              <div v-else class="quoted-items-grid">
                <div v-for="item in (remarkDetails?.items || [])" :key="item.id" class="quoted-item-card">
                  <div class="item-meta-top">
                    <span class="p-slug">{{ item.log?.platform }}</span>
                    <span class="p-user">{{ item.log?.senderName }}</span>
                  </div>
                  <div v-if="item.log?.mediaId && (item.log?.msgType === 'image' || item.log?.content?.includes('[Image]'))" class="item-img-box" @click="zoomedImageUrl = getStorehouseUrl(item.log.mediaId, item.log.platform)">
                    <img :src="getStorehouseUrl(item.log.mediaId, item.log.platform)" />
                  </div>
                  <div v-else class="item-text-box">
                    <p>{{ item.log?.content }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="editor-footer">
            <button @click="showEditModal = false" class="cancel-btn">Discard</button>
            <button @click="saveItemEdit" class="save-btn" :disabled="saving">
              {{ saving ? 'Saving...' : '✅ Save Changes' }}
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
.desk-view { height: calc(100vh - 120px); display: flex; flex-direction: column; gap: 1.5rem; padding: 1rem; position: relative; }
.desk-header { display: flex; justify-content: space-between; align-items: center; }
.title-group h1 { margin: 0; font-size: 1.8rem; color: var(--primary-color); }
.subtitle { margin: 0; opacity: 0.7; font-size: 0.9rem; }
.actions { display: flex; gap: 0.8rem; }
.add-shelf-btn, .back-btn { padding: 0.6rem 1.2rem; border-radius: 10px; font-weight: 700; cursor: pointer; }
.add-shelf-btn { background: var(--primary-color); color: white; border: none; }
.back-btn { background: rgba(var(--primary-rgb), 0.1); border: 1px solid var(--primary-color); color: var(--primary-color); }

.desktop-canvas { flex: 1; background: rgba(var(--primary-rgb), 0.03); border: 2px dashed rgba(var(--primary-rgb), 0.1); border-radius: 20px; overflow-y: auto; padding: 2rem; position: relative; }
.items-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1.5rem; }

.desk-tile { background: var(--card-bg); border: 1px solid rgba(var(--primary-rgb), 0.2); border-radius: 16px; padding: 1.2rem; display: flex; flex-direction: column; align-items: center; gap: 0.8rem; cursor: pointer; position: relative; transition: all 0.2s; backdrop-filter: blur(10px); }
.desk-tile:hover { transform: translateY(-5px); border-color: var(--primary-color); box-shadow: 0 8px 25px rgba(0,0,0,0.2); }
.tile-preview { width: 100%; height: 100px; border-radius: 12px; overflow: hidden; background: rgba(0,0,0,0.2); }
.tile-preview img { width: 100%; height: 100%; object-fit: cover; }
.tile-title { font-weight: 700; font-size: 0.9rem; text-align: center; width: 100%; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.remove-btn { position: absolute; top: -5px; right: -5px; background: #e74c3c; border: none; border-radius: 50%; width: 22px; height: 22px; color: white; font-size: 0.8rem; cursor: pointer; opacity: 0; transition: opacity 0.2s; z-index: 10; display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 5px rgba(0,0,0,0.3); }
.desk-tile:hover .remove-btn { opacity: 1; }

.shelves-rail { background: rgba(var(--primary-rgb), 0.05); backdrop-filter: blur(20px); border-radius: 20px; padding: 1.2rem; border: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; gap: 1rem; }
.shelves-container { display: flex; gap: 1rem; overflow-x: auto; padding-bottom: 0.5rem; }
.shelf-card { min-width: 130px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; padding: 1rem; display: flex; flex-direction: column; align-items: center; gap: 0.6rem; cursor: pointer; transition: all 0.3s; }
.shelf-card.active { background: var(--primary-color); }
.shelf-top { display: flex; justify-content: space-between; width: 100%; align-items: center; }
.s-actions { opacity: 0; display: flex; gap: 4px; transition: opacity 0.2s; }
.shelf-card:hover .s-actions { opacity: 1; }
.s-actions button { background: rgba(0,0,0,0.3); border: none; border-radius: 4px; padding: 2px 4px; color: #fff; font-size: 0.75rem; cursor: pointer; }
.s-name { font-weight: 700; font-size: 0.85rem; }

/* Unified Control Capsule Styling */
.unified-controls { display: flex; align-items: center; gap: 0.8rem; }
.mode-capsule { display: flex; background: rgba(0,0,0,0.4); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.mode-capsule button { background: none; border: none; color: #fff; padding: 6px 14px; border-radius: 9px; font-size: 0.75rem; font-weight: 800; cursor: pointer; opacity: 0.4; transition: all 0.2s; }
.mode-capsule button.active { background: var(--primary-color); opacity: 1; box-shadow: 0 2px 8px rgba(var(--primary-rgb), 0.4); }

.action-set { display: flex; background: rgba(255,255,255,0.05); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.action-item { background: none; border: none; color: #fff; width: 34px; height: 34px; border-radius: 9px; font-size: 1.1rem; cursor: pointer; opacity: 0.6; transition: all 0.2s; display: flex; align-items: center; justify-content: center; }
.action-item:hover { background: rgba(255,255,255,0.1); opacity: 1; }
.action-item.close:hover { background: #e74c3c; }

/* Editor Modal Adjustments */
.editor-modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); backdrop-filter: blur(12px); display: flex; align-items: center; justify-content: center; z-index: 3000; }
.editor-pane { background: var(--card-bg); width: 950px; max-width: 95vw; max-height: 92vh; border-radius: 28px; border: 1px solid rgba(var(--primary-rgb), 0.3); display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 25px 60px rgba(0,0,0,0.6); }
.editor-pane.is-full { width: 100vw; height: 100vh; max-height: 100vh; border-radius: 0; border: none; }

.editor-header { padding: 1.2rem 2.5rem; display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.05); }
.editor-body { flex: 1; overflow-y: auto; padding: 2.5rem; display: flex; flex-direction: column; gap: 2rem; }
.field { display: flex; flex-direction: column; gap: 0.6rem; }
.field input, .field textarea { background: rgba(0,0,0,0.25); border: 1px solid rgba(255,255,255,0.1); border-radius: 14px; padding: 1.2rem; color: #eee; width: 100%; outline: none; font-size: 1rem; }

.md-preview-area { background: rgba(0,0,0,0.35); padding: 2.5rem; border-radius: 14px; border: 1px solid rgba(255,255,255,0.05); min-height: 350px; color: #eee; line-height: 1.8; font-size: 1.1rem; }
.md-preview-area :deep(h1) { color: var(--primary-color); border-bottom: 1px solid rgba(255,255,255,0.1); padding-bottom: 8px; margin: 1.5rem 0 1rem; }

.editor-footer { padding: 1.5rem 2.5rem; display: flex; justify-content: flex-end; gap: 1.2rem; background: rgba(0,0,0,0.2); border-top: 1px solid rgba(255,255,255,0.05); }
.save-btn { background: var(--primary-color); color: #fff; padding: 0.8rem 2.8rem; border-radius: 12px; font-weight: 800; cursor: pointer; transition: all 0.2s; }

.global-zoom { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.9); z-index: 4000; display: flex; align-items: center; justify-content: center; cursor: zoom-out; }
.global-zoom img { max-width: 90vw; max-height: 90vh; }

/* UNIFIED QUOTED ITEMS GRID (Shared by Chat/Desk) */
.nested-remark-items { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid rgba(255,255,255,0.05); }
.section-label { font-size: 0.75rem; font-weight: 800; color: var(--primary-color); opacity: 0.6; letter-spacing: 1.5px; text-transform: uppercase; display: block; margin-bottom: 1rem; }

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

.custom-scrollbar::-webkit-scrollbar { width: 8px; height: 8px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
