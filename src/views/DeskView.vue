<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import { apiService } from '../services/api';
import UnifiedRemarkModal from '../components/UnifiedRemarkModal.vue';
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

const saveItemEdit = async (updatedData: { title: string, content: string }) => {
  if (!editingItem.value) return;
  saving.value = true;
  try {
    if (editingItem.value.type === 'snippet') {
      await apiService.updateSnippet(editingItem.value.refId, {
        name: updatedData.title,
        content: updatedData.content
      });
    } else if (editingItem.value.type === 'media') {
      await apiService.updateStorehouseItem(editingItem.value.refId, {
        title: updatedData.title,
        notes: updatedData.content
      });
    } else if (editingItem.value.type === 'bookmark') {
      await apiService.updateBookmark(editingItem.value.refId, {
        title: updatedData.title
      });
    } else if (editingItem.value.type === 'remark') {
      await apiService.updateRemark(editingItem.value.refId, {
        name: updatedData.title,
        content: updatedData.content,
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

    <!-- UNIFIED ITEM EDITOR MODAL -->
    <UnifiedRemarkModal 
      :show="showEditModal"
      :item="editingItem"
      :details="remarkDetails"
      :loading="modalLoading"
      @close="showEditModal = false"
      @save="saveItemEdit"
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
.desk-view { height: calc(100vh - 120px); display: flex; flex-direction: column; gap: 1.5rem; padding: 1rem; position: relative; }
.desk-header { display: flex; justify-content: space-between; align-items: center; }
.title-group h1 { margin: 0; font-size: 1.8rem; color: var(--primary-color); }
.subtitle { margin: 0; opacity: 0.7; font-size: 0.9rem; }
.actions { display: flex; gap: 0.8rem; }
.add-shelf-btn, .back-btn { padding: 0.6rem 1.2rem; border-radius: 10px; font-weight: 700; cursor: pointer; }
.add-shelf-btn { background: var(--primary-color); color: white; border: none; }
.back-btn { background: rgba(var(--primary-rgb), 0.1); border: 1px solid var(--primary-color); color: var(--primary-color); }

/* DESK SPECIFIC STYLES */
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

.custom-scrollbar::-webkit-scrollbar { width: 8px; height: 8px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
