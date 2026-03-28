<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { apiService, socket } from '../services/api';
import SnippetTreeNode from './SnippetTreeNode.vue';

const props = defineProps<{
  userId: string;
}>();

const allSnippets = ref<any[]>([]);
const snippets = ref<any[]>([]);
const pathStack = ref<any[]>([]);
const currentFolderId = ref<string | 'root'>('root');
const loading = ref(true);

// Modal state
const showAddModal = ref(false);
const isEditing = ref(false);
const editingId = ref<string | null>(null);
const newItemName = ref('');
const newItemContent = ref('');
const newItemIsFolder = ref(false);

const fetchData = async () => {
  loading.value = true;
  try {
    // 1. Fetch ALL snippets for the tree view
    allSnippets.value = await apiService.getSnippets(props.userId, undefined, true);
    
    // 2. Fetch current folder's snippets for the main view
    snippets.value = await apiService.getSnippets(props.userId, currentFolderId.value === 'root' ? null : currentFolderId.value);
  } catch (err) {
    console.error("Failed to fetch snippets:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchData();
  socket.on('snippetsUpdate', fetchData);
});

// Build Tree Structure for Sidebar
const treeData = computed(() => {
  const map: any = {};
  const roots: any[] = [];
  
  const items = allSnippets.value.map(item => ({ ...item, children: [], isOpen: false }));
  
  items.forEach(item => {
    map[item.id] = item;
  });
  
  items.forEach(item => {
    if (item.parent_id) {
      if (map[item.parent_id]) {
        map[item.parent_id].children.push(item);
      }
    } else {
      roots.push(item);
    }
  });
  
  return roots;
});

const enterFolder = (folder: any) => {
  if (!folder) {
    currentFolderId.value = 'root';
    pathStack.value = [];
  } else {
    currentFolderId.value = folder.id;
    // Rebuild path stack
    const newStack = [];
    let curr = folder;
    while (curr) {
      newStack.unshift(curr);
      curr = allSnippets.value.find(s => s.id === curr.parent_id);
    }
    pathStack.value = newStack;
  }
  fetchData();
};

const goBack = () => {
  if (pathStack.value.length > 0) {
    pathStack.value.pop();
    const parent = pathStack.value[pathStack.value.length - 1];
    enterFolder(parent || null);
  } else {
    enterFolder(null);
  }
};

const goHome = () => enterFolder(null);

const openAddModal = () => {
  isEditing.value = false;
  editingId.value = null;
  newItemName.value = '';
  newItemContent.value = '';
  newItemIsFolder.value = false;
  showAddModal.value = true;
};

const openEditModal = (item: any) => {
  isEditing.value = true;
  editingId.value = item.id;
  newItemName.value = item.name;
  newItemContent.value = item.content || '';
  newItemIsFolder.value = item.is_folder;
  showAddModal.value = true;
};

const saveItem = async () => {
  if (!newItemName.value.trim()) return;
  
  try {
    if (isEditing.value && editingId.value) {
      await apiService.updateSnippet(editingId.value, {
        name: newItemName.value,
        content: newItemContent.value
      });
    } else {
      await apiService.createSnippet({
        userId: props.userId,
        parentId: currentFolderId.value === 'root' ? null : currentFolderId.value,
        name: newItemName.value,
        content: newItemContent.value,
        isFolder: newItemIsFolder.value
      });
    }
    showAddModal.value = false;
    await fetchData();
  } catch (err) {
    alert("Save failed");
  }
};

const deleteItem = async (id: string) => {
  if (confirm("Are you sure? This will delete all contents if it's a folder.")) {
    try {
      await apiService.deleteSnippet(id);
      await fetchData();
    } catch (err) {
      alert("Delete failed");
    }
  }
};

const copyText = (text: string) => {
  navigator.clipboard.writeText(text);
  alert("Copied to clipboard!");
};
</script>

<template>
  <div class="snippet-explorer-container">
    <div class="tree-sidebar">
      <div class="sidebar-header">
        <span @click="goHome" class="root-link" :class="{ active: currentFolderId === 'root' }">🏠 All Snippets</span>
      </div>
      <div class="tree-body">
        <div v-for="node in treeData" :key="node.id">
          <SnippetTreeNode 
            :node="node" 
            :current-id="currentFolderId"
            @select="enterFolder"
          />
        </div>
      </div>
    </div>

    <div class="main-explorer snippet-explorer">
      <div class="explorer-header">
        <div class="breadcrumbs">
          <span @click="goHome" class="crumb home">🏠 Root</span>
          <span v-for="(folder, index) in pathStack" :key="folder.id" class="crumb-wrapper">
            <span class="sep">›</span>
            <span @click="enterFolder(folder)" class="crumb" :class="{ last: index === pathStack.length - 1 }">{{ folder.name }}</span>
          </span>
        </div>
        <button @click="openAddModal" class="add-btn">+ New</button>
      </div>

      <div v-if="loading" class="mini-loader">Loading snippets...</div>
      
      <div v-else class="explorer-body">
        <div v-if="pathStack.length > 0" @click="goBack" class="item-row back">
          📁 .. (Back)
        </div>
        
        <div v-for="item in snippets" :key="item.id" class="item-row" :class="{ 'is-selected': item.id === editingId }">
          <div v-if="item.is_folder" class="item-content folder">
            <div @click="enterFolder(item)" class="folder-link">
              📁 <strong>{{ item.name }}</strong>
            </div>
            <div class="item-actions">
              <button @click="openEditModal(item)" class="edit-small">✎</button>
              <button @click="deleteItem(item.id)" class="del-small">✕</button>
            </div>
          </div>
          <div v-else class="item-content snippet">
            <div class="snippet-info">
              <span class="snippet-name">📄 {{ item.name }}</span>
              <p v-if="item.content" class="snippet-preview">{{ item.content.substring(0, 50) }}{{ item.content.length > 50 ? '...' : '' }}</p>
            </div>
            <div class="item-actions">
              <button @click="copyText(item.content)" class="copy-small">📋 Copy</button>
              <button @click="openEditModal(item)" class="edit-small">✎</button>
              <button @click="deleteItem(item.id)" class="del-small">✕</button>
            </div>
          </div>
        </div>
        
        <div v-if="snippets.length === 0 && pathStack.length === 0" class="empty-hint">
          Your personal clipboard is empty.
        </div>
      </div>
    </div>

    <!-- Modal for New/Edit Item -->
    <Teleport to="body">
      <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
        <div class="modal card">
          <h3>{{ isEditing ? 'Edit Item' : 'Create New Item' }}</h3>
          <div v-if="!isEditing" class="form-group">
            <label>Type:</label>
            <div class="type-toggle">
              <button @click="newItemIsFolder = false" :class="{ active: !newItemIsFolder }">📄 Snippet</button>
              <button @click="newItemIsFolder = true" :class="{ active: newItemIsFolder }">📁 Folder</button>
            </div>
          </div>
          <div class="form-group">
            <label>Name:</label>
            <input v-model="newItemName" placeholder="e.g., Breakfast Order" />
          </div>
          <div v-if="!newItemIsFolder" class="form-group">
            <label>Content:</label>
            <textarea v-model="newItemContent" placeholder="Paste your text here..."></textarea>
          </div>
          <div class="modal-actions">
            <button @click="showAddModal = false" class="cancel-btn">Cancel</button>
            <button @click="saveItem" class="confirm-btn">{{ isEditing ? 'Update' : 'Create' }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.snippet-explorer-container {
  display: flex;
  gap: 1.5rem;
  height: 600px;
  margin-top: 1rem;
}

.tree-sidebar {
  width: 280px;
  background: rgba(255, 255, 255, 0.03);
  border: 2px solid var(--border-color);
  border-radius: 20px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  backdrop-filter: blur(10px);
}

.sidebar-header {
  margin-bottom: 1.5rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border-color);
}

.root-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem 0.8rem;
  border-radius: 8px;
  cursor: pointer;
  font-weight: bold;
  transition: all 0.2s ease;
  color: var(--secondary-color);
}

.root-link:hover {
  background: rgba(255, 255, 255, 0.05);
}

.root-link.active {
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
}

.tree-body {
  flex: 1;
  overflow-y: auto;
  padding-right: 0.5rem;
}

/* Custom Scrollbar for Sidebar */
.tree-body::-webkit-scrollbar {
  width: 4px;
}
.tree-body::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 10px;
}

.main-explorer {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.03);
  border: 2px solid var(--border-color);
  border-radius: 20px;
  overflow: hidden;
  backdrop-filter: blur(10px);
}

.explorer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.25rem 1.75rem;
  background: rgba(255, 255, 255, 0.02);
  border-bottom: 1px solid var(--border-color);
}

.breadcrumbs {
  display: flex;
  align-items: center;
  font-size: 0.95rem;
  gap: 0.5rem;
}

.crumb {
  cursor: pointer;
  opacity: 0.6;
  transition: all 0.2s ease;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}

.crumb:hover {
  opacity: 1;
  background: rgba(255,255,255,0.1);
  color: var(--primary-color);
}

.crumb.home {
  font-weight: bold;
}

.crumb.last {
  opacity: 1;
  color: var(--secondary-color);
  pointer-events: none;
}

.crumb-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.sep {
  opacity: 0.3;
  font-size: 1.2rem;
}

.add-btn {
  background: linear-gradient(135deg, var(--primary-color), var(--accent-color));
  color: white;
  border: none;
  padding: 0.5rem 1.2rem;
  border-radius: 8px;
  font-weight: bold;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.add-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
}

.explorer-body {
  padding: 0.75rem;
  max-height: 500px;
  overflow-y: auto;
}

.item-row {
  display: flex;
  align-items: center;
  padding: 1rem 1.25rem;
  border-radius: 10px;
  margin-bottom: 0.5rem;
  background: rgba(255,255,255,0.02);
  border: 1px solid transparent;
  transition: all 0.2s ease;
  gap: 1rem;
}

.item-row:hover {
  background: rgba(255,255,255,0.05);
  border-color: rgba(255,255,255,0.1);
  transform: translateX(4px);
}

.item-row.back {
  cursor: pointer;
  opacity: 0.6;
  font-style: italic;
}

.item-row.back:hover {
  opacity: 1;
  color: var(--primary-color);
}

.item-content {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.folder-link {
  cursor: pointer;
  color: var(--secondary-color);
  font-size: 1.05rem;
  flex: 1;
}

.item-actions {
  display: flex;
  gap: 0.5rem;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.item-row:hover .item-actions {
  opacity: 1;
}

.snippet-info {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.snippet-name {
  font-weight: bold;
  font-size: 1rem;
}

.snippet-preview {
  margin: 0.25rem 0 0 0;
  font-size: 0.85rem;
  opacity: 0.5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 300px;
}

.copy-small, .edit-small, .del-small {
  background: rgba(255,255,255,0.08);
  border: 1px solid rgba(255,255,255,0.1);
  color: var(--text-color);
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s ease;
}

.copy-small:hover { background: var(--primary-color); border-color: transparent; }
.edit-small:hover { background: var(--secondary-color); border-color: transparent; }
.del-small { color: #ef4444; }
.del-small:hover { background: #ef4444; color: white; border-color: transparent; }

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.85);
  backdrop-filter: blur(8px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal {
  width: 90%;
  max-width: 450px;
  padding: 2rem;
  background: var(--card-bg);
  border: 2px solid var(--border-color);
  border-radius: 20px;
  box-shadow: 0 20px 50px rgba(0,0,0,0.5);
}

.form-group {
  margin-bottom: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.form-group label {
  font-size: 0.9rem;
  font-weight: bold;
  opacity: 0.8;
  letter-spacing: 0.05rem;
}

.form-group input, .form-group textarea {
  background: rgba(0,0,0,0.2);
  border: 1px solid var(--border-color);
  padding: 0.75rem 1rem;
  border-radius: 10px;
  color: var(--text-color);
  font-size: 1rem;
  outline: none;
  transition: border-color 0.2s ease;
}

.form-group input:focus, .form-group textarea:focus {
  border-color: var(--primary-color);
}

.form-group textarea {
  height: 120px;
  resize: vertical;
}

.type-toggle {
  display: flex;
  gap: 0.75rem;
}

.type-toggle button {
  flex: 1;
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.6rem;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.type-toggle button.active {
  background: var(--primary-color);
  color: white;
  border-color: transparent;
  box-shadow: 0 4px 12px rgba(var(--primary-rgb), 0.3);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 2rem;
}

.cancel-btn {
  background: transparent;
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.6rem 1.5rem;
  border-radius: 8px;
  cursor: pointer;
}

.confirm-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 0.6rem 1.5rem;
  border-radius: 8px;
  font-weight: bold;
  cursor: pointer;
}

.empty-hint {
  text-align: center;
  padding: 4rem 2rem;
  opacity: 0.3;
  font-style: italic;
  font-size: 1.1rem;
}
</style>
