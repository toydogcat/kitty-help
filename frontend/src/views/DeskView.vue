<script setup lang="ts">
import { ref, onMounted, computed, onUnmounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { liveQuery } from 'dexie';
import { db } from '../services/localDb';
import { syncService } from '../services/syncService';
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
const draggingShelf = ref<any>(null); // Track shelf reordering
const dragOverShelfId = ref<string | null | 'desktop'>(null);
const draggingShelfOverId = ref<string | null>(null); // For visual feedback on shelf reordering

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

// --- 🔐 2FA Security Logic ---
const is2FAVerified = ref(false);
const show2FAModal = ref(false);
const totpCode = ref('');
const totpError = ref('');
const pendingAction = ref<(() => void) | null>(null);

const check2FA = async () => {
  try {
    const status = await apiService.getTOTPStatus();
    is2FAVerified.value = status.enabled && status.verified;
    return is2FAVerified.value;
  } catch {
    return false;
  }
};

const handleSensitiveAction = async (action: () => void) => {
  const verified = await check2FA();
  if (verified) {
    action();
  } else {
    show2FAModal.value = true;
    pendingAction.value = action;
  }
};

const verifyTOTP = async () => {
  totpError.value = '';
  try {
    await apiService.verifyTOTP(totpCode.value);
    totpCode.value = '';
    show2FAModal.value = false;
    is2FAVerified.value = true;
    if (pendingAction.value) {
      pendingAction.value();
      pendingAction.value = null;
    }
  } catch (err: any) {
    totpError.value = err.response?.data?.error || 'Verification failed';
  }
};



let shelvesSub: any = null;
let itemsSub: any = null;

const fetchData = async () => {
  loading.value = true;
  try {
    syncService.refreshShelves().catch(() => {});
    syncService.refreshDeskItems(activeShelfId.value || 'null').catch(() => {});
  } catch (err) {
    console.error("Desk background sync error");
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  const shelf = route.query.shelfId as string;
  if (shelf) activeShelfId.value = shelf;

  shelvesSub = liveQuery(() => db.shelves.orderBy('sortOrder').toArray()).subscribe(val => {
     shelves.value = val;
  });

  itemsSub = liveQuery(() => 
     db.deskItems.where('shelfId').equals(activeShelfId.value || 'null').sortBy('sortOrder')
  ).subscribe(val => {
     desktopItems.value = val;
     loading.value = false;
  });

  fetchData();
});

onUnmounted(() => {
  if (shelvesSub) shelvesSub.unsubscribe();
  if (itemsSub) itemsSub.unsubscribe();
});

watch(activeShelfId, (newId) => {
  if (itemsSub) itemsSub.unsubscribe();
  itemsSub = liveQuery(() => 
     db.deskItems.where('shelfId').equals(newId || 'null').sortBy('sortOrder')
  ).subscribe(val => {
     desktopItems.value = val;
  });
  syncService.refreshDeskItems(newId || 'null').catch(() => {});
});

const switchShelf = async (id: string | null) => {
  activeShelfId.value = id;
};

const handleAddShelf = async () => {
  if (!newShelfName.value) return;
  try {
    await syncService.createShelf({ name: newShelfName.value, sortOrder: shelves.value.length });
    newShelfName.value = '';
    showAddShelfModal.value = false;
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
    await syncService.updateShelf(renamingShelfId.value, { name: newShelfName.value });
    showRenameModal.value = false;
  } catch (err) {
    alert("Rename failed");
  }
};

const duplicateShelf = async (id: string) => {
  try {
    const s = shelves.value.find(x => x.id === id);
    if (!s) return;
    await syncService.createShelf({ name: `${s.name} (Copy)`, sortOrder: shelves.value.length });
  } catch (err) {
    alert("Duplicate failed");
  }
};

const deleteShelf = async (id: string) => {
  if (!confirm("Delete this shelf? Items will be moved to desktop.")) return;
  try {
    await syncService.deleteShelf(id);
    if (activeShelfId.value === id) activeShelfId.value = null;
  } catch (err) {
    alert("Delete failed");
  }
};

const onDragStart = (item: any) => {
  draggingItem.value = item;
  draggingShelf.value = null;
};

const onShelfDragStart = (shelf: any) => {
  draggingShelf.value = shelf;
  draggingItem.value = null;
};

const onDragOverShelf = (id: string | null | 'desktop') => {
  if (draggingShelf.value) {
    if (id !== 'desktop') draggingShelfOverId.value = id;
    return;
  }
  dragOverShelfId.value = id;
};

const onShelfDrop = async (targetId: string | null) => {
  if (!draggingShelf.value || draggingShelf.value.id === targetId) return;
  const oldIndex = shelves.value.findIndex(s => s.id === draggingShelf.value.id);
  const newIndex = shelves.value.findIndex(s => s.id === targetId);
  if (oldIndex === -1 || newIndex === -1) return;
  
  const newShelves = [...shelves.value];
  const [removed] = newShelves.splice(oldIndex, 1);
  newShelves.splice(newIndex, 0, removed);
  
  // EverSync sequential updates (temporary)
  for (let i = 0; i < newShelves.length; i++) {
    await syncService.updateShelf(newShelves[i].id, { sortOrder: i });
  }
  
  draggingShelf.value = null;
  draggingShelfOverId.value = null;
};

const onDropOnShelf = async (shelfId: string | null) => {
  if (draggingShelf.value) {
    onShelfDrop(shelfId);
    return;
  }
  if (!draggingItem.value) return;
  try {
    await syncService.updateDeskItem(draggingItem.value.id, { shelfId });
    draggingItem.value = null;
    dragOverShelfId.value = null;
  } catch (err) {
    console.error("Move failed:", err);
  } finally {
    dragOverShelfId.value = null;
  }
};

const removeItem = async (id: string, type: string = '') => {
  const performDelete = async () => {
    try {
      await unpinFromDesk(id);
    } catch (err) {
      alert("Remove failed");
    }
  };
  if (type === 'password') {
    handleSensitiveAction(performDelete);
  } else {
    performDelete();
  }
};

const getIcon = (type: string) => {
  switch (type) {
    case 'bookmark': return '🔖';
    case 'snippet': return '📄';
    case 'media': return '🖼️';
    case 'remark': return '💬';
    case 'password': return '🔑';
    case 'book': return '📚';
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

const openOriginal = async (item: any) => {
  const performOpen = async () => {
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
      content: item.notes || item.content || '' 
    };
    editMode.value = 'preview'; 
    showEditModal.value = true;
    isFullScreen.value = false;
    remarkDetails.value = null;

    if (item.type === 'remark') {
      modalLoading.value = true;
      try {
        const container = await db.remarks.get(item.refId);
        if (container) {
            const localItems = await db.remarkItems.where('containerId').equals(item.refId).toArray();
            const allItems = [...(container.items || [])];
            localItems.forEach(li => {
                if (!allItems.find(ai => ai.id === li.id)) {
                    allItems.push(li);
                }
            });
            remarkDetails.value = { ...container, items: allItems };
        } else {
            // Fallback sync if not found
            const data = await syncService.refreshRemarks();
            const c = data.containers?.find((x: any) => x.id === item.refId);
            if (c) {
                const items = data.items?.filter((x: any) => x.containerId === item.refId) || [];
                remarkDetails.value = { ...c, items };
            }
        }
      } catch (err) {
        console.error("Failed to load remark details:", err);
      } finally {
        modalLoading.value = false;
      }
    } else if (item.type === 'book') {
      modalLoading.value = true;
      try {
        const b = await db.bookcase.get(item.refId);
        if (b) {
          editBuffer.value.content = b.notes || '';
          editBuffer.value.title = b.title;
        } else {
           const remoteBooks = await syncService.refreshBookcase();
           const rb = remoteBooks.find((x: any) => x.id === item.refId);
           if (rb) {
              editBuffer.value.content = rb.notes || '';
              editBuffer.value.title = rb.title;
           }
        }
      } catch (err) {
        console.error("Failed to load book details:", err);
      } finally {
        modalLoading.value = false;
      }
    }
  };

  if (item.type === 'password') {
    handleSensitiveAction(performOpen);
  } else {
    performOpen();
  }
};

const saveItemEdit = async (updatedData: { title: string, content: string }) => {
  if (!editingItem.value) return;
  saving.value = true;
  try {
    if (editingItem.value.type === 'snippet') {
      await syncService.updateSnippet(editingItem.value.refId, {
        name: updatedData.title,
        content: updatedData.content
      });
    } else if (editingItem.value.type === 'media') {
      // Media doesn't have EverSync yet
      await apiService.updateStorehouseItem(editingItem.value.refId, {
        title: updatedData.title,
        notes: updatedData.content
      });
    } else if (editingItem.value.type === 'bookmark') {
      await syncService.updateBookmark(editingItem.value.refId, {
        title: updatedData.title
      });
    } else if (editingItem.value.type === 'remark') {
      await syncService.updateRemark(editingItem.value.refId, {
        name: updatedData.title,
        content: updatedData.content
      });
    } else if (editingItem.value.type === 'book') {
      await syncService.updateBookNote(editingItem.value.refId, { content: updatedData.content });
    }
    showEditModal.value = false;
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

    <!-- Shelves Area (Moved to Top) -->
    <div class="shelves-rail shadow-lg">
      <div class="rail-header">
        <span class="rail-title">📚 My Shelves</span>
        <span class="rail-hint">Drag folders to reorder • Drag items onto them to move</span>
      </div>
      <div class="shelves-container custom-scrollbar">
        <div 
          class="shelf-card desktop-link" 
          :class="{ active: activeShelfId === null, 'drag-over': dragOverShelfId === 'desktop' }" 
          @click="switchShelf(null)" 
          @dragover.prevent="onDragOverShelf('desktop')" 
          @dragleave="onDragOverShelf(null)" 
          @drop="onDropOnShelf(null)"
        >
          <span class="s-icon">{{ dragOverShelfId === 'desktop' ? '📥' : '🏠' }}</span>
          <span class="s-name">Desktop</span>
        </div>
        
        <div 
          v-for="s in shelves" 
          :key="s.id" 
          class="shelf-card" 
          :class="{ 
            active: activeShelfId === s.id, 
            'drag-over': dragOverShelfId === s.id,
            'shelf-dragging': draggingShelf?.id === s.id,
            'shelf-reorder-target': draggingShelfOverId === s.id && draggingShelf?.id !== s.id
          }" 
          draggable="true"
          @dragstart="onShelfDragStart(s)"
          @click="switchShelf(s.id)" 
          @dragover.prevent="onDragOverShelf(s.id)" 
          @dragleave="draggingShelfOverId = null; dragOverShelfId = null" 
          @drop="onDropOnShelf(s.id)"
        >
          <div class="shelf-top">
            <span class="s-icon">
              <template v-if="draggingShelf">📁</template>
              <template v-else>{{ dragOverShelfId === s.id ? '📥' : '📁' }}</template>
            </span>
            <div class="s-actions" v-if="!draggingShelf">
              <button @click.stop="duplicateShelf(s.id)" class="s-dup" title="Duplicate">👯</button>
              <button @click.stop="openRenameModal(s)" class="s-edit" title="Rename">✎</button>
              <button @click.stop="deleteShelf(s.id)" class="s-del" title="Delete">×</button>
            </div>
          </div>
          <span class="s-name">{{ s.name || 'Unnamed' }}</span>
        </div>
      </div>
    </div>

    <!-- Main Desktop Workspace -->
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
          <button @click.stop="removeItem(it.id, it.type)" class="remove-btn" title="Unlink from desk">×</button>
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

    <!-- 🔐 GLOBAL 2FA VERIFY MODAL FOR DESK SENSITIVE ACTIONS -->
    <div v-if="show2FAModal" class="modal-overlay" @click.self="show2FAModal = false">
      <div class="modal-content card glow auth-verify">
        <div class="modal-header center">
          <div class="icon-circle">🔑</div>
          <h3>Security Verification</h3>
          <p>This action requires a 2FA challenge.</p>
        </div>
        
        <div class="form-group center">
          <label>Google Authenticator Code</label>
          <input v-model="totpCode" class="otp-input" placeholder="000 000" maxlength="6" autofocus @keyup.enter="verifyTOTP" />
          <p v-if="totpError" class="error-msg">{{ totpError }}</p>
        </div>

        <div class="modal-actions full">
          <button class="btn-confirm big" @click="verifyTOTP">Confirm Action</button>
          <button class="btn-cancel" @click="show2FAModal = false">Cancel</button>
        </div>
      </div>
    </div>
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
.badge.password { background: #f1c40f; color: #000; }
.badge.snippet { background: #f1c40f; color: #000; }

.remove-btn { position: absolute; top: 12px; right: 12px; background: rgba(0,0,0,0.6); border: 1px solid rgba(255,255,255,0.2); color: #fff; border-radius: 50%; width: 24px; height: 24px; opacity: 0; transition: all 0.2s; display: flex; align-items: center; justify-content: center; z-index: 5; }
.desk-tile:hover .remove-btn { opacity: 1; }
.remove-btn:hover { background: #e74c3c; border-color: #e74c3c; transform: scale(1.1); }

.shelves-rail { 
  background: rgba(13, 17, 23, 0.85); 
  border-radius: 20px; 
  padding: 1rem 1.5rem; 
  border: 1px solid rgba(var(--primary-rgb), 0.2); 
  display: flex; 
  flex-direction: column; 
  gap: 0.8rem; 
  box-shadow: 0 10px 40px rgba(0,0,0,0.5); 
  backdrop-filter: blur(20px);
  position: sticky;
  top: 0px;
  z-index: 100;
  margin-top: -0.5rem;
}
.rail-header { display: flex; align-items: center; justify-content: space-between; padding: 0 8px; border-bottom: 1px solid rgba(255,255,255,0.05); padding-bottom: 8px; }
.rail-title { color: var(--primary-color); font-weight: 800; font-size: 1.1rem; letter-spacing: 0.5px; }
.rail-hint { font-size: 0.75rem; opacity: 0.4; font-style: italic; }

.shelves-container { display: flex; gap: 1rem; overflow-x: auto; padding-bottom: 4px; }
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
  justify-content: flex-start; 
  padding: 1.2rem 1rem;
  gap: 4px; 
  cursor: pointer; 
  transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  position: relative;
  overflow: hidden;
  text-align: center;
}

.shelf-top {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
  flex: 1;
}

.shelf-card.active { border-color: var(--primary-color); background: rgba(var(--primary-rgb), 0.15); box-shadow: 0 0 20px rgba(var(--primary-rgb), 0.2); }
.shelf-card:hover { transform: translateY(-5px); background: rgba(255,255,255,0.06); }

.shelf-card.drag-over { 
  transform: scale(1.05); 
  background: rgba(var(--primary-rgb), 0.2) !important; 
  border: 1px solid var(--primary-color); 
}

.shelf-dragging {
  opacity: 0.4;
  border: 2px dashed rgba(var(--primary-rgb), 0.5) !important;
}

.shelf-reorder-target {
  background: rgba(var(--primary-rgb), 0.1) !important;
  border-left: 6px solid var(--primary-color) !important;
  padding-left: 1.5rem !important;
}

@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0.4); }
  70% { box-shadow: 0 0 0 15px rgba(var(--primary-rgb), 0); }
  100% { box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0); }
}

.s-icon { 
  font-size: 1.8rem; 
  transition: transform 0.3s; 
  line-height: 1;
  display: block;
}
.shelf-card:hover .s-icon { transform: scale(1.1); }

.s-name { 
  font-weight: 700; 
  font-size: 0.85rem; 
  color: #fff !important;
  opacity: 1; 
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
  margin-top: 4px;
  display: block;
}

.s-actions { position: absolute; top: 8px; right: 8px; display: flex; gap: 4px; opacity: 0; transition: opacity 0.2s; z-index: 10; }
.shelf-card:hover .s-actions { opacity: 1; }
.s-actions button { background: rgba(0,0,0,0.6); border: 1px solid rgba(255,255,255,0.1); padding: 4px; border-radius: 6px; cursor: pointer; color: #fff; font-size: 0.7rem; display: flex; align-items: center; justify-content: center; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; height: 6px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(var(--primary-rgb), 0.2); border-radius: 10px; }
.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: var(--primary-color); }

/* 🔐 2FA Modal Styles (Synced with PasswordVault) */
.modal-overlay { position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.85); backdrop-filter: blur(10px); display: flex; align-items: center; justify-content: center; z-index: 2500; }
.modal-content { width: 90%; max-width: 450px; padding: 2.5rem; background: #1e1e24; border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 24px; text-align: center; }
.center { text-align: center; }
.icon-circle { font-size: 2.5rem; background: rgba(var(--primary-rgb), 0.1); width: 70px; height: 70px; display: flex; align-items: center; justify-content: center; border-radius: 50%; margin: 0 auto 1.5rem; }
.otp-input { font-size: 2.2rem !important; text-align: center !important; letter-spacing: 1rem; padding: 1.2rem !important; font-weight: 800; color: var(--primary-color) !important; background: rgba(0,0,0,0.3) !important; border: 2px solid var(--border-color) !important; width: 100%; border-radius: 12px; margin-top: 1rem; }
.modal-actions.full { display: flex; flex-direction: column; gap: 1rem; margin-top: 2rem; }
.btn-confirm.big { background: var(--primary-color); color: white; border: none; padding: 1.2rem; border-radius: 12px; font-size: 1.1rem; font-weight: 800; cursor: pointer; transition: all 0.3s; }
.btn-confirm.big:hover { filter: brightness(1.2); transform: translateY(-2px); box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.4); }
.btn-cancel { background: transparent; color: white; border: 1px solid rgba(255,255,255,0.1); padding: 0.8rem; border-radius: 10px; cursor: pointer; }
.error-msg { color: #f87171; font-size: 0.85rem; margin-top: 0.8rem; font-weight: bold; }
</style>
