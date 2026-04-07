<script setup lang="ts">
import { ref, onMounted, computed, nextTick } from 'vue';
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
const searchQuery = ref('');
const showAddModal = ref(false);
const isSaving = ref(false);
const viewMode = ref<'preview' | 'mixed' | 'notes'>('mixed');
const searchTerm = ref(''); 
const newFolderName = ref('');
const customFolders = ref<string[]>([]);
const dragOverFolder = ref<string | null>(null);
const collapsedFolders = ref<Set<string>>(new Set());

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
    
    const saved = localStorage.getItem('kb_custom_folders');
    const localFolders = saved ? JSON.parse(saved) : [];
    const combined = new Set([...localFolders]);
    books.value.forEach(b => { if (b.folder) combined.add(b.folder); });
    customFolders.value = Array.from(combined);

    if (books.value.length > 0 && !activeBook.value) {
      selectBook(books.value[0]);
    }
  } catch (err) { console.error('Sync issue:', err); } finally { isLoading.value = false; }
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
    } else {
      createNewNote();
    }
    
    if (isEPUB(activeBook.value) && viewMode.value !== 'notes') {
      isEpubLoading.value = true;
      nextTick(() => initEpubReader());
    }
  } catch (err) { console.error('Asset retrieval failed:', err); }
};

const cleanupEpub = () => {
  if (epubRendition.value) { try { epubRendition.value.destroy(); } catch(e){} epubRendition.value = null; }
  if (epubBook.value) { try { epubBook.value.destroy(); } catch(e){} epubBook.value = null; }
  isEpubLoading.value = false;
};

const initEpubReader = async () => {
  if (!activeBook.value || !isEPUB(activeBook.value) || !epubViewerRef.value) return;
  // @ts-ignore
  if (typeof ePub === 'undefined') { epubError.value = "Engine loading..."; setTimeout(initEpubReader, 1000); return; }

  const url = getFileUrl(activeBook.value);
  try {
    const response = await fetch(url, { cache: 'force-cache' });
    if (!response.ok) throw new Error(`HTTP ${response.status}: Retrieval failed`);
    const buffer = await response.arrayBuffer();
    // @ts-ignore
    epubBook.value = ePub(buffer);
    epubRendition.value = epubBook.value.renderTo(epubViewerRef.value, { width: "100%", height: "100%", flow: "paginated", manager: "default" });
    await epubRendition.value.display();
    isEpubLoading.value = false;
    epubError.value = null;
    epubRendition.value.themes.register("dark", { "body": { "color": "#cbd5e1 !important", "background": "#1e293b !important" }, "p": { "color": "#cbd5e1 !important" } });
    epubRendition.value.themes.select("dark");
  } catch (e: any) { epubError.value = e.message || "Conflict."; isEpubLoading.value = false; }
};

const createNewNote = () => {
  activeNote.value = { id: 'temp-' + Date.now(), title: 'Volume Abstract', content: '', noteType: 'both' };
};

const toggleNoteType = () => {
  if (!activeNote.value) return;
  const current = activeNote.value.noteType;
  if (current === 'txt') activeNote.value.noteType = 'both';
  else if (current === 'both') activeNote.value.noteType = 'md';
  else activeNote.value.noteType = 'txt';
};

const saveCurrentNote = async () => {
  if (!activeBook.value || !activeNote.value) return;
  isSaving.value = true;
  try {
    const payload = { title: activeNote.value.title, content: activeNote.value.content, 
                     noteType: activeNote.value.noteType === 'txt' ? 'txt' : 'markdown' };
    if (activeNote.value.id.startsWith('temp-')) {
      const res = await apiService.addBookNote(activeBook.value.id, payload);
      activeNote.value.id = res.id;
    } else { await apiService.updateBookNote(activeNote.value.id, payload); }
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
  } catch (err) { alert('Commit aborted'); } finally { isSaving.value = false; }
};

const deleteNote = async (id: string) => {
  if (id.startsWith('temp-')) { activeNote.value = null; return; }
  if (!confirm('Purge note?')) return;
  try {
    await apiService.removeBookNote(id);
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
    if (bookNotes.value.length > 0) activeNote.value = { ...bookNotes.value[0] };
    else createNewNote();
  } catch (err) { alert('Purge failed'); }
};

const removeBookStatus = async (id: string) => {
  if (!confirm('Unlink intellectual asset?')) return;
  try { await apiService.removeBook(id); activeBook.value = null; fetchBookcase(); } catch (err) { alert('Unlink error'); }
};

const importBook = async (res: any) => {
  try {
    await apiService.addBookToBookcase({ storeId: res.id, title: res.title || res.caption || 'Source', category: res.mediaType?.toUpperCase() || 'DOCUMENT' });
    showAddModal.value = false; fetchBookcase();
  } catch (err) { alert('Import disrupted'); }
};

const folders = computed(() => {
  const groups: Record<string, any[]> = { 'Uncategorized': [] };
  customFolders.value.forEach(f => { if (!groups[f]) groups[f] = []; });
  books.value.forEach(book => {
    if (searchTerm.value && !book.title?.toLowerCase().includes(searchTerm.value.toLowerCase())) return;
    const f = book.folder || 'Uncategorized';
    if (!groups[f]) groups[f] = []; groups[f].push(book);
  });
  return groups;
});

const onDragStart = (event: DragEvent, bookId: string) => {
  if (event.dataTransfer) { event.dataTransfer.setData('bookId', bookId); event.dataTransfer.effectAllowed = 'move'; }
};

const onDropIntoFolder = async (event: DragEvent, folderName: string) => {
  event.preventDefault(); dragOverFolder.value = null;
  const bookId = event.dataTransfer?.getData('bookId');
  if (!bookId) return;
  const targetFolder = folderName === 'Uncategorized' ? '' : folderName;
  try { await apiService.updateBookFolder(bookId, targetFolder); await fetchBookcase(); } catch (err) { fetchBookcase(); }
};

const getFileUrl = (book: any) => { if (!book || !book.storeId) return ''; return `${import.meta.env.VITE_API_URL}/api/storehouse/file/${book.storeId}`; };
const isEPUB = (book: any) => { if (!book) return false; return (book.title || '').toLowerCase().endsWith('.epub'); };

onMounted(() => {
  fetchBookcase();
  if (!document.getElementById('epub-js')) {
    const s = document.createElement('script'); s.id = 'epub-js'; s.src = 'https://unpkg.com/epubjs/dist/epub.min.js'; s.async = true; document.head.appendChild(s);
    const j = document.createElement('script'); j.src = 'https://unpkg.com/jszip/dist/jszip.min.js'; j.async = true; document.head.appendChild(j);
  }
});
</script>

<template>
  <div class="bookcase-v2">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="search-wrap"><input v-model="searchTerm" placeholder="Filter Research..." /></div>
        <button @click="showAddModal = true" class="icon-btn add-btn">+</button>
      </div>

      <div class="folder-list">
        <div v-for="(folderBooks, folderName) in folders" :key="folderName" class="folder-group" 
             :class="{ 'drop-target': dragOverFolder === folderName }"
             @dragover.prevent="dragOverFolder = String(folderName)" @dragleave="dragOverFolder = null" @drop="onDropIntoFolder($event, String(folderName))">
          <div class="folder-header" @click="collapsedFolders.has(String(folderName)) ? collapsedFolders.delete(String(folderName)) : collapsedFolders.add(String(folderName))">
            <span class="fold-arrow">{{ collapsedFolders.has(String(folderName)) ? '▶' : '▼' }}</span>
            <span class="folder-icon">📂</span>
            <span class="folder-name">{{ folderName }}</span>
            <span class="count">{{ folderBooks.length }}</span>
          </div>
          <div v-show="!collapsedFolders.has(String(folderName))" class="folder-content">
            <div v-for="book in folderBooks" :key="book.id" class="book-item" :class="{ active: activeBook?.id === book.id }"
                 draggable="true" @dragstart="onDragStart($event, book.id)" @click="selectBook(book)">
              <div class="item-icon">🔖</div>
              <div class="item-info"><div class="item-title">{{ book.title }}</div><div class="item-meta">{{ book.category }}</div></div>
            </div>
          </div>
        </div>
        <div class="new-folder-area"><input v-model="newFolderName" placeholder="+ Cluster" @keyup.enter="customFolders.push(newFolderName); newFolderName=''" /></div>
      </div>
    </aside>

    <main v-if="activeBook" class="workspace">
      <header class="ws-header">
        <div class="active-book-info"><h2>{{ activeBook.title }}</h2><span class="badge">{{ activeBook.category }}</span></div>
        <div class="ws-controls">
          <button @click="viewMode = 'preview'" :class="{ active: viewMode === 'preview' }">📖 Read</button>
          <button @click="viewMode = 'mixed'" :class="{ active: viewMode === 'mixed' }">🌗 Split</button>
          <button @click="viewMode = 'notes'" :class="{ active: viewMode === 'notes' }">📝 Note</button>
          <button @click="removeBookStatus(activeBook.id)" class="rm-btn">🗑️</button>
        </div>
      </header>

      <div class="ws-body" :class="'mode-' + viewMode">
        <div v-if="viewMode !== 'notes'" class="preview-pane">
          <iframe v-if="activeBook.title?.toLowerCase().endsWith('.pdf')" :src="getFileUrl(activeBook)" frameborder="0"></iframe>
          <div v-else-if="isEPUB(activeBook)" class="epub-viewer-container">
             <div ref="epubViewerRef" class="epub-canvas"></div>
             <div v-if="isEpubLoading" class="reader-overlay"><div class="spinner"></div></div>
             <div v-if="epubError" class="reader-overlay error"><span>{{ epubError }}</span><button @click="selectBook(activeBook)">Retry</button></div>
             <div v-if="epubRendition" class="epub-nav">
                <button @click="epubRendition.prev()" class="nav-btn">⬅️</button>
                <button @click="epubRendition.next()" class="nav-btn">➡️</button>
             </div>
          </div>
          <div v-else class="placeholder">Preview Not Active</div>
        </div>

        <div v-if="viewMode !== 'preview'" class="notes-pane">
          <div class="note-tabs">
            <div v-for="note in bookNotes" :key="note.id" class="note-tab" :class="{ active: activeNote?.id === note.id }" @click="activeNote = { ...note }">{{ note.title }}</div>
            <button @click="createNewNote" class="new-tab">+ New</button>
          </div>
          <div v-if="activeNote" class="note-editor">
            <div class="ed-nav">
              <input v-model="activeNote.title" />
              <button @click="toggleNoteType">{{ activeNote.noteType === 'both' ? 'TXT/MD' : activeNote.noteType?.toUpperCase() }}</button>
              <button @click="saveCurrentNote">COMMIT</button>
              <button @click="pinToDesk('note', activeNote.id)">📌</button>
              <button @click="deleteNote(activeNote.id)" class="del-btn-sub">🗑️</button>
            </div>
            <div class="ed-main" :class="'v-' + activeNote.noteType">
              <textarea v-if="activeNote.noteType !== 'md'" v-model="activeNote.content" />
              <div v-if="activeNote.noteType !== 'txt'" class="markdown-body" v-html="marked(activeNote.content || '')" />
            </div>
          </div>
        </div>
      </div>
    </main>
    <main v-else class="workspace empty">Deploy Intelligence.</main>

    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content">
        <header><h2>Library Discovery</h2></header>
        <div class="m-body">
          <input v-model="searchQuery" placeholder="Filter..." @input="apiService.getAvailableBooks(searchQuery).then(r => availableResources = r)" />
          <div class="r-list">
            <div v-for="res in availableResources" :key="res.id" @click="importBook(res)"><span>[{{ res.mediaType }}]</span>{{ res.title || res.caption }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-v2 { display: flex; height: 100vh; background: #111; color: #ccc; font-family: 'Outfit', sans-serif;}
.sidebar { width: 300px; background: #000; border-right: 1px solid #222; display: flex; flex-direction: column; }
.sidebar-header { padding: 1rem; display: flex; gap: 0.5rem; }
.search-wrap { flex: 1; }
.search-wrap input { width: 100%; padding: 0.5rem; background: #1a1a1a; border: 1px solid #333; border-radius: 4px; color: #fff; }
.add-btn { width: 34px; background: #d97706; border: none; color: #fff; border-radius: 4px; cursor: pointer; }
.folder-list { flex: 1; overflow-y: auto; padding: 0.5rem; }
.folder-header { padding: 0.75rem; display: flex; align-items: center; gap: 0.5rem; cursor: pointer; border-radius: 6px; }
.folder-header:hover { background: #1a1a1a; }
.fold-arrow { font-size: 0.6rem; width: 12px; }
.count { margin-left: auto; font-size: 0.7rem; color: #fbbf24; }
.book-item { padding: 0.6rem 0.6rem 0.6rem 2rem; display: flex; gap: 0.5rem; cursor: pointer; border-radius: 6px; margin-bottom: 2px; }
.book-item.active { background: #d9770611; border: 1px solid #d9770633; }
.item-title { font-size: 0.8rem; text-align: left; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; }
.item-meta { font-size: 0.6rem; opacity: 0.4; }
.workspace { flex: 1; display: flex; flex-direction: column; background: #000; }
.ws-header { height: 60px; padding: 0 1rem; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #222; }
.badge { font-size: 0.6rem; background: #fbbf2422; color: #fbbf24; padding: 2px 6px; border-radius: 4px; }
.ws-controls { display: flex; gap: 0.5rem; }
.ws-controls button { padding: 0.4rem 0.8rem; background: #1a1a1a; border: 1px solid #333; color: #999; border-radius: 6px; cursor: pointer; }
.ws-controls button.active { background: #d97706; color: #fff; border-color: #d97706; }
.rm-btn { background: #ef444411 !important; color: #ef4444 !important; border: none !important; }
.ws-body { flex: 1; display: flex; overflow: hidden; }
.preview-pane { flex: 1.3; position: relative; background: #1a1a1a; }
.preview-pane iframe { width: 100%; height: 100%; }
.epub-viewer-container { width: 100%; height: 100%; position: relative; }
.epub-canvas { width: 100%; height: 100%; }
.epub-nav { position: absolute; bottom: 2rem; left: 50%; transform: translateX(-50%); display: flex; gap: 1rem; z-index: 1000; }
.nav-btn { background: #d97706; color: #fff; border: none; padding: 0.5rem 1rem; border-radius: 30px; cursor: pointer; box-shadow: 0 4px 12px #000; }
.notes-pane { flex: 1; border-left: 1px solid #222; display: flex; flex-direction: column; }
.note-tabs { display: flex; gap: 2px; padding: 0.5rem 1rem 0; border-bottom: 1px solid #222; overflow-x: auto; }
.note-tab { padding: 0.5rem 1rem; font-size: 0.75rem; background: #1a1a1a; border-radius: 4px 4px 0 0; cursor: pointer; }
.note-tab.active { background: #d97706; color: #fff; }
.note-editor { flex: 1; display: flex; flex-direction: column; }
.ed-nav { padding: 0.75rem 1rem; display: flex; gap: 0.5rem; align-items: center; }
.ed-nav input { flex: 1; background: transparent; border: none; color: #fff; font-size: 1rem; font-weight: bold; outline: none; }
.ed-nav button { padding: 0.3rem 0.6rem; font-size: 0.7rem; background: #333; border: 1px solid #444; color: #ccc; border-radius: 4px; cursor: pointer; }
.del-btn-sub { color: #ef4444 !important; }
.ed-main { flex: 1; display: flex; overflow: hidden; }
.ed-main textarea { flex: 1; background: transparent; border: none; padding: 1.5rem; color: #ccc; font-family: monospace; font-size: 0.9rem; line-height: 1.6; resize: none; outline: none; border-right: 1px solid #222; }
.markdown-body { flex: 1; padding: 1.5rem; overflow-y: auto; text-align: left; }
.v-txt .markdown-body { display: none; }
.v-md textarea { display: none; }
.spinner { width: 24px; height: 24px; border: 2px solid #333; border-top-color: #fbbf24; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: #000e; z-index: 2000; display: flex; align-items: center; justify-content: center; }
.modal-content { background: #1a1a1a; width: 400px; border-radius: 12px; border: 1px solid #333; overflow: hidden; }
.m-body { padding: 1rem; }
.m-body input { width: 100%; padding: 0.5rem; background: #000; border: 1px solid #333; border-radius: 4px; color: #fff; margin-bottom: 1rem; }
.r-list { max-height: 300px; overflow-y: auto; }
.r-list div { padding: 0.6rem; border-bottom: 1px solid #333; font-size: 0.8rem; cursor: pointer; text-align: left; }
</style>
