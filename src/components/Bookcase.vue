<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';
import { usePin } from '../composables/usePin';

const props = defineProps<{
  userId: string;
}>();

const { pinToDesk } = usePin();

// --- Core Data ---
const books = ref<any[]>([]);
const availableResources = ref<any[]>([]);
const activeBook = ref<any>(null);
const bookNotes = ref<any[]>([]);
const activeNote = ref<any>(null);

// --- UI States ---
const isLoading = ref(false);
const isSearching = ref(false);
const searchQuery = ref('');
const showAddModal = ref(false);
const isSaving = ref(false);
const viewMode = ref<'preview' | 'mixed' | 'notes'>('mixed');
const searchTerm = ref(''); 
const newFolderName = ref('');
const customFolders = ref<string[]>([]);
const dragOverFolder = ref<string | null>(null);

// EPUB Reader State
const epubRendition = ref<any>(null);
const epubBook = ref<any>(null);
const epubViewerRef = ref<HTMLElement | null>(null);
const isEpubLoading = ref(false);
const epubError = ref<string | null>(null);

// --- Library Management ---
const fetchBookcase = async () => {
  isLoading.value = true;
  try {
    const data = await apiService.getBookcase();
    books.value = data || [];
    
    // Sync custom folders
    const saved = localStorage.getItem('kb_custom_folders');
    const localFolders = saved ? JSON.parse(saved) : [];
    const combined = new Set([...localFolders]);
    books.value.forEach(b => { if (b.folder) combined.add(b.folder); });
    customFolders.value = Array.from(combined);

    if (books.value.length > 0 && !activeBook.value) {
      selectBook(books.value[0]);
    }
  } catch (err) {
    console.error('Failed to fetch bookcase:', err);
  } finally {
    isLoading.value = false;
  }
};

const selectBook = async (book: any) => {
  cleanupEpub();
  activeBook.value = { ...book };
  activeNote.value = null;
  bookNotes.value = [];
  epubError.value = null;
  
  try {
    bookNotes.value = await apiService.getBookNotes(book.id);
    if (bookNotes.value.length > 0) {
      activeNote.value = { ...bookNotes.value[0] };
      // Normalize noteType for cycle
      if (!['txt', 'both', 'md'].includes(activeNote.value.noteType)) {
          activeNote.value.noteType = activeNote.value.noteType === 'markdown' ? 'both' : 'txt';
      }
    } else {
      createNewNote();
    }
    
    if (isEPUB(activeBook.value) && viewMode.value !== 'notes') {
      isEpubLoading.value = true;
      nextTick(() => initEpubReader());
    }
  } catch (err) {
    console.error('Failed to fetch notes:', err);
  }
};

const cleanupEpub = () => {
  if (epubRendition.value) { try { epubRendition.value.destroy(); } catch(e){} epubRendition.value = null; }
  if (epubBook.value) { try { epubBook.value.destroy(); } catch(e){} epubBook.value = null; }
  isEpubLoading.value = false;
};

const initEpubReader = async () => {
  if (!activeBook.value || !isEPUB(activeBook.value) || !epubViewerRef.value) return;
  // @ts-ignore
  if (typeof ePub === 'undefined') { epubError.value = "Library loading..."; setTimeout(initEpubReader, 1000); return; }

  const url = getFileUrl(activeBook.value);
  try {
    const response = await fetch(url, { cache: 'force-cache' });
    if (!response.ok) throw new Error(`HTTP ${response.status}: Fetch failure`);
    const buffer = await response.arrayBuffer();
    // @ts-ignore
    epubBook.value = ePub(buffer);
    epubRendition.value = epubBook.value.renderTo(epubViewerRef.value, { width: "100%", height: "100%", flow: "scrolled", manager: "default" });
    await epubRendition.value.display();
    isEpubLoading.value = false;
    epubError.value = null;
    epubRendition.value.themes.register("dark", {
       "body": { "color": "#cbd5e1 !important", "background": "#1e293b !important" },
       "p": { "color": "#cbd5e1 !important" }
    });
    epubRendition.value.themes.select("dark");
  } catch (e: any) { epubError.value = e.message || "Engine failure."; isEpubLoading.value = false; }
};

const createNewNote = () => {
  activeNote.value = {
    id: 'temp-' + Date.now(),
    title: 'New Study Note',
    content: '',
    noteType: 'both' 
  };
};

const toggleNoteType = () => {
  if (!activeNote.value) return;
  const current = activeNote.value.noteType;
  if (current === 'txt' || current === 'markdown') activeNote.value.noteType = 'both';
  else if (current === 'both') activeNote.value.noteType = 'md';
  else activeNote.value.noteType = 'txt';
};

const saveCurrentNote = async () => {
  if (!activeBook.value || !activeNote.value) return;
  isSaving.value = true;
  try {
    const payload = {
       title: activeNote.value.title,
       content: activeNote.value.content,
       noteType: activeNote.value.noteType === 'txt' ? 'txt' : 'markdown'
    };
    if (activeNote.value.id.startsWith('temp-')) {
      const res = await apiService.addBookNote(activeBook.value.id, payload);
      activeNote.value.id = res.id;
    } else {
      await apiService.updateBookNote(activeNote.value.id, payload);
    }
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
  } catch (err) { alert('Save failed'); } finally { isSaving.value = false; }
};

const deleteNote = async (id: string) => {
  if (id.startsWith('temp-')) { activeNote.value = null; return; }
  if (!confirm('Delete note?')) return;
  try {
    await apiService.removeBookNote(id);
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
    if (bookNotes.value.length > 0) activeNote.value = { ...bookNotes.value[0] };
    else createNewNote();
  } catch (err) { alert('Delete failure'); }
};

const removeBookStatus = async (id: string) => {
  if (!confirm('Remove from library?')) return;
  try {
    await apiService.removeBook(id);
    activeBook.value = null;
    fetchBookcase();
  } catch (err) { alert('Remove failed'); }
};

const importBook = async (res: any) => {
  try {
    await apiService.addBookToBookcase({ storeId: res.id, title: res.title || res.caption || 'Untitled', category: res.mediaType?.toUpperCase() || 'DOCUMENT' });
    showAddModal.value = false;
    fetchBookcase();
  } catch (err) { alert('Import failed'); }
};

// --- Folder Logic ---
const folders = computed(() => {
  const groups: Record<string, any[]> = { 'Uncategorized': [] };
  customFolders.value.forEach(f => { if (!groups[f]) groups[f] = []; });
  
  books.value.forEach(book => {
    if (searchTerm.value && !book.title?.toLowerCase().includes(searchTerm.value.toLowerCase())) return;
    const f = book.folder || 'Uncategorized';
    if (!groups[f]) groups[f] = [];
    groups[f].push(book);
  });
  return groups;
});

const onDragStart = (event: DragEvent, bookId: string) => {
  if (event.dataTransfer) {
    event.dataTransfer.setData('bookId', bookId);
    event.dataTransfer.effectAllowed = 'move';
  }
};

const onDropIntoFolder = async (event: DragEvent, folderName: string) => {
  event.preventDefault();
  dragOverFolder.value = null;
  const bookId = event.dataTransfer?.getData('bookId');
  if (!bookId) return;
  const targetFolder = folderName === 'Uncategorized' ? '' : folderName;
  try {
    await apiService.updateBookFolder(bookId, targetFolder);
    await fetchBookcase();
  } catch (err) { fetchBookcase(); }
};

const createFolder = () => {
  const name = newFolderName.value.trim();
  if (name && !customFolders.value.includes(name)) customFolders.value.push(name);
  newFolderName.value = '';
};

watch(customFolders, (newVal) => { localStorage.setItem('kb_custom_folders', JSON.stringify(newVal)); }, { deep: true });

const getFileUrl = (book: any) => { if (!book || !book.storeId) return ''; return `${import.meta.env.VITE_API_URL}/api/storehouse/file/${book.storeId}`; };
const isEPUB = (book: any) => { if (!book) return false; return (book.title || '').toLowerCase().endsWith('.epub'); };

onMounted(() => {
  fetchBookcase();
  if (!document.getElementById('epub-js')) {
    const script = document.createElement('script'); script.id = 'epub-js'; script.src = 'https://unpkg.com/epubjs/dist/epub.min.js'; script.async = true; document.head.appendChild(script);
    const jszip = document.createElement('script'); jszip.src = 'https://unpkg.com/jszip/dist/jszip.min.js'; jszip.async = true; document.head.appendChild(jszip);
  }
});
</script>

<template>
  <div class="bookcase-v2">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="search-wrap"><input v-model="searchTerm" placeholder="Search volumes..." /></div>
        <button @click="showAddModal = true; isSearching = true" class="icon-btn add-btn"><span>+</span></button>
      </div>

      <div class="folder-list">
        <div 
          v-for="(folderBooks, folderName) in folders" :key="folderName"
          class="folder-group" :class="{ 'drop-target': dragOverFolder === folderName }"
          @dragover.prevent="dragOverFolder = folderName" @dragleave="dragOverFolder = null"
          @drop="onDropIntoFolder($event, folderName)"
        >
          <div class="folder-header">
            <span class="folder-icon">📂</span>
            <span class="folder-name">{{ folderName }}</span>
            <span class="count">{{ folderBooks.length }}</span>
          </div>
          <div class="folder-content">
            <div 
              v-for="book in folderBooks" :key="book.id" :data-id="book.id"
              class="book-item" :class="{ active: activeBook?.id === book.id }"
              draggable="true" @dragstart="onDragStart($event, book.id)" @dragend="dragOverFolder = null"
              @click="selectBook(book)"
            >
              <div class="item-icon">🔖</div>
              <div class="item-info">
                <div class="item-title">{{ book.title }}</div>
                <div class="item-meta">{{ book.category }}</div>
              </div>
            </div>
          </div>
        </div>
        <div class="new-folder-area"><input v-model="newFolderName" placeholder="+ New Cluster" @keyup.enter="createFolder" /></div>
      </div>
    </aside>

    <main v-if="activeBook" class="workspace">
      <header class="ws-header">
        <div class="active-book-info"><h2>{{ activeBook.title }}</h2><span class="badge">{{ activeBook.category }}</span></div>
        <div class="ws-controls">
          <div class="mode-toggle">
            <button @click="viewMode = 'preview'" :class="{ active: viewMode === 'preview' }">📖 Read</button>
            <button @click="viewMode = 'mixed'" :class="{ active: viewMode === 'mixed' }">🌗 Split</button>
            <button @click="viewMode = 'notes'" :class="{ active: viewMode === 'notes' }">📝 Notes</button>
          </div>
          <button @click="removeBookStatus(activeBook.id)" class="icon-btn delete-btn">🗑️</button>
        </div>
      </header>

      <div class="ws-body" :class="'mode-' + viewMode">
        <div v-if="viewMode !== 'notes'" class="preview-pane">
          <div v-if="activeBook.title?.toLowerCase().endsWith('.pdf')" class="pdf-viewer"><iframe :src="getFileUrl(activeBook)" frameborder="0"></iframe></div>
          <div v-else-if="isEPUB(activeBook)" class="epub-viewer-container">
             <div ref="epubViewerRef" class="epub-canvas"></div>
             <div v-if="isEpubLoading" class="reader-overlay"><div class="spinner"></div><span>Parsing...</span></div>
             <div v-if="epubError" class="reader-overlay error"><span>{{ epubError }}</span><button @click="selectBook(activeBook)" class="retry-btn">Retry</button></div>
             <div v-if="epubRendition" class="epub-nav"><button @click="epubRendition.prev()" class="nav-btn">⬅️</button><button @click="epubRendition.next()" class="nav-btn">➡️</button></div>
          </div>
          <div v-else class="placeholder-viewer"><div class="msg"><p>Format not supported for preview.</p></div></div>
        </div>

        <div v-if="viewMode !== 'preview'" class="notes-pane">
          <div class="note-tabs">
            <div v-for="note in bookNotes" :key="note.id" class="note-tab" :class="{ active: activeNote?.id === note.id }" @click="activeNote = { ...note }">{{ note.title }}</div>
            <button @click="createNewNote" class="new-note-tab">+ New Record</button>
          </div>
          <div v-if="activeNote" class="note-editor-container">
            <div class="editor-header">
              <input v-model="activeNote.title" class="note-title-input" />
              <div class="editor-actions">
                <button @click="toggleNoteType" class="toggle-btn-cycle">
                    {{ activeNote.noteType === 'both' ? 'TXT/MD' : activeNote.noteType?.toUpperCase() }}
                </button>
                <button @click="saveCurrentNote" :disabled="isSaving" class="save-btn">COMMIT</button>
                <button @click="pinToDesk('note', activeNote.id)" class="pin-note-btn">📌</button>
                <button @click="deleteNote(activeNote.id)" class="delete-btn-sub">🗑️</button>
              </div>
            </div>
            <div class="editor-main" :class="'view-' + activeNote.noteType">
              <textarea v-if="activeNote.noteType !== 'md'" v-model="activeNote.content" class="note-textarea" placeholder="Research logs..." />
              <div v-if="activeNote.noteType !== 'txt'" class="note-preview markdown-body" v-html="marked(activeNote.content || '')" />
            </div>
          </div>
        </div>
      </div>
    </main>
    <main v-else class="workspace empty-ws"><h1>Ready for Study Session</h1></main>

    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content">
        <div class="modal-header"><h2>Source Explorer</h2><button @click="showAddModal = false" class="close-btn">&times;</button></div>
        <div class="search-bar"><input v-model="searchQuery" placeholder="Filter sources..." @input="apiService.getAvailableBooks(searchQuery).then(r => availableResources = r)" /></div>
        <div class="available-list">
          <div v-for="res in availableResources" :key="res.id" class="resource-item" @click="importBook(res)"><span class="res-type">[{{ res.mediaType }}]</span>{{ res.title || res.caption }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-v2 { display: flex; height: 100vh; background: #0f172a; color: #e2e8f0; font-family: 'Outfit', sans-serif;}
.sidebar { width: 280px; background: rgba(0, 0, 0, 0.25); border-right: 1px solid rgba(255, 255, 255, 0.05); display: flex; flex-direction: column; }
.sidebar-header { padding: 1.25rem; display: flex; gap: 0.5rem; border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.search-wrap { flex: 1; }
.search-wrap input { width: 100%; padding: 0.5rem 0.75rem; background: rgba(255, 255, 255, 0.05); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 6px; color: white; }
.add-btn { width: 34px; background: #d97706; border: none; border-radius: 6px; color: white; cursor: pointer; }
.folder-list { flex: 1; overflow-y: auto; padding: 0.5rem; }
.folder-group { margin-bottom: 0.5rem; border: 1px solid transparent; border-radius: 8px; transition: all 0.2s; }
.folder-group.drop-target { background: rgba(217, 119, 6, 0.1); border-color: rgba(217, 119, 6, 0.5); }
.folder-header { padding: 0.6rem 0.75rem; display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; color: #94a3b8; }
.folder-header .count { margin-left: auto; font-size: 0.65rem; background: rgba(255, 255, 255, 0.05); padding: 0.1rem 0.4rem; border-radius: 10px; }
.book-item { padding: 0.65rem; border-radius: 8px; display: flex; gap: 0.75rem; cursor: pointer; }
.book-item:hover { background: rgba(255, 255, 255, 0.05); }
.book-item.active { background: rgba(217, 119, 6, 0.15); border: 1px solid rgba(217, 119, 6, 0.2); }
.item-title { font-size: 0.8rem; font-weight: 500; text-align: left; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; }
.item-meta { font-size: 0.6rem; opacity: 0.4; text-transform: uppercase; margin-top: 2px; text-align: left; }
.workspace { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.ws-header { height: 60px; padding: 0 1.5rem; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.active-book-info h2 { font-size: 0.9rem; margin: 0; }
.badge { background: rgba(217, 119, 6, 0.1); color: #fbbf24; padding: 0.2rem 0.5rem; border-radius: 4px; font-size: 0.6rem; font-weight: 900; }
.ws-body { flex: 1; display: flex; overflow: hidden; }
.preview-pane { flex: 1.3; border-right: 1px solid rgba(255, 255, 255, 0.03); background: #1e293b; position: relative; }
.pdf-viewer iframe { width: 100%; height: 100%; position: absolute; }
.epub-viewer-container { width: 100%; height: 100%; position: relative; }
.epub-canvas { width: 100%; height: 100%; }
.reader-overlay { position: absolute; top:0; left:0; width:100%; height:100%; background: #1e293b; display: flex; flex-direction: column; align-items: center; justify-content: center; z-index: 50; }
.notes-pane { flex: 1; display: flex; flex-direction: column; background: #0f172a; }
.note-tabs { padding: 0.75rem 1rem 0; display: flex; gap: 0.25rem; border-bottom: 1px solid rgba(255, 255, 255, 0.03); overflow-x: auto; }
.note-tab { padding: 0.5rem 1rem; font-size: 0.75rem; background: rgba(255, 255, 255, 0.02); border-radius: 6px 6px 0 0; cursor: pointer; white-space: nowrap; }
.note-tab.active { background: #1e293b; color: #fbbf24; }
.note-editor-container { flex: 1; display: flex; flex-direction: column; }
.editor-header { padding: 1rem; display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; }
.note-title-input { flex: 1; background: transparent; border: none; font-size: 1rem; font-weight: 700; color: white; outline: none; }
.toggle-btn-cycle { padding: 0.4rem 1rem; background: rgba(255, 255, 255, 0.08); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 6px; color: #fbbf24; font-size: 0.7rem; font-weight: 900; cursor: pointer; }
.save-btn { padding: 0.4rem 1.25rem; background: #d97706; border: none; color: white; border-radius: 6px; font-size: 0.75rem; font-weight: 900; cursor: pointer; }
.pin-note-btn { background: rgba(59, 130, 246, 0.1); border: 1px solid rgba(59, 130, 246, 0.2); border-radius: 6px; padding: 0.4rem; cursor: pointer; }
.delete-btn-sub { background: transparent; border: none; color: #ef4444; opacity: 0.5; padding: 0.4rem; cursor: pointer; }
.editor-main { flex: 1; display: flex; overflow: hidden; }
.view-txt .note-preview { display: none; }
.view-md .note-textarea { display: none; }
.note-textarea { flex: 1; padding: 1.5rem; background: transparent; border: none; color: #cbd5e1; font-family: 'JetBrains Mono', monospace; font-size: 0.9rem; line-height: 1.7; resize: none; outline: none; border-right: 1px solid rgba(255, 255, 255, 0.02); }
.note-preview { flex: 1; padding: 1.5rem; overflow-y: auto; text-align: left; }
.spinner { width: 24px; height: 24px; border: 3px solid rgba(255,255,255,0.05); border-top-color: #fbbf24; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.9); z-index: 2000; display: flex; justify-content: center; align-items: center; }
.modal-content { background: #1e293b; width: 500px; border-radius: 12px; overflow: hidden; display: flex; flex-direction: column; max-height: 70vh; }
.resource-item { padding: 0.8rem; border-bottom: 1px solid rgba(255,255,255,0.03); cursor: pointer; font-size: 0.8rem; text-align: left; }
</style>
