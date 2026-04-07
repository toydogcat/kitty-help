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

const savedFolder = localStorage.getItem('snippet_last_folder') || 'root';
const savedPath = localStorage.getItem('snippet_last_path');

const pathStack = ref<any[]>(savedPath ? JSON.parse(savedPath) : []);
const currentFolderId = ref<string | 'root'>(savedFolder);
const loading = ref(true);

watch([currentFolderId, pathStack], () => {
  localStorage.setItem('snippet_last_folder', currentFolderId.value);
  localStorage.setItem('snippet_last_path', JSON.stringify(pathStack.value));
}, { deep: true });

const showAddModal = ref(false);
const isEditing = ref(false);
const editingId = ref<string | null>(null);
const pinnedIds = ref<Set<string>>(new Set());
const newItemName = ref('');
const newItemContent = ref('');
const newItemIsFolder = ref(false);
const isFullScreen = ref(false);
const draggedItem = ref<any>(null);
const dropTargetId = ref<string | null>(null);
const isDropOverRoot = ref(false);
const editMode = ref<'txt' | 'md'>('md'); // DEFAULT TO MD

const fetchData = async () => {
  loading.value = true;
  try {
    allSnippets.value = await apiService.getSnippets(undefined, true);
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

watch(() => props.userId, (newVal) => {
  if (newVal) fetchData();
});

const treeData = computed(() => {
  const map: any = {};
  const roots: any[] = [];
  const items = allSnippets.value.map(item => ({ ...item, children: [], isOpen: false }));
  items.forEach(item => { map[item.id] = item; });
  items.forEach(item => {
    if (item.parentId) {
      if (map[item.parentId]) map[item.parentId].children.push(item);
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
    openEditModal(folder);
    return;
  } else {
    currentFolderId.value = folder.id;
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
  editMode.value = 'txt'; // New items start in text mode
  showAddModal.value = true;
};

const editingParentId = ref<string | null>(null);

const openEditModal = (item: any) => {
  isEditing.value = true;
  editingId.value = item.id;
  editingParentId.value = item.parentId;
  newItemName.value = item.name;
  newItemContent.value = item.content || '';
  newItemIsFolder.value = item.isFolder;
  isFullScreen.value = false; 
  editMode.value = 'md'; // PREVIEW BY DEFAULT
  showAddModal.value = true;
};

const saveItem = async () => {
  if (!newItemName.value.trim()) return;
  try {
    if (isEditing.value && editingId.value) {
      await apiService.updateSnippet(editingId.value, {
        name: newItemName.value,
        content: newItemContent.value,
        parentId: editingParentId.value
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
  if (confirm("Are you sure?")) {
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

const exportToJSON = () => {
  const dataStr = JSON.stringify(allSnippets.value, null, 2);
  const dataUri = 'data:application/json;charset=utf-8,'+ encodeURIComponent(dataStr);
  const exportFileDefaultName = `kitty_snippets_${new Date().toISOString().split('T')[0]}.json`;
  const linkElement = document.createElement('a');
  linkElement.setAttribute('href', dataUri);
  linkElement.setAttribute('download', exportFileDefaultName);
  linkElement.click();
};

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
        await apiService.createSnippet({
          parentId: null,
          name: item.name + ' (Imported)',
          content: item.content,
          isFolder: item.isFolder || item.is_folder
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
    alert("Speech recognition not supported");
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

const handleDragStart = (item: any) => { draggedItem.value = item; };
const handleDragEnd = () => { draggedItem.value = null; dropTargetId.value = null; isDropOverRoot.value = false; };
const handleDragOver = (item: any) => { if (draggedItem.value?.id === item.id) return; dropTargetId.value = item.id; };
const handleDragLeave = (item: any) => { if (dropTargetId.value === item.id) dropTargetId.value = null; };

const handleDrop = async (targetItem: any | 'root') => {
  if (!draggedItem.value) return;
  const targetId = targetItem === 'root' ? null : targetItem.id;
  const targetIsFolder = targetItem === 'root' ? true : targetItem.isFolder;
  if (draggedItem.value.id === targetId) { handleDragEnd(); return; }
  try {
    loading.value = true;
    if (targetIsFolder) {
      await apiService.updateSnippet(draggedItem.value.id, {
        name: draggedItem.value.name,
        content: draggedItem.value.content,
        parentId: targetId,
        sortOrder: 0
      });
    } else {
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
    handleDragEnd();
    await fetchData();
  }
};

const addToDesk = async (item: any) => {
  try {
    await apiService.addDeskItem({ type: 'snippet', refId: item.id, shelfId: null });
    pinnedIds.value.add(item.id);
    setTimeout(() => pinnedIds.value.delete(item.id), 2000);
  } catch (err) {
    console.error("Failed to pin:", err);
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
      <div class="tree-body custom-scrollbar">
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
          <input type="file" ref="fileInput" @change="importFromJSON" accept=".json" style="display: none" />
          <button @click="openAddModal" class="add-btn">+ New</button>
        </div>
      </div>

      <div v-if="loading" class="mini-loader">Loading snippets...</div>
      
      <div v-else class="explorer-body custom-scrollbar">
        <div v-if="pathStack.length > 0" @click="goBack" class="item-row back">
          📁 .. (Back)
        </div>
        
        <div 
          v-for="item in snippets" 
          :key="item.id" 
          class="item-row" 
          :class="{ 
            'is-dragging': draggedItem?.id === item.id,
            'is-drop-target': dropTargetId === item.id
          }"
          draggable="true"
          @dragstart="handleDragStart(item)"
          @dragover.prevent="handleDragOver(item)"
          @dragleave="handleDragLeave(item)"
          @drop="handleDrop(item)"
          @dragend="handleDragEnd"
          @click="item.isFolder ? enterFolder(item) : openEditModal(item)"
        >
          <div v-if="item.isFolder" class="item-content folder">
            <div class="folder-link">
              📁 <strong>{{ item.name }}</strong>
            </div>
          </div>
          <div v-else class="item-content snippet">
            <div class="snippet-info">
              <span class="snippet-name">📄 {{ item.name }}</span>
              <p v-if="item.content" class="snippet-preview">{{ item.content.substring(0, 80) }}...</p>
            </div>
          </div>

          <div class="item-actions">
            <button @click.stop="addToDesk(item)" class="pin-small" title="Add to Desk">📌</button>
            <button v-if="!item.isFolder" @click.stop="copyText(item.content)" class="copy-small">📋 Copy</button>
            <button @click.stop="openEditModal(item)" class="edit-small">✎</button>
            <button @click.stop="deleteItem(item.id)" class="del-small">✕</button>
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
        <div class="modal-card wide-editor" :class="{ 'is-full': isFullScreen }">
          <div class="modal-header">
            <h3>{{ isEditing ? 'Edit Item' : 'Create New Item' }}</h3>
            <div class="modal-controls">
              <div class="mode-switch" v-if="!newItemIsFolder">
                <button :class="{ active: editMode === 'md' }" @click="editMode = 'md'">MD PREVIEW</button>
                <button :class="{ active: editMode === 'txt' }" @click="editMode = 'txt'">TXT / EDIT</button>
              </div>
              <button @click="isFullScreen = !isFullScreen" class="ctrl-btn">{{ isFullScreen ? '❐' : '⛶' }}</button>
              <button @click="showAddModal = false" class="ctrl-btn">✕</button>
            </div>
          </div>
          
          <div class="modal-body custom-scrollbar">
            <div v-if="!isEditing" class="form-group row">
              <label>Type:</label>
              <div class="type-selection">
                <button @click="newItemIsFolder = false" :class="{ selected: !newItemIsFolder }">📄 Snippet</button>
                <button @click="newItemIsFolder = true" :class="{ selected: newItemIsFolder }">📁 Folder</button>
              </div>
            </div>

            <div class="form-group">
              <label>Name</label>
              <div class="input-row">
                <input v-model="newItemName" placeholder="e.g. My Notes" />
                <button @click="toggleVoice('name')" :class="{ active: isRecordingName }" class="mic-btn">🎙️</button>
              </div>
            </div>

            <div v-if="!newItemIsFolder" class="form-group fill">
              <label>Content (Markdown Supported)</label>
              <div v-if="editMode === 'md'" class="md-preview-box" v-html="marked.parse(newItemContent || '')"></div>
              <div v-else class="editor-row">
                <textarea v-model="newItemContent" placeholder="Paste your clipboard content here..."></textarea>
                <button @click="toggleVoice('content')" :class="{ active: isRecordingContent }" class="mic-btn-float">🎙️</button>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button @click="showAddModal = false" class="cancel-btn">Discard</button>
            <button @click="saveItem" class="save-btn">
              {{ isEditing ? '✅ Update Changes' : '✨ Create Now' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.snippet-explorer-container { display: flex; gap: 1rem; height: calc(100vh - 280px); min-height: 550px; }
.tree-sidebar { width: 260px; background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255,255,255,0.1); border-radius: 20px; padding: 1.5rem; display: flex; flex-direction: column; }
.main-explorer { flex: 1; display: flex; flex-direction: column; background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.1); border-radius: 20px; overflow: hidden; }

.explorer-header { display: flex; justify-content: space-between; align-items: center; padding: 1.5rem; background: rgba(255,255,255,0.02); border-bottom: 1px solid rgba(255,255,255,0.05); }
.header-actions { display: flex; gap: 0.8rem; }
.add-btn { background: var(--primary-color); color: white; border: none; padding: 0.5rem 1.5rem; border-radius: 10px; font-weight: 800; cursor: pointer; }

.explorer-body { flex: 1; padding: 1rem; overflow-y: auto; display: flex; flex-direction: column; gap: 0.5rem; }
.item-row { display: flex; align-items: center; padding: 1rem; border-radius: 12px; background: rgba(255,255,255,0.03); border: 1px solid transparent; cursor: pointer; transition: all 0.2s; position: relative; }
.item-row:hover { background: rgba(255,255,255,0.06); border-color: var(--primary-color); transform: translateX(5px); }

.item-content { flex: 1; }
.snippet-name { font-weight: 700; color: #fff; }
.snippet-preview { font-size: 0.8rem; opacity: 0.5; margin-top: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 500px; }

.item-actions { display: flex; gap: 6px; opacity: 0; transition: opacity 0.2s; }
.item-row:hover .item-actions { opacity: 1; }
.item-actions button { background: rgba(255,255,255,0.1); border: none; border-radius: 6px; padding: 4px 10px; color: #fff; font-size: 0.75rem; cursor: pointer; }
.item-actions button:hover { background: var(--primary-color); }

/* Unified Editor Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); backdrop-filter: blur(10px); display: flex; align-items: center; justify-content: center; z-index: 3000; }
.modal-card.wide-editor { width: 900px; max-width: 95vw; background: var(--card-bg); border-radius: 24px; border: 1px solid rgba(var(--primary-rgb), 0.3); display: flex; flex-direction: column; overflow: hidden; }
.modal-card.is-full { width: 100vw; height: 100vh; border-radius: 0; }

.modal-header { padding: 1.5rem 2.5rem; display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.05); }
.mode-switch { display: flex; background: rgba(0,0,0,0.3); padding: 4px; border-radius: 10px; gap: 4px; margin-right: 1.5rem; }
.mode-switch button { background: none; border: none; color: #fff; padding: 6px 14px; border-radius: 8px; font-size: 0.7rem; font-weight: 800; cursor: pointer; opacity: 0.4; }
.mode-switch button.active { background: var(--primary-color); opacity: 1; }

.modal-body { flex: 1; padding: 2.5rem; display: flex; flex-direction: column; gap: 1.8rem; }
.form-group { display: flex; flex-direction: column; gap: 0.6rem; }
.form-group.fill { flex: 1; }
.input-row { display: flex; gap: 10px; }
input, textarea { background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 12px; padding: 1rem; color: #fff; font-size: 1rem; width: 100%; outline: none; }
textarea { height: 350px; resize: none; }

.md-preview-box { background: rgba(0,0,0,0.4); padding: 2rem; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); height: 350px; overflow-y: auto; color: #eee; line-height: 1.7; }
.md-preview-box :deep(h1) { color: var(--primary-color); border-bottom: 1px solid rgba(255,255,255,0.1); padding-bottom: 8px; }

.modal-footer { padding: 1.5rem 2.5rem; display: flex; justify-content: flex-end; gap: 1rem; background: rgba(0,0,0,0.2); }
.save-btn { background: var(--primary-color); color: #fff; border-radius: 12px; font-weight: 800; padding: 0.8rem 2.5rem; border: none; cursor: pointer; }
.cancel-btn { background: transparent; color: #aaa; border: 1px solid #444; border-radius: 12px; padding: 0.8rem 1.5rem; cursor: pointer; }

.mic-btn-float { position: absolute; right: 1.5rem; bottom: 1.5rem; background: rgba(var(--primary-rgb), 0.2); border: none; border-radius: 50%; width: 40px; height: 40px; cursor: pointer; }
.editor-row { position: relative; }

.custom-scrollbar::-webkit-scrollbar { width: 6px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
