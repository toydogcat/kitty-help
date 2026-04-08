<script setup lang="ts">
import { ref, onMounted, computed, watch, onUnmounted } from 'vue';
import { liveQuery } from 'dexie';
import { db } from '../services/localDb';
import { apiService, socket } from '../services/api';
import { syncService } from '../services/syncService';
import { usePin } from '../composables/usePin';
import SnippetTreeNode from './SnippetTreeNode.vue';

const { pinToDesk } = usePin();
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
const editMode = ref<'txt' | 'md'>('md');

let snippetsSub: any = null;
let allSnippetsSub: any = null;

const fetchData = async () => {
    // 🏠 EverSync: Refresh in background
    syncService.refreshSnippets(currentFolderId.value === 'root' ? null : currentFolderId.value).catch(() => {});
    syncService.refreshSnippets(null, true).catch(() => {});
};

onMounted(() => {
  // 🛰️ Live Reactive Queries
  snippetsSub = liveQuery(() => 
    db.snippets.where('parentId').equals(currentFolderId.value).sortBy('sortOrder')
  ).subscribe(val => {
    snippets.value = val as any;
    loading.value = false;
  });

  allSnippetsSub = liveQuery(() => 
    db.snippets.toArray()
  ).subscribe(val => {
    allSnippets.value = val as any;
  });

  fetchData();
  socket.on('snippetsUpdate', fetchData);
});

onUnmounted(() => {
  if (snippetsSub) snippetsSub.unsubscribe();
  if (allSnippetsSub) allSnippetsSub.unsubscribe();
  socket.off('snippetsUpdate', fetchData);
});

watch(() => props.userId, (newVal) => {
  if (newVal) fetchData();
});

// Watch currentFolderId and RE-SUBSCRIBE
watch(currentFolderId, (newId) => {
  if (snippetsSub) snippetsSub.unsubscribe();
  snippetsSub = liveQuery(() => 
    db.snippets.where('parentId').equals(newId).sortBy('sortOrder')
  ).subscribe(val => {
    snippets.value = val as any;
  });
  fetchData();
});

const treeData = computed(() => {
  const map: any = {};
  const roots: any[] = [];
  // Sort by sortOrder first
  const sortedRaw = [...allSnippets.value].sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));
  const items = sortedRaw.map(item => ({ ...item, children: [], isOpen: false }));
  
  items.forEach(item => { map[item.id] = item; });
  items.forEach(item => {
    if (item.parentId && item.parentId !== 'root' && map[item.parentId]) {
      map[item.parentId].children.push(item);
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
  editMode.value = 'txt';
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
  editMode.value = 'md'; 
  showAddModal.value = true;
};

const saveItem = async () => {
  if (!newItemName.value.trim()) return;
  try {
    if (isEditing.value && editingId.value) {
      await syncService.updateSnippet(editingId.value, {
        name: newItemName.value,
        content: newItemContent.value,
        parentId: editingParentId.value
      });
    } else {
      await syncService.createSnippet({
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
      await syncService.deleteSnippet(id);
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
const handleDragEnd = () => { draggedItem.value = null; dropTargetId.value = null; isDropOverRoot.value = false; dropPosition.value = 'inside'; };

const dropPosition = ref<'inside' | 'before' | 'after'>('inside');

const handleDragOver = (e: DragEvent, item: any) => {
  e.preventDefault();
  if (draggedItem.value?.id === item.id) return;
  
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const y = e.clientY - rect.top;
  const threshold = rect.height / 3;

  if (y < threshold) {
    dropPosition.value = 'before';
  } else if (y > rect.height - threshold) {
    dropPosition.value = 'after';
  } else {
    if (item.isFolder) {
      dropPosition.value = 'inside';
    } else {
      dropPosition.value = y < rect.height / 2 ? 'before' : 'after';
    }
  }
  dropTargetId.value = item.id;
};

const handleDragLeave = (item: any) => { if (dropTargetId.value === item.id) dropTargetId.value = null; };

const handleDrop = async (targetItem: any | 'root') => {
  if (!draggedItem.value) return;
  const targetId = targetItem === 'root' ? null : targetItem.id;
  if (draggedItem.value.id === targetId) { handleDragEnd(); return; }

  // 🛡️ SECONDARY SAFETY: Prevent moving into non-folder via handleDrop
  if (targetId) {
    const target = allSnippets.value.find(s => s.id === targetId);
    if (target && !target.isFolder) {
        handleReorder({ targetNode: target, position: 'after' });
        return;
    }
  }

  try {
    loading.value = true;
    const cleanData = {
        name: draggedItem.value.name,
        content: draggedItem.value.content,
        parentId: targetId,
        isFolder: draggedItem.value.isFolder,
        sortOrder: draggedItem.value.sortOrder
    };
    await syncService.updateSnippet(draggedItem.value.id, cleanData);
  } catch (err) {
    console.error("Drop failed:", err);
  } finally {
    handleDragEnd();
    await fetchData();
  }
};

const handleReorder = async (data: { targetNode: any, position: 'before' | 'after' }) => {
    if (!draggedItem.value) return;
    const target = data.targetNode;
    if (draggedItem.value.id === target.id) return;

    try {
        loading.value = true;
        const parentId = target.parentId;
        const siblings = allSnippets.value
            .filter(s => s.parentId === parentId && s.id !== draggedItem.value.id)
            .sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));

        // Sanitize dragged item
        const cleanDragged = {
            id: draggedItem.value.id,
            name: draggedItem.value.name,
            content: draggedItem.value.content,
            isFolder: draggedItem.value.isFolder,
            parentId: parentId
        };
        
        siblings.splice(insertIdx, 0, cleanDragged as any);

        const updates = [];
        for (let i = 0; i < siblings.length; i++) {
            const node = siblings[i];
            if (node.sortOrder !== i || node.id === cleanDragged.id) {
                const updateData = {
                    name: node.name,
                    content: node.content,
                    isFolder: node.isFolder,
                    parentId: node.parentId,
                    sortOrder: i
                };
                updates.push(syncService.updateSnippet(node.id, updateData));
            }
        }

        if (updates.length > 0) {
            await Promise.all(updates);
        }
        await fetchData();
    } catch (err) {
        console.error("Reorder failed:", err);
    } finally {
        handleDragEnd();
    }
};

const addToDesk = async (item: any) => {
  try {
    await pinToDesk('snippet', item.id);
    pinnedIds.value.add(item.id);
    setTimeout(() => pinnedIds.value.delete(item.id), 2000);
  } catch (err) {
    alert("Pinning failed");
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
            @drop-on-node="(data: any) => handleDrop(data.targetNode)"
            @drop-reorder="handleReorder"
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
            'is-drop-target': dropTargetId === item.id,
            'drop-before': dropTargetId === item.id && dropPosition === 'before',
            'drop-after': dropTargetId === item.id && dropPosition === 'after',
            'drop-inside': dropTargetId === item.id && dropPosition === 'inside'
          }"
          draggable="true"
          @dragstart="handleDragStart(item)"
          @dragover.prevent="handleDragOver($event, item)"
          @dragleave="handleDragLeave(item)"
          @drop="dropPosition === 'inside' ? handleDrop(item) : handleReorder({ targetNode: item, position: dropPosition })"
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
              <span class="snippet-name">
                📄 {{ item.name }}
                <span v-if="item.syncStatus === 'pending'" class="sync-badge" title="Pending Sync">⏳</span>
                <span v-if="item.syncStatus === 'error'" class="sync-badge error" title="Sync Failed">⚠️</span>
              </span>
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
      </div>
    </div>

    <!-- Modal for New/Edit Item -->
    <Teleport to="body">
      <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
        <div class="modal-card wide-editor" :class="{ 'is-full': isFullScreen }">
          <div class="modal-header">
            <h3>{{ isEditing ? 'Edit Item' : 'Create New Item' }}</h3>
            
            <!-- UNIFIED CONTROL CAPSULE -->
            <div class="unified-controls">
               <div class="mode-capsule" v-if="!newItemIsFolder">
                  <button :class="{ active: editMode === 'md' }" @click="editMode = 'md'">MD PREVIEW</button>
                  <button :class="{ active: editMode === 'txt' }" @click="editMode = 'txt'">TXT / EDIT</button>
               </div>
               <div class="action-set">
                  <button @click="isFullScreen = !isFullScreen" class="action-item" title="Maximize">
                    {{ isFullScreen ? '❐' : '⛶' }}
                  </button>
                  <button @click="showAddModal = false" class="action-item close" title="Close">✕</button>
               </div>
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

.item-row.drop-inside {
  background: rgba(var(--primary-rgb),0.2) !important;
  border: 1px dashed var(--primary-color);
}

.item-row.drop-before {
  border-top: 4px solid var(--primary-color);
}

.item-row.drop-after {
  border-bottom: 4px solid var(--primary-color);
}

.item-content { flex: 1; }
.snippet-name { font-weight: 700; color: #fff; }
.snippet-preview { font-size: 0.8rem; opacity: 0.5; margin-top: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 500px; }

.item-actions { display: flex; gap: 6px; opacity: 0; transition: opacity 0.2s; }
.item-row:hover .item-actions { opacity: 1; }
.item-actions button { background: rgba(255,255,255,0.1); border: none; border-radius: 6px; padding: 4px 10px; color: #fff; font-size: 0.75rem; cursor: pointer; }
.item-actions button:hover { background: var(--primary-color); }

.sync-badge { font-size: 0.8rem; margin-left: 0.5rem; opacity: 0.8; }
.sync-badge.error { color: #ff5555; opacity: 1; }

/* Unified Controls Styling */
.unified-controls { display: flex; align-items: center; gap: 1rem; }
.mode-capsule { display: flex; background: rgba(0,0,0,0.4); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.mode-capsule button { background: none; border: none; color: #fff; padding: 6px 14px; border-radius: 9px; font-size: 0.75rem; font-weight: 800; cursor: pointer; opacity: 0.4; transition: all 0.2s; }
.mode-capsule button.active { background: var(--primary-color); opacity: 1; box-shadow: 0 2px 8px rgba(var(--primary-rgb), 0.4); }

.action-set { display: flex; background: rgba(255,255,255,0.05); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.action-item { background: none; border: none; color: #fff; width: 34px; height: 34px; border-radius: 9px; font-size: 1.1rem; cursor: pointer; opacity: 0.6; transition: all 0.2s; display: flex; align-items: center; justify-content: center; }
.action-item:hover { background: rgba(255,255,255,0.1); opacity: 1; }
.action-item.close:hover { background: #e74c3c; color: #fff; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); backdrop-filter: blur(10px); display: flex; align-items: center; justify-content: center; z-index: 3001; }
.modal-card.wide-editor { width: 950px; max-width: 95vw; background: var(--card-bg); border-radius: 28px; border: 1px solid rgba(var(--primary-rgb), 0.3); display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 25px 60px rgba(0,0,0,0.6); }
.modal-card.is-full { width: 100vw; height: 100vh; border-radius: 0; border: none; }

.modal-header { padding: 1.2rem 2.5rem; display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.05); }
.modal-body { flex: 1; padding: 2.5rem; display: flex; flex-direction: column; gap: 1.8rem; }
.form-group.fill { flex: 1; }
input, textarea { background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); border-radius: 14px; padding: 1.2rem; color: #fff; font-size: 1rem; width: 100%; outline: none; }
textarea { height: 400px; resize: none; }

.md-preview-box { background: rgba(0,0,0,0.4); padding: 2.5rem; border-radius: 14px; border: 1px solid rgba(255,255,255,0.05); height: 400px; overflow-y: auto; color: #eee; line-height: 1.8; font-size: 1.1rem; }
.md-preview-box :deep(h1) { color: var(--primary-color); border-bottom: 1px solid rgba(255,255,255,0.1); padding-bottom: 8px; margin: 1.5rem 0 1rem; }

.modal-footer { padding: 1.5rem 2.8rem; display: flex; justify-content: flex-end; gap: 1.2rem; background: rgba(0,0,0,0.2); }
.save-btn { background: var(--primary-color); color: #fff; border-radius: 12px; font-weight: 800; padding: 0.9rem 3rem; border: none; cursor: pointer; transition: all 0.2s; }
.save-btn:hover { filter: brightness(1.1); transform: translateY(-2px); }

.mic-btn-float { position: absolute; right: 1.5rem; bottom: 1.5rem; background: rgba(var(--primary-rgb), 0.2); border: none; border-radius: 50%; width: 44px; height: 44px; cursor: pointer; }
.editor-row { position: relative; }

.custom-scrollbar::-webkit-scrollbar { width: 8px; height: 8px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
