<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiService } from '../services/api';
import UnifiedRemarkModal from '../components/UnifiedRemarkModal.vue';
import { usePin } from '../composables/usePin';

const route = useRoute();
const router = useRouter();

const { unpinFromDesk } = usePin();

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

const activeShelfName = computed(() => {
  if (!activeShelfId.value) return 'Main Desktop';
  const s = shelves.value.find(s => s.id === activeShelfId.value);
  return s ? s.name : 'Unknown Shelf';
});

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
    await unpinFromDesk(id);
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

// Helper removed because it is now handled inside UnifiedRemarkModal component

const openOriginal = async (item: any) => {
  if (item.type === 'bookmark' && item.url) {
    if (item.url.startsWith('/') && !item.url.startsWith('//')) {
      router.push(item.url);
    } else {
      window.open(item.url, '_blank');
    }
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
        <p class="subtitle">Current Context: <strong>{{ activeShelfName }}</strong> <span v-if="desktopItems.length" class="count-tag">({{ desktopItems.length }} items)</span></p>
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
            <div class="tile-badges">
              <span class="badge" :class="it.type">{{ it.type.toUpperCase() }}</span>
            </div>
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
.desk-header { display: flex; justify-content: space-between; align-items: center; background: rgba(var(--primary-rgb), 0.05); padding: 1.2rem; border-radius: 16px; border: 1px solid rgba(var(--primary-rgb), 0.1); }
.title-group h1 { margin: 0; font-size: 1.6rem; color: var(--primary-color); text-shadow: 0 0 15px rgba(var(--primary-rgb), 0.3); }
.subtitle { margin: 4px 0 0 0; opacity: 0.8; font-size: 0.85rem; display: flex; align-items: center; gap: 8px; }
.count-tag { background: rgba(var(--primary-rgb), 0.2); color: var(--primary-color); padding: 2px 8px; border-radius: 20px; font-weight: 700; font-size: 0.75rem; border: 1px solid rgba(var(--primary-rgb), 0.3); }

.actions { display: flex; gap: 0.8rem; }
.add-shelf-btn, .back-btn { padding: 0.6rem 1.2rem; border-radius: 10px; font-weight: 700; cursor: pointer; transition: all 0.2s; }
.add-shelf-btn { background: var(--primary-color); color: white; border: none; box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.4); }
.add-shelf-btn:hover { transform: translateY(-2px); box-shadow: 0 6px 20px rgba(var(--primary-rgb), 0.5); }
.back-btn { background: rgba(var(--primary-rgb), 0.1); border: 1px solid var(--primary-color); color: var(--primary-color); }

.desktop-canvas { flex: 1; background: rgba(0, 0, 0, 0.2); border: 2px solid rgba(var(--primary-rgb), 0.1); border-radius: 24px; overflow-y: auto; padding: 2rem; position: relative; box-shadow: inset 0 0 40px rgba(0,0,0,0.3); }
.items-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 1.5rem; }

.desk-tile { background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 20px; padding: 1rem; display: flex; flex-direction: column; gap: 0.8rem; cursor: pointer; position: relative; transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1); backdrop-filter: blur(12px); }
.desk-tile:hover { transform: translateY(-8px); border-color: var(--primary-color); background: rgba(var(--primary-rgb), 0.05); box-shadow: 0 15px 35px rgba(0,0,0,0.4); }

.tile-preview { width: 100%; height: 110px; border-radius: 14px; overflow: hidden; background: #000; border: 1px solid rgba(255,255,255,0.1); }
.tile-preview img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.5s; }
.desk-tile:hover .tile-preview img { transform: scale(1.1); }

.tile-icon { width: 100%; height: 110px; display: flex; align-items: center; justify-content: center; font-size: 3rem; background: rgba(255,255,255,0.02); border-radius: 14px; border: 1px dashed rgba(255,255,255,0.1); }

.tile-content { flex: 1; padding: 4px; display: flex; flex-direction: column; gap: 4px; }
.tile-title { font-weight: 700; font-size: 0.95rem; color: #fff; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.tile-badges { display: flex; gap: 6px; }
.badge { font-size: 0.65rem; font-weight: 800; padding: 2px 8px; border-radius: 6px; text-transform: uppercase; letter-spacing: 0.5px; box-shadow: 0 2px 5px rgba(0,0,0,0.2); }
.badge.bookmark { background: #4a90e2; color: #fff; }
.badge.remark { background: #9013fe; color: #fff; }
.badge.media { background: #2ecc71; color: #fff; }
.badge.snippet { background: #f1c40f; color: #000; }

.remove-btn { position: absolute; top: 12px; right: 12px; background: rgba(0,0,0,0.6); border: 1px solid rgba(255,255,255,0.2); color: #fff; border-radius: 50%; width: 24px; height: 24px; opacity: 0; transition: all 0.2s; display: flex; align-items: center; justify-content: center; z-index: 5; }
.desk-tile:hover .remove-btn { opacity: 1; }
.remove-btn:hover { background: #e74c3c; border-color: #e74c3c; transform: scale(1.1); }

.shelves-rail { background: rgba(13, 17, 23, 0.9); border-radius: 24px; padding: 1.5rem; border: 1px solid rgba(var(--primary-rgb), 0.2); display: flex; flex-direction: column; gap: 1rem; box-shadow: 0 -10px 40px rgba(0,0,0,0.5); }
.rail-header { display: flex; align-items: center; justify-content: space-between; padding: 0 8px; }
.rail-title { color: var(--primary-color); font-weight: 800; font-size: 1.2rem; letter-spacing: 1px; }
.rail-hint { font-size: 0.8rem; opacity: 0.5; font-style: italic; }

.shelves-container { display: flex; gap: 1.2rem; overflow-x: auto; padding: 0.5rem; }
.shelf-card { 
  min-width: 160px; 
  height: 110px; 
  background: rgba(255, 255, 255, 0.03); 
  border: 1px solid rgba(255, 255, 255, 0.1); 
  border-radius: 20px; 
  padding: 1.2rem; 
  display: flex; 
  flex-direction: column; 
  align-items: center; 
  justify-content: center; 
  gap: 8px; 
  cursor: pointer; 
  transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  position: relative;
  overflow: hidden;
}

.shelf-card.active { border-color: var(--primary-color); background: rgba(var(--primary-rgb), 0.15); box-shadow: 0 0 20px rgba(var(--primary-rgb), 0.2); }
.shelf-card:hover { transform: translateY(-5px); background: rgba(255,255,255,0.06); }

.shelf-card.drag-over { 
  transform: scale(1.1); 
  background: rgba(var(--primary-rgb), 0.25) !important; 
  border: 2px dashed var(--primary-color); 
  animation: pulse 1s infinite;
}

.shelf-card.drag-over::before {
  content: '📥';
  font-size: 1.5rem;
  margin-bottom: 4px;
}

.shelf-card.drag-over .s-name {
  color: var(--primary-color);
  font-weight: 900;
}

@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0.4); }
  70% { box-shadow: 0 0 0 15px rgba(var(--primary-rgb), 0); }
  100% { box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0); }
}

.s-icon { font-size: 1.8rem; transition: transform 0.3s; }
.shelf-card:hover .s-icon { transform: scale(1.2); }
.s-name { font-weight: 700; font-size: 0.95rem; opacity: 0.9; }

.s-actions { position: absolute; top: 10px; right: 10px; display: flex; gap: 4px; opacity: 0; transition: opacity 0.2s; }
.shelf-card:hover .s-actions { opacity: 1; }
.s-actions button { background: rgba(0,0,0,0.5); border: none; padding: 4px; border-radius: 6px; cursor: pointer; color: #fff; font-size: 0.7rem; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; height: 6px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(var(--primary-rgb), 0.2); border-radius: 10px; }
.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: var(--primary-color); }
</style>
