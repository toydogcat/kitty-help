<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';
import { usePin } from '../composables/usePin';

const props = defineProps<{ userId: string; }>();
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

const fetchBookcase = async () => {
  isLoading.value = true;
  try {
    const data = await apiService.getBookcase();
    books.value = data || [];
    const saved = localStorage.getItem('kb_custom_folders');
    if (saved) customFolders.value = JSON.parse(saved);
    books.value.forEach(b => { if (b.folder && !customFolders.value.includes(b.folder)) customFolders.value.push(b.folder); });
    if (books.value.length > 0 && !activeBook.value) selectBook(books.value[0]);
  } catch (e) { console.error('Sync fail:', e); } finally { isLoading.value = false; }
};

const selectBook = async (book: any) => {
  cleanupEpub();
  activeBook.value = { ...book };
  activeNote.value = null;
  bookNotes.value = [];
  epubError.value = null;
  
  try {
    bookNotes.value = await apiService.getBookNotes(book.id);
    if (bookNotes.value.length > 0) activeNote.value = { ...bookNotes.value[0] };
    else createNewNote();
    
    if (isEPUB(activeBook.value) && viewMode.value !== 'notes') {
      isEpubLoading.value = true;
      nextTick(() => initEpubReader());
    }
  } catch (e) { console.error('Selection break:', e); }
};

const cleanupEpub = () => {
  if (epubRendition.value) { try { epubRendition.value.destroy(); } catch(e){} epubRendition.value = null; }
  if (epubBook.value) { try { epubBook.value.destroy(); } catch(e){} epubBook.value = null; }
  isEpubLoading.value = false;
};

const waitForLibs = () => {
  return new Promise((resolve, reject) => {
    let attempts = 0;
    const check = () => {
      // @ts-ignore
      if (typeof ePub !== 'undefined' && typeof JSZip !== 'undefined') resolve(true);
      else if (attempts > 50) reject(new Error("Core engines (EPub/JSZip) timeout."));
      else { attempts++; setTimeout(check, 200); }
    };
    check();
  });
};

const initEpubReader = async () => {
  if (!activeBook.value || !isEPUB(activeBook.value) || !epubViewerRef.value) return;
  
  try {
    await waitForLibs();
    const url = getFileUrl(activeBook.value);
    const response = await fetch(url, { cache: 'force-cache' });
    if (!response.ok) throw new Error("Cloud stream retrieval failed");
    const buffer = await response.arrayBuffer();

    // @ts-ignore
    epubBook.value = ePub(buffer);
    epubRendition.value = epubBook.value.renderTo(epubViewerRef.value, { 
       width: "100%", height: "100%", flow: "paginated", manager: "continuous"
    });
    
    // NUCLEAR FIX: Hook into content before it hits the iframe to strip scripts and fix fonts
    epubRendition.value.hooks.content.register((contents: any) => {
       // 1. Remove all scripts to satisfy the sandbox
       const doc = contents.document;
       const scripts = doc.querySelectorAll('script');
       scripts.forEach((s: any) => s.remove());
       
       // 2. Clear out bad CSS that references local files (res:///)
       const styles = doc.querySelectorAll('style, link[rel="stylesheet"]');
       styles.forEach((style: any) => {
          if (style.textContent?.includes('res://')) {
             style.textContent = style.textContent.replace(/url\(["']?res:\/\/[^)]+\)/g, 'none');
          }
       });

       // 3. Inject clean font styling
       return contents.addStylesheetRules({
          "body": { 
             "font-family": "'Outfit', system-ui, -apple-system, sans-serif !important",
             "color": "#cbd5e1 !important",
             "background": "transparent !important"
          }
       });
    });

    epubRendition.value.on("attached", () => {
       const frames = epubViewerRef.value?.querySelectorAll('iframe');
       frames?.forEach(f => {
          f.removeAttribute("sandbox");
          f.setAttribute("sandbox", "allow-same-origin"); // No allow-scripts needed since we stripped them
       });
    });

    await epubRendition.value.display();
    isEpubLoading.value = false;
    epubError.value = null;
  } catch (e: any) { 
    console.error('Reader Intel Error:', e);
    epubError.value = e.message || "Engine synchronization failure."; 
    isEpubLoading.value = false; 
  }
};

const createNewNote = () => {
  activeNote.value = { id: 'temp-' + Date.now(), title: 'Volume Abstract', content: '', noteType: 'both' };
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
    const payload = { title: activeNote.value.title, content: activeNote.value.content, 
                     noteType: activeNote.value.noteType === 'txt' ? 'txt' : 'markdown' };
    if (activeNote.value.id.startsWith('temp-')) {
      const res = await apiService.addBookNote(activeBook.value.id, payload);
      activeNote.value.id = res.id;
    } else { await apiService.updateBookNote(activeNote.value.id, payload); }
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
  } catch (e) { alert('Commit fail'); } finally { isSaving.value = false; }
};

const deleteNote = async (id: string) => {
  if (id.startsWith('temp-')) { activeNote.value = null; return; }
  if (!confirm('Eliminate data?')) return;
  try {
    await apiService.removeBookNote(id);
    bookNotes.value = await apiService.getBookNotes(activeBook.value.id);
    if (bookNotes.value.length > 0) activeNote.value = { ...bookNotes.value[0] };
    else createNewNote();
  } catch (e) { alert('Operation aborted'); }
};

const removeBookStatus = async (id: string) => {
  if (!confirm('Detach?')) return;
  try { await apiService.removeBook(id); activeBook.value = null; fetchBookcase(); } catch (e) { alert('Operation fail'); }
};

const importBook = async (res: any) => {
  try {
    await apiService.addBookToBookcase({ storeId: res.id, title: res.title || res.caption || 'Source', category: res.mediaType?.toUpperCase() || 'VOLUME' });
    showAddModal.value = false; fetchBookcase();
  } catch (e) { alert('Import error'); }
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
  try { await apiService.updateBookFolder(bookId, targetFolder); await fetchBookcase(); } catch (e) { fetchBookcase(); }
};

const getFileUrl = (book: any) => { if (!book || !book.storeId) return ''; return `${import.meta.env.VITE_API_URL}/api/storehouse/file/${book.storeId}`; };
const isEPUB = (book: any) => { if (!book) return false; return (book.title || '').toLowerCase().endsWith('.epub'); };

onMounted(() => {
  fetchBookcase();
  if (!document.getElementById('jszip-js')) {
    const j = document.createElement('script'); j.id = 'jszip-js'; j.src = 'https://unpkg.com/jszip/dist/jszip.min.js'; j.async = true; document.head.appendChild(j);
    const s = document.createElement('script'); s.id = 'epub-js'; s.src = 'https://unpkg.com/epubjs/dist/epub.min.js'; s.async = true; document.head.appendChild(s);
  }
});

watch(customFolders, (newVal) => { localStorage.setItem('kb_custom_folders', JSON.stringify(newVal)); }, { deep: true });
</script>

<template>
  <div class="bookcase-v2">
    <aside class="sidebar">
      <div class="sidebar-header"><input v-model="searchTerm" placeholder="Filter Research..." /><button @click="showAddModal = true" class="add-btn">+</button></div>
      <div class="folder-list">
        <div v-for="(folderBooks, folderName) in folders" :key="folderName" class="folder-group" 
             :class="{ 'drop-target': dragOverFolder === folderName }" @dragover.prevent="dragOverFolder = String(folderName)" @dragleave="dragOverFolder = null" @drop="onDropIntoFolder($event, String(folderName))">
          <div class="folder-header" @click="collapsedFolders.has(String(folderName)) ? collapsedFolders.delete(String(folderName)) : collapsedFolders.add(String(folderName))">
            <span class="fold-arrow">{{ collapsedFolders.has(String(folderName)) ? '▶' : '▼' }}</span>
            <span class="folder-name">{{ folderName }}</span>
            <span class="count">{{ folderBooks.length }}</span>
          </div>
          <div v-show="!collapsedFolders.has(String(folderName))" class="folder-content">
            <div v-for="book in folderBooks" :key="book.id" class="book-item" :class="{ active: activeBook?.id === book.id }" draggable="true" @dragstart="onDragStart($event, book.id)" @click="selectBook(book)">
              <div class="item-icon">🔖</div><div class="item-info"><div class="item-title">{{ book.title }}</div><div class="item-meta">{{ book.category }}</div></div>
            </div>
          </div>
        </div>
        <div class="new-folder-area"><input v-model="newFolderName" placeholder="+ Cluster" @keyup.enter="customFolders.push(newFolderName); newFolderName=''" /></div>
      </div>
    </aside>

    <main v-if="activeBook" class="workspace">
      <header class="ws-header">
        <div class="active-book-info"><h2>{{ activeBook.title }}</h2><span class="badge">{{ activeBook.category }}</span></div>
        <div class="tabs-nav">
          <button @click="viewMode = 'preview'" :class="{ active: viewMode === 'preview' }">📖 READ</button>
          <button @click="viewMode = 'mixed'" :class="{ active: viewMode === 'mixed' }">🌗 SPLIT</button>
          <button @click="viewMode = 'notes'" :class="{ active: viewMode === 'notes' }">📝 LOGS</button>
        </div>
        <button @click="removeBookStatus(activeBook.id)" class="detach-btn">🗑️</button>
      </header>

      <div class="ws-body" :class="'mode-' + viewMode">
        <div v-if="viewMode !== 'notes'" class="preview-pane">
          <iframe v-if="activeBook.title?.toLowerCase().endsWith('.pdf')" :src="getFileUrl(activeBook)" frameborder="0"></iframe>
          <div v-else-if="isEPUB(activeBook)" class="epub-reader">
             <div ref="epubViewerRef" class="epub-canvas"></div>
             <div v-if="isEpubLoading" class="loader"><div class="spin"></div></div>
             <div v-if="epubError" class="overlay error"><span>{{ epubError }}</span><button @click="selectBook(activeBook)">RETRY</button></div>
             <div v-if="epubRendition" class="reader-controls">
                <button @click="epubRendition.prev()" class="nav-btn">⬅️</button>
                <button @click="epubRendition.next()" class="nav-btn">➡️</button>
             </div>
          </div>
        </div>

        <div v-if="viewMode !== 'preview'" class="notes-pane">
          <div class="note-tabs">
            <div v-for="note in bookNotes" :key="note.id" class="note-tab" :class="{ active: activeNote?.id === note.id }" @click="activeNote = { ...note }">{{ note.title }}</div>
            <button @click="createNewNote" class="new-btn">+ NEW</button>
          </div>
          <div v-if="activeNote" class="editor">
            <div class="ed-toolbar">
              <input v-model="activeNote.title" class="title-in" />
              <div class="actions">
                <button @click="toggleNoteType" class="cycle-btn">{{ activeNote.noteType === 'both' ? 'SPLIT' : activeNote.noteType?.toUpperCase() }}</button>
                <button @click="saveCurrentNote" class="commit-btn">COMMIT</button>
                <button @click="pinToDesk('note', activeNote.id)" class="pin-btn">📌</button>
                <button @click="deleteNote(activeNote.id)" class="del-btn">🗑️</button>
              </div>
            </div>
            <div class="ed-body" :class="'v-' + activeNote.noteType">
              <textarea v-if="activeNote.noteType !== 'md'" v-model="activeNote.content" placeholder="Research logs..." />
              <div v-if="activeNote.noteType !== 'txt'" class="markdown-body" v-html="marked(activeNote.content || '')" />
            </div>
          </div>
        </div>
      </div>
    </main>
    <main v-else class="empty-workspace">DEPLOY SYSTEM STANDBY.</main>

    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal">
        <header>INTEL REPOSITORY</header>
        <div class="m-content">
          <input v-model="searchQuery" placeholder="Filter Sources..." @input="apiService.getAvailableBooks(searchQuery).then(r => availableResources = r)" />
          <div class="items">
            <div v-for="res in availableResources" :key="res.id" @click="importBook(res)"><span>[{{ res.mediaType }}]</span>{{ res.title || res.caption }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-v2 { display: flex; height: 100vh; background: #0a0c10; color: #a0a8b1; font-family: 'Outfit', sans-serif;}
.sidebar { width: 280px; background: #000; border-right: 1px solid #1a1e23; display: flex; flex-direction: column; }
.sidebar-header { padding: 1.25rem; display: flex; gap: 0.5rem; }
.sidebar-header input { flex:1; padding: 0.5rem; background: #11151a; border: 1px solid #222; border-radius: 4px; color: #fff; font-size: 0.8rem; }
.add-btn { width: 34px; background: #d97706; border: none; color: #fff; border-radius: 4px; cursor: pointer; font-weight: bold; }
.folder-list { flex: 1; overflow-y: auto; padding: 0.5rem; }
.folder-header { padding: 0.75rem; display: flex; align-items: center; gap: 0.5rem; cursor: pointer; border-radius: 6px; font-weight: 700; color: #e2e8f0; font-size: 0.85rem; }
.folder-header:hover { background: #11151a; }
.count { margin-left: auto; font-size: 0.7rem; color: #fbbf24; background: #fbbf2411; padding: 2px 6px; border-radius: 4px; }
.book-item { padding: 0.65rem 0.65rem 0.65rem 1.5rem; display: flex; gap: 0.6rem; cursor: pointer; border-radius: 6px; margin-bottom: 2px; }
.book-item.active { background: #d977061a; border: 1px solid #d9770633; color: #fff; }
.item-title { font-size: 0.8rem; text-align: left; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; }
.item-meta { font-size: 0.6rem; opacity: 0.4; }
.workspace { flex: 1; display: flex; flex-direction: column; }
.ws-header { height: 64px; padding: 0 1.25rem; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #1a1e23; background: #000; }
.badge { font-size: 0.6rem; background: #fbbf2422; color: #fbbf24; padding: 2px 6px; border-radius: 4px; font-weight: 800; }
.tabs-nav { display: flex; gap: 4px; background: #11151a; padding: 4px; border-radius: 8px; }
.tabs-nav button { padding: 0.4rem 1rem; border: none; background: transparent; color: #666; font-size: 0.75rem; font-weight: 700; border-radius: 6px; cursor: pointer; transition: all 0.2s; }
.tabs-nav button.active { background: #d97706; color: #fff; }
.ws-body { flex: 1; display: flex; overflow: hidden; }
.preview-pane { flex: 1.4; position: relative; background: #121519; border-right: 1px solid #1a1e23; }
.epub-reader { width:100%; height:100%; position:relative; overflow:hidden;}
.epub-canvas { width:100%; height:100%; min-height: 500px; }
.reader-controls { position: absolute; bottom: 2rem; left: 50%; transform: translateX(-50%); display: flex; gap: 1rem; z-index: 1000; }
.nav-btn { background: #d97706DD; backdrop-filter: blur(8px); color: #fff; border: 1px solid #fbbf2444; padding: 0.6rem 1.25rem; border-radius: 4px; cursor: pointer; font-size: 0.8rem; font-weight: 900; }
.notes-pane { flex: 1; display: flex; flex-direction: column; background: #0a0c10; }
.note-tabs { display: flex; padding: 0.6rem 1rem 0; border-bottom: 1px solid #1a1e23; overflow-x: auto; gap: 4px; }
.note-tab { padding: 0.5rem 1.25rem; font-size: 0.8rem; background: #11151a; border-radius: 6px 6px 0 0; cursor: pointer; color: #666; }
.note-tab.active { background: #1a1e23; color: #fbbf24; }
.ed-toolbar { padding: 1rem; display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #1a1e23; }
.title-in { flex: 1; background: transparent; border: none; color: #fff; font-size: 1.1rem; font-weight: 700; outline: none; }
.commit-btn { padding: 0.4rem 1rem; background: #d97706; border: none; color: #fff; border-radius: 4px; font-size: 0.75rem; font-weight: 900; cursor: pointer; }
.ed-body { flex: 1; display: flex; overflow: hidden; }
.ed-body textarea { flex: 1; background: transparent; border: none; padding: 1.5rem; color: #cbd5e1; font-family: monospace; font-size: 0.95rem; line-height: 1.7; resize: none; outline: none; border-right: 1px solid #1a1e23; }
.markdown-body { flex: 1; padding: 1.5rem; overflow-y: auto; text-align: left; }
.v-txt .markdown-body { display: none; }
.v-md textarea { display: none; }
.loader { position: absolute; top:0; left:0; width:100%; height:100%; background: #111; display: flex; align-items: center; justify-content: center; z-index: 50; }
.spin { width: 32px; height: 32px; border: 3px solid #333; border-top-color: #d97706; border-radius: 50%; animation: rot 0.8s linear infinite; }
@keyframes rot { to { transform: rotate(360deg); } }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: #000E; z-index: 2000; display: flex; align-items: center; justify-content: center; }
.modal { background: #11151a; width: 440px; border-radius: 12px; border: 1px solid #222; overflow: hidden; }
.modal header { padding: 1rem; background: #000; border-bottom: 1px solid #222; font-weight: 900; color: #fff; font-size: 0.8rem; letter-spacing: 1px; }
.m-content { padding: 1.5rem; }
.m-content input { width: 100%; padding: 0.75rem; background: #000; border: 1px solid #222; border-radius: 6px; color: #fff; margin-bottom: 1.5rem; }
.items { max-height: 350px; overflow-y: auto; }
.items div { padding: 0.75rem; border-bottom: 1px solid #1a1e23; cursor: pointer; font-size: 0.85rem; text-align: left; transition: all 0.2s; }
.items div:hover { background: #d9770611; color: #fbbf24; }
.items div span { color: #fbbf24; opacity: 0.5; margin-right: 0.5rem; font-size: 0.7rem; font-weight: bold; }
</style>
