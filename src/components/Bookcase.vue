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
    
    // Merge folders from books and localStorage
    const saved = localStorage.getItem('kb_custom_folders');
    const localFolders = saved ? JSON.parse(saved) : [];
    
    const combined = new Set([...localFolders]);
    books.value.forEach(b => {
      if (b.folder) combined.add(b.folder);
    });
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
  
  activeBook.value = book;
  activeNote.value = null;
  bookNotes.value = [];
  epubError.value = null;
  
  try {
    bookNotes.value = await apiService.getBookNotes(book.id);
    if (bookNotes.value.length > 0) {
      activeNote.value = { ...bookNotes.value[0] };
    } else {
      createNewNote();
    }
    
    if (isEPUB(book) && viewMode.value !== 'notes') {
      isEpubLoading.value = true;
      nextTick(() => initEpubReader());
    }
  } catch (err) {
    console.error('Failed to fetch notes:', err);
  }
};

const cleanupEpub = () => {
  if (epubRendition.value) {
    try { epubRendition.value.destroy(); } catch(e){}
    epubRendition.value = null;
  }
  if (epubBook.value) {
    try { epubBook.value.destroy(); } catch(e){}
    epubBook.value = null;
  }
  isEpubLoading.value = false;
};

const initEpubReader = async () => {
  if (!activeBook.value || !isEPUB(activeBook.value) || !epubViewerRef.value) return;
  
  // @ts-ignore
  if (typeof ePub === 'undefined') {
    epubError.value = "Library still loading...";
    setTimeout(initEpubReader, 1000);
    return;
  }

  const url = getFileUrl(activeBook.value);
  try {
    // Fetch as BLOB to avoid relative path 403 errors in epub.js
    const response = await fetch(url);
    if (!response.ok) throw new Error("File fetch failed");
    const buffer = await response.arrayBuffer();

    // @ts-ignore
    epubBook.value = ePub(buffer);
    
    epubRendition.value = epubBook.value.renderTo(epubViewerRef.value, {
      width: "100%",
      height: "100%",
      flow: "scrolled",
      manager: "default"
    });
    
    epubRendition.value.display().then(() => {
      isEpubLoading.value = false;
      epubError.value = null;
      // Dark theme for epub.js
      epubRendition.value.themes.register("dark", {
         "body": { "color": "#cbd5e1 !important", "background": "#1e293b !important" },
         "p": { "color": "#cbd5e1 !important" }
      });
      epubRendition.value.themes.select("dark");
    }).catch((err: any) => {
      console.error('EPUB display error:', err);
      epubError.value = "Failed to render book content. The file might be corrupted.";
      isEpubLoading.value = false;
    });
  } catch (e) {
    console.error('Reader init error:', e);
    epubError.value = "Incompatible file format or Network error.";
    isEpubLoading.value = false;
  }
};

const createNewNote = () => {
  activeNote.value = {
    id: 'temp-' + Date.now(),
    title: 'New Study Note',
    content: '',
    noteType: 'markdown'
  };
};

const selectNote = (note: any) => {
  activeNote.value = { ...note };
};

const saveCurrentNote = async () => {
  if (!activeBook.value || !activeNote.value) return;
  isSaving.value = true;
  try {
    if (activeNote.value.id.startsWith('temp-')) {
      const res = await apiService.addBookNote(activeBook.value.id, {
        title: activeNote.value.title,
        content: activeNote.value.content,
        noteType: activeNote.value.noteType
      });
      activeNote.value.id = res.id;
    } else {
      await apiService.updateBookNote(activeNote.value.id, {
        title: activeNote.value.title,
        content: activeNote.value.content,
        noteType: activeNote.value.noteType
      });
    }
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
  } catch (err) {
    alert('Failed to save note');
  } finally {
    isSaving.value = false;
  }
};

const deleteNote = async (id: string) => {
  if (id.startsWith('temp-')) {
    activeNote.value = null;
    return;
  }
  if (!confirm('Delete this note?')) return;
  try {
    await apiService.removeBookNote(id);
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
    if (bookNotes.value.length > 0) {
      selectNote(bookNotes.value[0]);
    } else {
      createNewNote();
    }
  } catch (err) {
    alert('Delete failed');
  }
};

// --- Store Discovery ---
const searchAvailable = async () => {
  isSearching.value = true;
  try {
    availableResources.value = await apiService.getAvailableBooks(searchQuery.value);
  } catch (err) {
    console.error('Search failed:', err);
  } finally {
    isSearching.value = false;
  }
};

const importBook = async (res: any) => {
  try {
    await apiService.addBookToBookcase({
      storeId: res.id,
      title: res.title || res.caption || 'Untitled Book',
      category: res.mediaType?.toUpperCase() || 'DOCUMENT'
    });
    showAddModal.value = false;
    fetchBookcase();
  } catch (err) {
    alert('Import failed');
  }
};

const removeBookStatus = async (id: string) => {
  if (!confirm('Unlink this book from your library?')) return;
  try {
    await apiService.removeBook(id);
    activeBook.value = null;
    fetchBookcase();
  } catch (err) {
    alert('Unlink failed');
  }
};

// --- Folder & Drag and Drop Logic ---
const folders = computed(() => {
  const groups: Record<string, any[]> = { 'Uncategorized': [] };
  customFolders.value.forEach(f => { if (!groups[f]) groups[f] = []; });
  
  if (books.value) {
    books.value.forEach(book => {
      if (searchTerm.value && !book.title?.toLowerCase().includes(searchTerm.value.toLowerCase())) return;
      const f = book.folder || 'Uncategorized';
      if (!groups[f]) groups[f] = [];
      groups[f].push(book);
    });
  }
  return groups;
});

const onDragStart = (event: DragEvent, bookId: string) => {
  if (event.dataTransfer) {
    event.dataTransfer.setData('bookId', bookId);
    event.dataTransfer.effectAllowed = 'move';
    setTimeout(() => {
       const el = document.querySelector(`[data-id="${bookId}"]`);
       if (el) (el as HTMLElement).style.opacity = '0.3';
    }, 0);
  }
};

const onDragEnd = (bookId: string) => {
  const el = document.querySelector(`[data-id="${bookId}"]`);
  if (el) (el as HTMLElement).style.opacity = '1';
  dragOverFolder.value = null;
};

const onDropIntoFolder = async (event: DragEvent, folderName: string) => {
  event.preventDefault();
  dragOverFolder.value = null;
  const bookId = event.dataTransfer?.getData('bookId');
  if (!bookId) return;

  const targetFolder = folderName === 'Uncategorized' ? '' : folderName;
  try {
    await apiService.updateBookFolder(bookId, targetFolder);
    // Sync state immediately
    const b = books.value.find(bk => bk.id === bookId);
    if (b) b.folder = targetFolder;
  } catch (err) {
    console.error('Book move failed:', err);
    fetchBookcase(); 
  }
};

const createFolder = () => {
  const name = newFolderName.value.trim();
  if (!name) return;
  if (!customFolders.value.includes(name)) {
    customFolders.value.push(name);
  }
  newFolderName.value = '';
};

// Persistence for empty folders
watch(customFolders, (newVal) => {
  localStorage.setItem('kb_custom_folders', JSON.stringify(newVal));
}, { deep: true });

// --- Helpers ---
const getFileUrl = (book: any) => {
  if (!book || !book.storeId) return '';
  return `${import.meta.env.VITE_API_URL}/api/storehouse/file/${book.storeId}`;
};

const isPDF = (book: any) => {
  if (!book) return false;
  const title = (book.title || '').toLowerCase();
  const category = (book.category || '').toLowerCase();
  return category.includes('pdf') || title.endsWith('.pdf');
};

const isEPUB = (book: any) => {
  if (!book) return false;
  const title = (book.title || '').toLowerCase();
  return title.endsWith('.epub');
};

watch(viewMode, (newVal) => {
  if (newVal !== 'notes' && isEPUB(activeBook.value)) {
    cleanupEpub();
    isEpubLoading.value = true;
    nextTick(() => initEpubReader());
  }
});

onMounted(() => {
  fetchBookcase();
  
  if (!document.getElementById('epub-js')) {
    const script = document.createElement('script');
    script.id = 'epub-js';
    script.src = 'https://unpkg.com/epubjs/dist/epub.min.js';
    script.async = true;
    document.head.appendChild(script);
    
    const jszip = document.createElement('script');
    jszip.src = 'https://unpkg.com/jszip/dist/jszip.min.js';
    jszip.async = true;
    document.head.appendChild(jszip);
  }
});
</script>

<template>
  <div class="bookcase-v2">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="search-wrap"><input v-model="searchTerm" placeholder="Filter research..." /></div>
        <button @click="showAddModal = true; searchAvailable()" class="icon-btn add-book-btn"><span>+</span></button>
      </div>

      <div class="folder-list">
        <div v-if="isLoading" class="list-loader">Syncing library...</div>
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
              draggable="true" @dragstart="onDragStart($event, book.id)" @dragend="onDragEnd(book.id)"
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
        <div class="new-folder-area"><input v-model="newFolderName" placeholder="+ New Folder" @keyup.enter="createFolder" /></div>
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
          <button @click="removeBookStatus(activeBook.id)" class="icon-btn delete-item-btn">🗑️</button>
        </div>
      </header>

      <div class="ws-body" :class="'mode-' + viewMode">
        <div v-if="viewMode !== 'notes'" class="preview-pane">
          <div v-if="isPDF(activeBook)" class="pdf-viewer">
            <iframe :src="getFileUrl(activeBook)" frameborder="0"></iframe>
          </div>
          <div v-else-if="isEPUB(activeBook)" class="epub-viewer-container">
             <div ref="epubViewerRef" class="epub-canvas"></div>
             
             <div v-if="isEpubLoading" class="reader-overlay">
                <div class="spinner"></div>
                <span>Parsing book content...</span>
             </div>
             
             <div v-if="epubError" class="reader-overlay error">
                <div class="icon">⚠️</div>
                <span>{{ epubError }}</span>
                <a :href="getFileUrl(activeBook)" target="_blank" class="download-link">Open Original File</a>
             </div>

             <div v-if="epubRendition" class="epub-nav">
                <button @click="epubRendition.prev()" class="nav-btn">⬅️</button>
                <button @click="epubRendition.next()" class="nav-btn">➡️</button>
             </div>
          </div>
          <div v-else class="placeholder-viewer">
             <div class="msg"><div class="icon">🔍</div><p>Online preview for <b>{{ activeBook.category }}</b> soon.</p></div>
          </div>
        </div>

        <div v-if="viewMode !== 'preview'" class="notes-pane">
          <div class="note-tabs">
            <div 
              v-for="note in bookNotes" :key="note.id" 
              class="note-tab" :class="{ active: activeNote?.id === note.id }"
              @click="selectNote(note)"
            >{{ note.title }}</div>
            <button @click="createNewNote" class="new-note-tab">+ New</button>
          </div>
          <div v-if="activeNote" class="note-editor-container">
            <div class="editor-header">
              <input v-model="activeNote.title" class="note-title-input" placeholder="Title..." />
              <div class="editor-actions">
                <button @click="activeNote.noteType = activeNote.noteType === 'markdown' ? 'txt' : 'markdown'" class="toggle-btn">{{ activeNote.noteType === 'markdown' ? 'MD' : 'TXT' }}</button>
                <button @click="saveCurrentNote" :disabled="isSaving" class="save-btn">{{ isSaving ? '...' : 'Save' }}</button>
                <button @click="pinToDesk('note', activeNote.id)" class="pin-note-btn">📌</button>
                <button v-if="!activeNote.id.startsWith('temp-')" @click="deleteNote(activeNote.id)" class="delete-note-btn">🗑️</button>
              </div>
            </div>
            <div class="editor-main">
              <textarea v-model="activeNote.content" class="note-textarea" placeholder="Start typing..." />
              <div v-if="activeNote.noteType === 'markdown'" class="note-preview markdown-body" v-html="marked(activeNote.content || '')" />
            </div>
          </div>
        </div>
      </div>
    </main>
    <main v-else class="workspace empty-ws">
       <div class="welcome"><h1>Research Workspace</h1><p>Select a volume to begin your study session.</p></div>
    </main>

    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content add-book-modal">
        <div class="modal-header"><h2>Library Store</h2><button @click="showAddModal = false" class="close-btn">&times;</button></div>
        <div class="search-bar"><input v-model="searchQuery" placeholder="Search..." @input="searchAvailable" /></div>
        <div class="available-list">
          <div v-for="res in availableResources" :key="res.id" class="resource-item" @click="importBook(res)">
            <span class="res-type">[{{ res.mediaType }}]</span><span class="res-title">{{ res.title || res.caption || 'Untitled' }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-v2 { display: flex; height: 100vh; background: #0f172a; color: #e2e8f0; font-family: 'Outfit', sans-serif;}
.sidebar { width: 300px; background: rgba(0, 0, 0, 0.25); border-right: 1px solid rgba(255, 255, 255, 0.05); display: flex; flex-direction: column; }
.sidebar-header { padding: 1.25rem; display: flex; gap: 0.5rem; border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.search-wrap { flex: 1; }
.search-wrap input { width: 100%; padding: 0.5rem 0.75rem; background: rgba(255, 255, 255, 0.05); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 6px; color: white; }
.add-book-btn { width: 34px; background: #d97706; border: none; border-radius: 6px; color: white; font-weight: bold; cursor: pointer; }
.folder-list { flex: 1; overflow-y: auto; padding: 0.5rem; }
.folder-group { margin-bottom: 0.5rem; border: 1px solid transparent; border-radius: 8px; transition: all 0.2s; }
.folder-group.drop-target { background: rgba(217, 119, 6, 0.1); border-color: rgba(217, 119, 6, 0.5); transform: translateY(-2px); }
.folder-header { padding: 0.6rem 0.75rem; display: flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; font-weight: 600; color: #94a3b8; }
.folder-header .count { margin-left: auto; font-size: 0.7rem; background: rgba(255, 255, 255, 0.05); padding: 0.1rem 0.4rem; border-radius: 10px; }
.book-item { padding: 0.75rem; border-radius: 8px; display: flex; gap: 0.75rem; cursor: pointer; transition: all 0.2s; margin-bottom: 2px; }
.book-item:hover { background: rgba(255, 255, 255, 0.05); }
.book-item.active { background: rgba(217, 119, 6, 0.15); border: 1px solid rgba(217, 119, 6, 0.3); }
.item-title { font-size: 0.85rem; font-weight: 500; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; overflow: hidden; text-align: left;}
.item-meta { font-size: 0.65rem; opacity: 0.4; text-transform: uppercase; margin-top: 2px; text-align: left;}
.new-folder-area { padding: 1rem; }
.new-folder-area input { width: 100%; padding: 0.5rem 1rem; background: rgba(255, 255, 255, 0.03); border: 1px dashed rgba(255, 255, 255, 0.15); border-radius: 10px; color: #94a3b8; outline: none; }
.workspace { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.ws-header { height: 64px; padding: 0 1.5rem; display: flex; align-items: center; justify-content: space-between; background: rgba(255, 255, 255, 0.02); border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.active-book-info { display: flex; align-items: center; gap: 1rem; max-width: 60%; }
.active-book-info h2 { font-size: 1rem; margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.badge { background: rgba(217, 119, 6, 0.2); color: #fbbf24; padding: 0.2rem 0.6rem; border-radius: 4px; font-size: 0.7rem; font-weight: bold; }
.mode-toggle { display: flex; background: rgba(255, 255, 255, 0.05); border-radius: 8px; padding: 2px; }
.mode-toggle button { padding: 0.4rem 0.9rem; border: none; background: transparent; color: #94a3b8; font-size: 0.8rem; cursor: pointer; border-radius: 6px; }
.mode-toggle button.active { background: rgba(255, 255, 255, 0.1); color: white; }
.ws-body { flex: 1; display: flex; overflow: hidden; }
.preview-pane { flex: 1.2; border-right: 1px solid rgba(255, 255, 255, 0.05); background: #1e293b; position: relative; }
.pdf-viewer iframe { width: 100%; height: 100%; position: absolute; }
.epub-viewer-container { width: 100%; height: 100%; position: relative; background: #1e293b; }
.epub-canvas { width: 100%; height: 100%; }
.reader-overlay { position: absolute; top: 0; left: 0; width: 100%; height: 100%; background: rgba(30, 41, 59, 0.9); display: flex; flex-direction: column; gap: 1rem; align-items: center; justify-content: center; z-index: 50; }
.reader-overlay.error { color: #ef4444; }
.epub-nav { position: absolute; bottom: 1.5rem; left: 50%; transform: translateX(-50%); display: flex; gap: 1rem; z-index: 100; }
.nav-btn { padding: 0.5rem 1.2rem; background: rgba(15, 23, 42, 0.9); color: white; border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 20px; cursor: pointer; }
.notes-pane { flex: 1; display: flex; flex-direction: column; background: #0f172a; }
.note-tabs { padding: 0.75rem 1rem 0; display: flex; gap: 0.25rem; border-bottom: 1px solid rgba(255, 255, 255, 0.05); overflow-x: auto; }
.note-tab { padding: 0.5rem 1rem; font-size: 0.85rem; background: rgba(255, 255, 255, 0.02); border-radius: 6px 6px 0 0; cursor: pointer; white-space: nowrap; border: 1px solid transparent; border-bottom: none; }
.note-tab.active { background: #1e293b; color: white; border-color: rgba(255, 255, 255, 0.05); }
.new-note-tab { padding: 0.5rem 1rem; font-size: 0.8rem; background: transparent; border: none; color: #fbbf24; cursor: pointer; }
.editor-header { padding: 1rem; display: flex; justify-content: space-between; align-items: center; gap: 1rem; border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.note-title-input { flex: 1; background: transparent; border: none; font-size: 1.1rem; font-weight: 600; color: white; outline: none; }
.editor-actions { display: flex; gap: 0.5rem; }
.save-btn { padding: 0.4rem 1.25rem; background: #d97706; border: none; color: white; border-radius: 6px; font-size: 0.85rem; font-weight: bold; cursor: pointer; }
.pin-note-btn { background: rgba(59, 130, 246, 0.1); border: 1px solid rgba(59, 130, 246, 0.2); color: #60a5fa; border-radius: 6px; padding: 0.4rem 0.8rem; cursor: pointer; }
.delete-note-btn { background: rgba(239, 68, 68, 0.1); border: none; color: #ef4444; border-radius: 6px; padding: 0.4rem 0.6rem; cursor: pointer; }
.editor-main { flex: 1; display: flex; overflow: hidden; }
.note-textarea { flex: 1; padding: 1.5rem; background: transparent; border: none; color: #cbd5e1; font-family: 'JetBrains Mono', monospace; font-size: 1rem; line-height: 1.6; resize: none; outline: none; border-right: 1px solid rgba(255, 255, 255, 0.03); }
.note-preview { flex: 1; padding: 1.5rem; overflow-y: auto; text-align: left; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.85); z-index: 2000; display: flex; justify-content: center; align-items: center; }
.modal-content { background: #1e293b; border: 1px solid rgba(255,255,255,0.1); border-radius: 12px; width: 600px; max-height: 80vh; display: flex; flex-direction: column; overflow: hidden; }
.modal-header { padding: 1.25rem; border-bottom: 1px solid rgba(255,255,255,0.05); display: flex; justify-content: space-between; align-items: center; }
.empty-ws { flex: 1; display: flex; align-items: center; justify-content: center; background: #0f172a; text-align: center; }
.download-link { margin-top: 1rem; color: #fbbf24; text-decoration: underline; font-size: 0.85rem; }
.spinner { width: 30px; height: 30px; border: 3px solid rgba(255,255,255,0.1); border-top-color: #fbbf24; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
