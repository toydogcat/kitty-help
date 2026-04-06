<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
  userRole: string;
  isAdmin: boolean;
  isToby: boolean;
}>();

// State
const shelves = ref<any[]>([]);
const desktopItems = ref<any[]>([]);
const activeShelfId = ref<string | null>(null); // null means "Desktop"
const loading = ref(true);
const draggingItem = ref<any>(null);
const dragOverShelfId = ref<string | null | 'desktop'>(null);

// Modals
const showAddShelfModal = ref(false);
const newShelfName = ref('');

onMounted(() => {
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

// Drag and Drop Logic
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
    default: return '📦';
  }
};

const getThumbnail = (item: any) => {
  if (item.type === 'media' && item.fileId) {
    const baseUrl = apiService.getStorehouseFileUrl(item.fileId, item.source);
    return `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}w=256`;
  }
  return null;
};

const openOriginal = (item: any) => {
  // Logic to open original item based on type
  if (item.type === 'bookmark' && item.url) {
    window.open(item.url, '_blank');
  } else {
    // In a real app, you might emit an event or route to the specific editor
    alert(`Opening ${item.type}: ${item.title}. (Integration with editors coming soon!)`);
  }
};
</script>

<template>
  <div class="desk-view">
    <!-- Header -->
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

    <!-- Desktop Area -->
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
          @dblclick="openOriginal(it)"
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

    <!-- Shelves Area (Bottom Rail) -->
    <div class="shelves-rail shadow-lg">
      <div class="rail-header">
        <span class="rail-title">📚 My Shelves</span>
        <span class="rail-hint">Drag items below to store</span>
      </div>
      
      <div class="shelves-container">
        <!-- Main Desktop Entry -->
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

        <!-- Dynamic Shelves -->
        <div 
          v-for="s in shelves" 
          :key="s.id"
          class="shelf-card"
          :class="{ active: activeShelfId === s.id, 'drag-over': dragOverShelfId === s.id }"
          @click="switchShelf(s.id)"
          @dblclick="switchShelf(s.id)"
          @dragover.prevent="onDragOverShelf(s.id)"
          @dragleave="onDragOverShelf(null)"
          @drop="onDropOnShelf(s.id)"
        >
          <div class="shelf-top">
            <span class="s-icon">{{ dragOverShelfId === s.id ? '📥' : '📁' }}</span>
            <button @click.stop="deleteShelf(s.id)" class="s-del">×</button>
          </div>
          <span class="s-name">{{ s.name }}</span>
        </div>
      </div>
    </div>

    <!-- Modal for New Shelf -->
    <div v-if="showAddShelfModal" class="modal-overlay" @click.self="showAddShelfModal = false">
      <div class="modal-card mini">
        <h3>Create New Shelf</h3>
        <input v-model="newShelfName" placeholder="Shelf Name..." @keyup.enter="handleAddShelf" autoFocus />
        <div class="modal-actions">
          <button @click="showAddShelfModal = false">Cancel</button>
          <button @click="handleAddShelf" class="confirm-btn">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.desk-view {
  height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  padding: 1rem;
  position: relative;
}

.desk-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title-group h1 { margin: 0; font-size: 1.8rem; color: var(--primary-color); }
.subtitle { margin: 0; opacity: 0.7; font-size: 0.9rem; }

.actions { display: flex; gap: 0.8rem; }
.add-shelf-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 0.6rem 1.2rem;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
}

.back-btn {
  background: rgba(var(--primary-rgb), 0.1);
  border: 1px solid var(--primary-color);
  color: var(--primary-color);
  padding: 0.6rem 1.2rem;
  border-radius: 10px;
  font-weight: 700;
  cursor: pointer;
}

/* Desktop Canvas */
.desktop-canvas {
  flex: 1;
  background: rgba(var(--primary-rgb), 0.03);
  border: 2px dashed rgba(var(--primary-rgb), 0.1);
  border-radius: 20px;
  position: relative;
  overflow-y: auto;
  padding: 2rem;
  transition: all 0.3s;
}

.desktop-canvas.drop-active {
  background: rgba(var(--primary-rgb), 0.08);
  border-color: var(--primary-color);
}

.desk-loader, .empty-desk {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  opacity: 0.6;
}

.empty-icon { font-size: 4rem; margin-bottom: 1rem; }

/* Grid Tiles */
.items-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 1.5rem;
}

.desk-tile {
  background: var(--card-bg);
  border: 1px solid rgba(var(--primary-rgb), 0.2);
  border-radius: 16px;
  padding: 1.2rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.8rem;
  cursor: grab;
  position: relative;
  transition: all 0.2s;
  backdrop-filter: blur(10px);
}

.desk-tile:hover {
  transform: translateY(-5px);
  border-color: var(--primary-color);
  box-shadow: 0 8px 25px rgba(0,0,0,0.2);
}

.desk-tile:active { cursor: grabbing; }

.tile-icon { font-size: 2.5rem; }
.tile-preview {
  width: 100%;
  height: 100px;
  border-radius: 12px;
  overflow: hidden;
  background: rgba(0,0,0,0.2);
  display: flex;
  align-items: center;
  justify-content: center;
}
.tile-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease;
}
.desk-tile:hover .tile-preview img {
  transform: scale(1.1);
}

.tile-content { text-align: center; width: 100%; }
.tile-title {
  display: block;
  font-weight: 700;
  font-size: 0.95rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.tile-meta { font-size: 0.7rem; opacity: 0.5; font-weight: 800; letter-spacing: 1px; }

.remove-btn {
  position: absolute;
  top: 5px;
  right: 5px;
  background: none;
  border: none;
  color: #ff5f5f;
  font-size: 1.2rem;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.2s;
}
.desk-tile:hover .remove-btn { opacity: 1; }

/* Shelves Rail */
.shelves-rail {
  background: rgba(var(--primary-rgb), 0.1);
  backdrop-filter: blur(20px);
  border-radius: 20px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border: 1px solid rgba(255,255,255,0.05);
}

.rail-header { display: flex; justify-content: space-between; align-items: center; padding: 0 0.5rem; }
.rail-title { font-weight: 800; font-size: 0.9rem; letter-spacing: 1px; }
.rail-hint { font-size: 0.75rem; opacity: 0.5; font-style: italic; }

.shelves-container {
  display: flex;
  gap: 1rem;
  overflow-x: auto;
  padding: 0.5rem;
}

.shelf-card {
  min-width: 120px;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 15px;
  padding: 0.8rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.shelf-card.active {
  background: var(--primary-color);
  border-color: transparent;
  box-shadow: 0 0 20px rgba(var(--primary-rgb), 0.4);
}

.shelf-card.drag-over {
  border-color: var(--primary-color);
  background: rgba(var(--primary-rgb), 0.25);
  transform: scale(1.08);
  box-shadow: 0 0 15px var(--primary-color);
}

.shelf-card.desktop-link {
  flex-direction: row;
  justify-content: center;
}

.shelf-card:hover:not(.active):not(.drag-over) {
  background: rgba(255,255,255,0.1);
  border-color: var(--primary-color);
}

.shelf-top { display: flex; justify-content: space-between; width: 100%; align-items: flex-start; }
.s-icon { font-size: 1.5rem; transition: transform 0.2s; }
.drag-over .s-icon { transform: translateY(-2px); }
.s-name { font-weight: 700; font-size: 0.85rem; }

.s-del {
  background: none;
  border: none;
  color: #ff5f5f;
  font-size: 0.9rem;
  cursor: pointer;
  padding: 0;
  opacity: 0.3;
}
.s-del:hover { opacity: 1; }

.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
}
.modal-card.mini {
  background: var(--card-bg);
  padding: 2rem;
  border-radius: 20px;
  width: 320px;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.modal-card input {
  padding: 0.8rem;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  background: rgba(255,255,255,0.05);
  color: white;
}

.modal-actions { display: flex; justify-content: flex-end; gap: 1rem; }
.confirm-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 8px;
  font-weight: 700;
}
</style>
