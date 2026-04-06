<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import { apiService, socket } from '../services/api';
import SnippetTreeNode from './SnippetTreeNode.vue';
import { marked } from 'marked';

const props = defineProps<{
  userId: string;
}>();

const allSnippets = ref<any[]>([]);
const snippets = ref<any[]>([]);

// Persistent state restoration
const savedFolder = localStorage.getItem('snippet_last_folder') || 'root';
const savedPath = localStorage.getItem('snippet_last_path');

const pathStack = ref<any[]>(savedPath ? JSON.parse(savedPath) : []);
const currentFolderId = ref<string | 'root'>(savedFolder);
const loading = ref(true);

// Watcher to save state on change
watch([currentFolderId, pathStack], () => {
  localStorage.setItem('snippet_last_folder', currentFolderId.value);
  localStorage.setItem('snippet_last_path', JSON.stringify(pathStack.value));
}, { deep: true });

// Modal state
const showAddModal = ref(false);
const isEditing = ref(false);
const editingId = ref<string | null>(null);
const newItemName = ref('');
const newItemContent = ref('');
const newItemIsFolder = ref(false);
const isFullScreen = ref(false);
const draggedItem = ref<any>(null);
const dropTargetId = ref<string | null>(null);
const isDropOverRoot = ref(false);
const editMode = ref<'txt' | 'md'>('txt');

const fetchData = async () => {
  // If we have a token, the backend will identify us. We don't strictly need props.userId
  // but we wait for loading to be false initially.
  loading.value = true;
  try {
    // 1. Fetch ALL snippets for the tree view
    allSnippets.value = await apiService.getSnippets(undefined, true);
    
    // 2. Fetch current folder's snippets for the main view
    snippets.value = await apiService.getSnippets(currentFolderId.value === 'root' ? null : currentFolderId.value);
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

// WATCH for identity resolution - CRITICAL for first load persistence
watch(() => props.userId, (newVal) => {
  if (newVal) fetchData();
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
    if (item.parentId) {
      if (map[item.parentId]) {
        map[item.parentId].children.push(item);
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
  } else if (!folder.isFolder) {
    // If it's a snippet, open edit modal!
    openEditModal(folder);
    return;
  } else {
    currentFolderId.value = folder.id;
    // Rebuild path stack
    const newStack = [];
    let curr = folder;
    while (curr) {
      newStack.unshift(curr);
      curr = allSnippets.value.find(s => s.id === curr.parentId);
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

const editingParentId = ref<string | null>(null);

const openEditModal = (item: any) => {
  isEditing.value = true;
  editingId.value = item.id;
  editingParentId.value = item.parentId; // Store original parentId
  newItemName.value = item.name;
  newItemContent.value = item.content || '';
  newItemIsFolder.value = item.isFolder;
  isFullScreen.value = false; 
  editMode.value = 'txt'; // Reset to text mode on open
  showAddModal.value = true;
};

const saveItem = async () => {
  if (!newItemName.value.trim()) return;
  
  try {
    if (isEditing.value && editingId.value) {
      await apiService.updateSnippet(editingId.value, {
        name: newItemName.value,
        content: newItemContent.value,
        parentId: editingParentId.value // Keep it where it was
      });
    } else {
      await apiService.createSnippet({
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

// JSON Export
const exportToJSON = () => {
  const dataStr = JSON.stringify(allSnippets.value, null, 2);
  const dataUri = 'data:application/json;charset=utf-8,'+ encodeURIComponent(dataStr);
  const exportFileDefaultName = `kitty_snippets_${new Date().toISOString().split('T')[0]}.json`;
  
  const linkElement = document.createElement('a');
  linkElement.setAttribute('href', dataUri);
  linkElement.setAttribute('download', exportFileDefaultName);
  linkElement.click();
};

// JSON Import
const fileInput = ref<HTMLInputElement | null>(null);
const importFromJSON = async (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (!target.files || !target.files[0]) return;
  
  const file = target.files[0];
  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const importedData = JSON.parse(e.target?.result as string);
      if (!Array.isArray(importedData)) throw new Error("Invalid format");
      
      loading.value = true;
      for (const item of importedData) {
        // Try to create snippets, ignoring existing ones or duplicates for now
        // A better way would be to check if they exist, but for simplicity we'll just push
        await apiService.createSnippet({
          parentId: null, // Importing to root for safety
          name: item.name + ' (Imported)',
          content: item.content,
          isFolder: item.isFolder || item.is_folder // Support both for now
        });
      }
      alert(`Successfully imported ${importedData.length} items!`);
      await fetchData();
    } catch (err) {
      alert("Import failed: " + (err as Error).message);
    } finally {
      loading.value = false;
      if (fileInput.value) fileInput.value.value = '';
    }
  };
  reader.readAsText(file);
};

// Voice Input Logic
const isRecordingName = ref(false);
const isRecordingContent = ref(false);
const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
const recognition = SpeechRecognition ? new SpeechRecognition() : null;

if (recognition) {
  recognition.lang = 'zh-TW';
  recognition.continuous = false;
  recognition.interimResults = false;
  
  recognition.onresult = (event: any) => {
    const transcript = event.results[0][0].transcript;
    if (isRecordingName.value) newItemName.value += transcript;
    if (isRecordingContent.value) newItemContent.value += transcript;
    isRecordingName.value = false;
    isRecordingContent.value = false;
  };

  recognition.onend = () => {
    isRecordingName.value = false;
    isRecordingContent.value = false;
  };
}

const toggleVoice = (target: 'name' | 'content') => {
  if (!recognition) {
    alert("Speech recognition not supported in this browser.");
    return;
  }
  
  if (isRecordingName.value || isRecordingContent.value) {
    recognition.stop();
  } else {
    if (target === 'name') isRecordingName.value = true;
    else isRecordingContent.value = true;
    recognition.start();
  }
};

// --- DRAG & DROP LOGIC ---
const handleDragStart = (item: any) => {
  draggedItem.value = item;
};

const handleDragEnd = () => {
  draggedItem.value = null;
  dropTargetId.value = null;
  isDropOverRoot.value = false;
};

const handleDragOver = (item: any) => {
  if (draggedItem.value?.id === item.id) return;
  dropTargetId.value = item.id;
};

const handleDragLeave = (item: any) => {
  if (dropTargetId.value === item.id) {
    dropTargetId.value = null;
  }
};

const handleDrop = async (targetItem: any | 'root') => {
  if (!draggedItem.value) return;
  
  const targetId = targetItem === 'root' ? null : targetItem.id;
  const targetIsFolder = targetItem === 'root' ? true : targetItem.isFolder;

  if (draggedItem.value.id === targetId) {
    draggedItem.value = null;
    dropTargetId.value = null;
    isDropOverRoot.value = false;
    return;
  }

  try {
    loading.value = true;
    // CASE 1: Move into folder (including root)
    if (targetIsFolder) {
      await apiService.updateSnippet(draggedItem.value.id, {
        name: draggedItem.value.name,
        content: draggedItem.value.content,
        parentId: targetId,
        sortOrder: 0 // Put at top
      });
    } else {
      // CASE 2: Reorder
      let newOrder = targetItem.sortOrder + 1;
      await apiService.updateSnippet(draggedItem.value.id, {
        name: draggedItem.value.name,
        content: draggedItem.value.content,
        parentId: targetItem.parentId,
        sortOrder: newOrder
      });
    }
  } catch (err) {
    console.error("Drop failed:", err);
  } finally {
    draggedItem.value = null;
    dropTargetId.value = null;
    isDropOverRoot.value = false;
    await fetchData();
  }
};
</script>

<template>
  <div class="snippet-explorer-container">
    <div class="tree-sidebar">
      <div class="sidebar-header">
        <span 
          @click="goHome" 
          class="root-link" 
          :class="{ active: currentFolderId === 'root', 'drop-over': isDropOverRoot }"
          @dragover.prevent="isDropOverRoot = true"
          @dragleave="isDropOverRoot = false"
          @drop="handleDrop('root')"
        >🏠 All Snippets</span>
      </div>
      <div class="tree-body">
        <div v-for="node in treeData" :key="node.id">
          <SnippetTreeNode 
            :node="node" 
            :current-id="currentFolderId"
            @select="enterFolder"
            @drop-on-node="(data) => handleDrop(data.targetNode)"
            @drag-start="handleDragStart"
            @drag-end="handleDragEnd"
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
        <div class="header-actions">
          <button @click="exportToJSON" class="secondary-btn" title="Export to JSON">📤 Export</button>
          <button @click="fileInput?.click()" class="secondary-btn" title="Import from JSON">📥 Import</button>
          <input type="file" ref="fileInput" @change="importFromJSON" accept=".json" class="hidden" />
          <button @click="openAddModal" class="add-btn">+ New</button>
        </div>
      </div>

      <div v-if="loading" class="mini-loader">Loading snippets...</div>
      
      <div v-else class="explorer-body">
        <div v-if="pathStack.length > 0" @click="goBack" class="item-row back">
          📁 .. (Back)
        </div>
        
        <div 
          v-for="item in snippets" 
          :key="item.id" 
          class="item-row" 
          :class="{ 
            'is-selected': item.id === editingId,
            'is-dragging': draggedItem?.id === item.id,
            'is-drop-target': dropTargetId === item.id
          }"
          draggable="true"
          @dragstart="handleDragStart(item)"
          @dragover.prevent="handleDragOver(item)"
          @dragleave="handleDragLeave(item)"
          @drop="handleDrop(item)"
          @dragend="handleDragEnd"
        >
          <div v-if="item.isFolder" class="item-content folder">
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
        <div class="modal card" :class="{ 'is-full': isFullScreen }">
          <div class="modal-header">
            <h3>{{ isEditing ? 'Edit Item' : 'Create New Item' }}</h3>
            <div class="modal-controls">
              <button @click="isFullScreen = !isFullScreen" class="expand-btn" title="Toggle Fullscreen">
                {{ isFullScreen ? '❐' : '⛶' }}
              </button>
              <button @click="showAddModal = false" class="close-modal">✕</button>
            </div>
          </div>
          
          <div v-if="!isEditing" class="form-group">
            <label>Type:</label>
            <div class="type-toggle">
              <button @click="newItemIsFolder = false" :class="{ active: !newItemIsFolder }">📄 Snippet</button>
              <button @click="newItemIsFolder = true" :class="{ active: newItemIsFolder }">📁 Folder</button>
            </div>
          </div>

          <div class="form-group">
            <label>Name:</label>
            <div class="input-with-voice">
              <input v-model="newItemName" placeholder="e.g., Breakfast Order" />
              <button 
                @click="toggleVoice('name')" 
                class="voice-btn" 
                :class="{ recording: isRecordingName }"
                title="Voice Input"
              >
                🎙️
              </button>
            </div>
          </div>

          <div v-if="!newItemIsFolder" class="form-group content-group">
            <div class="field-header">
              <label>Content:</label>
              <div class="edit-mode-toggle">
                <button :class="{ active: editMode === 'txt' }" @click="editMode = 'txt'">TXT</button>
                <button :class="{ active: editMode === 'md' }" @click="editMode = 'md'">MD</button>
              </div>
            </div>
            
            <div class="input-with-voice" v-if="editMode === 'txt'">
              <textarea v-model="newItemContent" placeholder="Paste your text here..."></textarea>
              <button 
                @click="toggleVoice('content')" 
                class="voice-btn textarea-voice" 
                :class="{ recording: isRecordingContent }"
                title="Voice Input"
              >
                🎙️
              </button>
            </div>
            <div v-else class="markdown-preview" v-html="marked.parse(newItemContent || '')"></div>
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
  gap: 1rem;
  height: calc(100vh - 250px);
  min-height: 500px;
  margin-top: 0.5rem;
}

.tree-sidebar {
  width: 240px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
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

.root-link.drop-over {
  background: var(--primary-color) !important;
  color: white;
  transform: scale(1.05);
  box-shadow: 0 0 15px var(--primary-color);
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
  background: rgba(255, 255, 255, 0.01);
  border: 1px solid var(--border-color);
  border-radius: 15px;
  overflow: hidden;
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
  padding: 0.75rem 1rem;
  border-radius: 8px;
  margin-bottom: 0.4rem;
  background: rgba(255,255,255,0.02);
  border: 1px solid rgba(255,255,255,0.02);
  transition: all 0.15s ease;
  gap: 0.8rem;
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
  transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.modal.is-full {
  max-width: 95vw;
  width: 95vw;
  height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal.is-full .content-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.modal.is-full .input-with-voice {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.modal.is-full textarea {
  height: 100% !important;
  flex: 1;
  font-size: 1.1rem;
  resize: none;
}

.expand-btn {
  background: transparent;
  border: none;
  color: var(--secondary-color);
  font-size: 1.1rem;
  cursor: pointer;
  opacity: 0.5;
  transition: all 0.2s;
  padding: 0.2rem 0.5rem;
}

.expand-btn:hover {
  opacity: 1;
  color: var(--primary-color);
  transform: scale(1.2);
}

.modal-controls {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

/* NEW: Markdown Styles */
.field-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.6rem;
}

.edit-mode-toggle {
  display: flex;
  background: rgba(0,0,0,0.3);
  padding: 2px;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.edit-mode-toggle button {
  background: transparent;
  border: none;
  font-size: 0.7rem;
  font-weight: 800;
  padding: 0.2rem 0.5rem;
  color: var(--text-color);
  cursor: pointer;
  opacity: 0.5;
  border-radius: 4px;
}

.edit-mode-toggle button.active {
  background: var(--primary-color);
  color: white;
  opacity: 1;
}

.markdown-preview {
  flex: 1;
  background: rgba(0,0,0,0.2);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 1.5rem;
  text-align: left;
  overflow-y: auto;
  color: var(--text-color);
  font-size: 0.95rem;
  line-height: 1.6;
}

.markdown-preview :deep(h1), .markdown-preview :deep(h2), .markdown-preview :deep(h3) {
  margin-top: 1rem;
  margin-bottom: 0.5rem;
  color: var(--secondary-color);
}

.markdown-preview :deep(code) {
  background: rgba(255,255,255,0.1);
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  font-family: monospace;
}

.markdown-preview :deep(pre) {
  background: rgba(0,0,0,0.3);
  padding: 1rem;
  border-radius: 8px;
  overflow-x: auto;
  margin: 1rem 0;
}

.markdown-preview :deep(ul), .markdown-preview :deep(ol) {
  padding-left: 1.5rem;
  margin: 1rem 0;
}

.item-row.is-dragging {
  opacity: 0.3;
  transform: scale(0.95);
  border: 1px dashed var(--primary-color);
}

.item-row.is-drop-target {
  background: rgba(var(--primary-rgb), 0.1) !important;
  border: 2px solid var(--primary-color) !important;
  transform: scale(1.02);
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

.header-actions {
  display: flex;
  gap: 0.8rem;
  align-items: center;
}

.secondary-btn {
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-color);
  border: 1px solid var(--border-color);
  padding: 0.5rem 1rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.2s;
}

.secondary-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--secondary-color);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.close-modal {
  background: transparent;
  border: none;
  color: var(--secondary-color);
  font-size: 1.2rem;
  cursor: pointer;
  opacity: 0.5;
  transition: opacity 0.2s;
}

.close-modal:hover {
  opacity: 1;
}

.input-with-voice {
  position: relative;
  display: flex;
  width: 100%;
}

.input-with-voice input, 
.input-with-voice textarea {
  width: 100%;
  padding-right: 3rem !important;
}

.voice-btn {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: none;
  font-size: 1.2rem;
  cursor: pointer;
  filter: grayscale(1);
  transition: all 0.3s;
  z-index: 5;
}

.textarea-voice {
  top: 1.5rem;
  transform: none;
}

.voice-btn.recording {
  filter: none;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% { transform: translateY(-50%) scale(1); opacity: 1; }
  50% { transform: translateY(-50%) scale(1.3); opacity: 0.7; }
  100% { transform: translateY(-50%) scale(1); opacity: 1; }
}

.textarea-voice.recording {
  animation: pulse-textarea 1.5s infinite;
}

@keyframes pulse-textarea {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.7; }
  100% { transform: scale(1); opacity: 1; }
}

.empty-hint {
  text-align: center;
  padding: 4rem 2rem;
  opacity: 0.3;
  font-style: italic;
  font-size: 1.1rem;
}
</style>
