<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue';
import { apiService } from '../services/api';
import BookmarkTreeNode from './BookmarkTreeNode.vue';

interface Bookmark {
  id: string;
  parentId?: string | null;
  title: string;
  url?: string | null;
  category: string;
  passwordId?: string | null;
  isFolder: boolean;
  sortOrder: number;
  createdAt?: string;
}

const props = defineProps<{
  userId: string;
  hasSecurityTrust: boolean;
  deviceId?: string;
}>();

const emit = defineEmits(['request-verify']);

const savedFolder = localStorage.getItem('bookmark_last_folder') || 'root';
const savedPath = localStorage.getItem('bookmark_last_path');

const bookmarks = ref<Bookmark[]>([]);
const allBookmarks = ref<Bookmark[]>([]);
const vaultPasswords = ref<any[]>([]);
const showAddModal = ref(false);
const editingId = ref<string | null>(null);
const loading = ref(false);
const currentFolderId = ref<string | 'root'>(savedFolder);
const breadcrumbs = ref<{id: string, title: string}[]>(savedPath ? JSON.parse(savedPath) : []);

// Persist on change
watch([currentFolderId, breadcrumbs], () => {
  localStorage.setItem('bookmark_last_folder', currentFolderId.value);
  localStorage.setItem('bookmark_last_path', JSON.stringify(breadcrumbs.value));
}, { deep: true });

const newBookmark = ref({
  title: '',
  url: '',
  category: 'General',
  password_id: '' as string | null,
  isFolder: false,
  parentId: null as string | null
});

// Drag and drop state
const draggedItem = ref<any>(null);
const dropTargetId = ref<string | null>(null);
const isDropOverRoot = ref(false);

const categories = ['General', 'Work', 'Social', 'Dev', 'Entertainment', 'Tools'];

const fetchBookmarks = async () => {
  loading.value = true;
  try {
    const res = await apiService.getBookmarks(currentFolderId.value === 'root' ? undefined : currentFolderId.value);
    bookmarks.value = res;
    
    // FETCH ALL RECURSIVELY for tree
    const allRes = await apiService.getBookmarks(undefined, true);
    allBookmarks.value = allRes;
  } catch (err) {
    console.error("Failed to fetch bookmarks:", err);
  } finally {
    loading.value = false;
  }
};

const treeData = computed(() => {
  const map: any = {};
  const roots: any[] = [];
  const items = allBookmarks.value.map(b => ({ ...b, children: [] }));
  items.forEach(b => map[b.id] = b);
  items.forEach(b => {
    if (b.parentId && map[b.parentId]) {
      map[b.parentId].children.push(b);
    } else {
      roots.push(b);
    }
  });
  return roots;
});

const enterFolder = (folder: Bookmark) => {
  currentFolderId.value = folder.id;
  breadcrumbs.value.push({ id: folder.id, title: folder.title });
  fetchBookmarks();
};

const goBack = (id: string | 'root' | any) => {
  if (typeof id === 'object' && id !== null) {
      if (id.isFolder) {
          currentFolderId.value = id.id;
          const path = [];
          let curr = id;
          while (curr) {
              path.unshift({ id: curr.id, title: curr.title });
              curr = allBookmarks.value.find((b: any) => b.id === curr.parentId);
          }
          breadcrumbs.value = path;
          fetchBookmarks();
      } else {
          openEditModal(id);
      }
    return;
  }

  if (id === 'root') {
    currentFolderId.value = 'root';
    breadcrumbs.value = [];
  } else {
    const index = breadcrumbs.value.findIndex(b => b.id === id);
    if (index !== -1) {
      breadcrumbs.value = breadcrumbs.value.slice(0, index + 1);
      currentFolderId.value = id;
    }
  }
  fetchBookmarks();
};

const handleDragStart = (item: any) => {
  draggedItem.value = item;
};

const handleDragEnd = () => {
  draggedItem.value = null;
  dropTargetId.value = null;
  isDropOverRoot.value = false;
};

const handleDragOver = (id: string, isFolder: boolean = true) => {
  if (draggedItem.value?.id === id) return;
  if (!isFolder) return;
  dropTargetId.value = id;
};

const handleDragLeave = () => {
    dropTargetId.value = null;
};

const handleDrop = async (targetId: string | 'root') => {
  if (!draggedItem.value) return;
  const tId = targetId === 'root' ? null : targetId;
  if (draggedItem.value.id === tId) return;

  try {
    await apiService.updateBookmark(draggedItem.value.id, {
      ...draggedItem.value,
      parentId: tId
    });
    fetchBookmarks();
  } catch (err) {
    console.error("Move failed:", err);
  } finally {
    draggedItem.value = null;
    dropTargetId.value = null;
    isDropOverRoot.value = false;
  }
};

const fetchVault = async () => {
  if (!props.userId) return;
  try {
    const res = await apiService.getPasswords();
    vaultPasswords.value = res;
  } catch (err) {
    console.warn("Failed to fetch vault for linking:", err);
  }
};

const openAddModal = () => {
  editingId.value = null;
  newBookmark.value = {
    title: '',
    url: '',
    category: 'General',
    password_id: '',
    isFolder: false,
    parentId: currentFolderId.value === 'root' ? null : currentFolderId.value
  };
  showAddModal.value = true;
};

const openEditModal = (bookmark: Bookmark) => {
  editingId.value = bookmark.id;
  newBookmark.value = {
    title: bookmark.title,
    url: bookmark.url || '',
    category: bookmark.category || 'General',
    password_id: bookmark.passwordId || '',
    isFolder: bookmark.isFolder || false,
    parentId: bookmark.parentId || null
  };
  showAddModal.value = true;
};

const saveBookmark = async () => {
  if (!newBookmark.value.title.trim()) return;

  try {
    const payload = {
      title: newBookmark.value.title,
      url: newBookmark.value.isFolder ? null : newBookmark.value.url,
      category: newBookmark.value.category,
      passwordId: newBookmark.value.password_id || null,
      isFolder: newBookmark.value.isFolder,
      parentId: newBookmark.value.parentId
    };

    if (editingId.value) {
      await apiService.updateBookmark(editingId.value, payload);
    } else {
      await apiService.addBookmark(payload);
    }
    showAddModal.value = false;
    fetchBookmarks();
  } catch (err) {
    console.error('Failed to save bookmark:', err);
  }
};

const getDisplayUrl = (url: string) => {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
};

const getFavicon = (url: string) => {
  try {
    const domain = new URL(url).hostname;
    return `https://www.google.com/s2/favicons?domain=${domain}&sz=64`;
  } catch {
    return "";
  }
};

const isWithinGracePeriod = (bookmark: Bookmark) => {
  if (!bookmark.createdAt) return false;
  const createdAtTime = new Date(bookmark.createdAt).getTime();
  const now = new Date().getTime();
  const diffMinutes = (now - createdAtTime) / (1000 * 60);
  return diffMinutes < 30;
};

const copyToClipboard = async (bookmark: Bookmark) => {
  if (bookmark.passwordId && !isWithinGracePeriod(bookmark)) {
    if (!props.hasSecurityTrust) {
      emit('request-verify');
      return;
    }
  }

  try {
    if (!bookmark.url) return;
    await navigator.clipboard.writeText(bookmark.url);
  } catch (err) {
    console.error('Failed to copy: ', err);
  }
};

const confirmDelete = async (bookmark: Bookmark) => {
  if (bookmark.passwordId && !isWithinGracePeriod(bookmark)) {
    if (!props.hasSecurityTrust) {
      emit('request-verify');
      return;
    }
  }

  if (confirm(`Are you sure you want to delete "${bookmark.title}"?`)) {
    try {
      await apiService.deleteBookmark(bookmark.id);
      fetchBookmarks();
    } catch (err) {
      console.error("Delete failed:", err);
    }
  }
};

const addToDesk = async (bookmark: Bookmark) => {
  try {
    await apiService.addDeskItem({
      type: 'bookmark',
      refId: bookmark.id,
      shelfId: null,
      sortOrder: 0
    });
    // Trigger a small toast or just a console log
    console.log("Pinned to desk:", bookmark.title);
  } catch (err) {
    console.error("Failed to add to desk:", err);
  }
};

onMounted(() => {
  fetchBookmarks();
  fetchVault();
});

watch(() => props.userId, () => {
  fetchBookmarks();
  fetchVault();
});
</script>

<script lang="ts">
export default {
  name: "BookmarkGrid"
};
</script>

<template>
  <div class="bookmark-explorer-container">
    <div class="tree-sidebar">
      <div 
        class="sidebar-header-root" 
        :class="{ active: currentFolderId === 'root', 'drop-over': isDropOverRoot }"
        @click="goBack('root')"
        @dragover.prevent="isDropOverRoot = true"
        @dragleave="isDropOverRoot = false"
        @drop="handleDrop('root'); isDropOverRoot = false"
      >
        🏠 Root
      </div>
      <div class="tree-body">
        <div v-for="node in treeData" :key="node.id">
          <BookmarkTreeNode 
            :node="node" 
            :current-id="currentFolderId"
            @select="goBack"
            @drop-on-node="(data: any) => handleDrop(data.targetNode.id)"
            @drag-start="handleDragStart"
            @drag-end="handleDragEnd"
          />
        </div>
      </div>
    </div>

    <div class="main-panel">
      <div class="section-header">
        <div class="title-group">
          <h3>🌐 雲端書籤 (Cloud Bookmarks)</h3>
          <p class="subtitle">Sync your favorite links with dual-platform password protection.</p>
        </div>
        <div class="header-actions">
          <button @click="openAddModal" class="add-btn">
            <span>+</span> Add
          </button>
        </div>
      </div>

      <div class="breadcrumbs-row" v-if="currentFolderId !== 'root' || breadcrumbs.length > 0">
        <span 
          class="breadcrumb-item" 
          :class="{ 'drop-target-breadcrumb': dropTargetId === 'root' }"
          @click="goBack('root')"
          @dragover.prevent="handleDragOver('root')"
          @dragleave="handleDragLeave"
          @drop="handleDrop('root')"
        >🏠 Root</span>
        <template v-for="crumb in breadcrumbs" :key="crumb.id">
          <span class="breadcrumb-separator">›</span>
          <span 
            class="breadcrumb-item" 
            :class="{ 'drop-target-breadcrumb': dropTargetId === crumb.id }"
            @click="goBack(crumb.id)"
            @dragover.prevent="handleDragOver(crumb.id)"
            @dragleave="handleDragLeave"
            @drop="handleDrop(crumb.id)"
          >{{ crumb.title }}</span>
        </template>
      </div>

      <div v-if="bookmarks.length === 0" class="empty-state card">
        <div class="empty-icon">📂</div>
        <p>這裡目前是空的。點點右上角來新增吧！</p>
      </div>

      <div v-else class="bookmark-grid">
        <div 
          v-for="bm in bookmarks" 
          :key="bm.id" 
          class="bookmark-card card" 
          :class="{ 
            protected: bm.passwordId,
            'is-folder': bm.isFolder,
            'is-dragging': draggedItem?.id === bm.id,
            'drop-target': dropTargetId === bm.id
          }"
          :draggable="true"
          @dragstart="handleDragStart(bm)"
          @dragover.prevent="handleDragOver(bm.id, bm.isFolder)"
          @dragleave="handleDragLeave"
          @drop="handleDrop(bm.id)"
          @dragend="handleDragEnd"
        >
          <div class="card-bg-glow"></div>
          
          <div class="card-header">
            <div class="favicon-wrapper" @click="bm.isFolder && enterFolder(bm)">
              <template v-if="bm.isFolder">📁</template>
              <template v-else-if="bm.url && getFavicon(bm.url)">
                <img :src="getFavicon(bm.url)" alt="icon" @error="(e: any) => e.target.style.display = 'none'" />
              </template>
              <span v-else class="default-icon">🔗</span>
            </div>
            <div class="header-right">
              <span v-if="bm.passwordId" class="lock-indicator" :class="{ unlocked: hasSecurityTrust }">
                {{ hasSecurityTrust ? '🔓' : '🔒' }}
              </span>
              <span class="category-tag">{{ bm.category }}</span>
            </div>
          </div>

          <div class="card-body" @click="bm.isFolder && enterFolder(bm)">
            <h4 class="bm-title" :title="bm.title">{{ bm.title }}</h4>
            <p class="bm-url">{{ bm.isFolder ? 'Folder' : getDisplayUrl(bm.url || '') }}</p>
          </div>

          <div class="card-actions">
            <template v-if="!bm.isFolder">
              <a :href="bm.url || '#'" target="_blank" class="action-btn launch" title="Open Link">🚀 Open</a>
              <button @click="copyToClipboard(bm)" class="action-btn copy" :class="{ 'verify-needed': bm.passwordId && !hasSecurityTrust }" title="Copy URL">
                {{ bm.passwordId && !hasSecurityTrust && !isWithinGracePeriod(bm) ? '🔑 Verify' : '📋 Copy' }}
              </button>
            </template>
            <template v-else>
              <button @click="enterFolder(bm)" class="action-btn launch">📂 Open Folder</button>
            </template>
            <button @click="addToDesk(bm)" class="action-btn pin" title="Add to Desk">📌</button>
            <button @click="confirmDelete(bm)" class="action-btn delete" title="Delete">🗑️</button>
          </div>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
        <div class="modal-content card glow">
          <div class="modal-header">
            <h3>{{ editingId ? 'Edit Bookmark' : 'Add New Bookmark' }}</h3>
          </div>
          
          <div class="form-grid">
            <div class="form-group">
              <label>Title</label>
              <input v-model="newBookmark.title" placeholder="e.g. My Folder or Google" />
            </div>
            <div class="form-group">
              <label>Type</label>
              <div class="type-toggle">
                <button :class="{ active: !newBookmark.isFolder }" @click="newBookmark.isFolder = false" :disabled="!!editingId">🔗 Link</button>
                <button :class="{ active: newBookmark.isFolder }" @click="newBookmark.isFolder = true" :disabled="!!editingId">📁 Folder</button>
              </div>
            </div>
          </div>

          <div class="form-grid" v-if="!newBookmark.isFolder">
            <div class="form-group">
              <label>URL</label>
              <input v-model="newBookmark.url" placeholder="e.g. google.com" />
            </div>
            <div class="form-group">
              <label>Category</label>
              <select v-model="newBookmark.category">
                <option v-for="cat in categories" :key="cat" :value="cat">{{ cat }}</option>
              </select>
            </div>
          </div>

          <div class="form-group security-link">
            <label>🔑 Link to Password Vault (Optional)</label>
            <select v-model="newBookmark.password_id">
              <option value="">-- No Password Protection --</option>
              <option v-for="p in vaultPasswords" :key="p.id" :value="p.id">
                {{ p.site_name }} ({{ p.account }})
              </option>
            </select>
          </div>

          <div class="modal-actions">
            <button @click="showAddModal = false" class="cancel-btn">Cancel</button>
            <button @click="saveBookmark" class="confirm-btn">{{ editingId ? 'Update' : 'Create' }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.bookmark-explorer-container {
  display: flex;
  gap: 1.25rem;
  height: 100%;
}

.tree-sidebar {
  width: 250px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  backdrop-filter: blur(10px);
}

.sidebar-header-root {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.75rem 1rem;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 800;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  color: var(--secondary-color);
  margin-bottom: 1.25rem;
  border-bottom: 1px solid var(--border-color);
}

.sidebar-header-root.active {
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
}

.sidebar-header-root.drop-over {
  background: var(--primary-color) !important;
  color: white;
  transform: scale(1.05);
  box-shadow: 0 0 20px var(--primary-color);
}

.tree-body {
  flex: 1;
  overflow-y: auto;
}

.main-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 2rem;
  padding-left: 1.2rem;
}

.add-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 0.6rem 1.2rem;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
}

.bookmark-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
  padding: 1.5rem;
  overflow-y: auto;
}

.bookmark-card {
  height: 140px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  position: relative;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  overflow: hidden;
}

.bookmark-card:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: var(--primary-color);
  transform: translateY(-5px);
  box-shadow: 0 10px 30px rgba(0,0,0,0.3);
}

.card-bg-glow {
  position: absolute;
  top: -50%;
  left: -50%;
  width: 200%;
  height: 200%;
  background: radial-gradient(circle at center, rgba(var(--primary-rgb), 0.15) 0%, transparent 60%);
  opacity: 0;
  transition: opacity 0.3s;
  pointer-events: none;
}

.bookmark-card:hover .card-bg-glow {
  opacity: 1;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.favicon-wrapper {
  width: 32px;
  height: 32px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 5px;
}

.favicon-wrapper img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.category-tag {
  font-size: 0.65rem;
  font-weight: 800;
  text-transform: uppercase;
  padding: 0.15rem 0.5rem;
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
  border-radius: 10px;
}

.card-body {
  text-align: left;
}

.bm-title {
  margin: 0;
  font-size: 1rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 700;
}

.bm-url {
  font-size: 0.75rem;
  opacity: 0.5;
  margin: 0;
}

.card-actions {
  display: flex;
  gap: 0.5rem;
}

.action-btn {
  flex: 1;
  padding: 0.35rem;
  border-radius: 6px;
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255,255,255,0.05);
  color: white;
  text-decoration: none;
  text-align: center;
}

.action-btn:hover {
  background: var(--primary-color);
  border-color: var(--primary-color);
}

.action-btn.delete:hover {
  background: #ff5757;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0,0,0,0.85);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 3000;
}

.modal-content {
  width: 90%;
  max-width: 500px;
  padding: 2.5rem;
  background: #1e1e24;
  border-radius: 20px;
}

.form-group {
  margin-bottom: 1.2rem;
  text-align: left;
}

.form-group label {
  display: block;
  font-size: 0.8rem;
  margin-bottom: 0.5rem;
  opacity: 0.8;
}

input, select {
  width: 100%;
  padding: 0.75rem;
  background: rgba(0,0,0,0.2);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  color: white;
}

.type-toggle {
  display: flex;
  gap: 0.5rem;
}

.type-toggle button {
  flex: 1;
  padding: 0.5rem;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 6px;
  color: white;
  cursor: pointer;
}

.type-toggle button.active {
  background: var(--primary-color);
  border-color: var(--primary-color);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 1.5rem;
}

.cancel-btn, .confirm-btn {
  padding: 0.6rem 1.5rem;
  border-radius: 8px;
  cursor: pointer;
  font-weight: bold;
}

.confirm-btn {
  background: var(--primary-color);
  border: none;
  color: white;
}

.breadcrumbs-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  font-size: 0.9rem;
  opacity: 0.8;
}

.breadcrumb-item {
  cursor: pointer;
  transition: color 0.2s;
}

.breadcrumb-item:hover {
  color: var(--primary-color);
}
</style>
