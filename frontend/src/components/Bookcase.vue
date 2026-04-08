<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, onUnmounted } from 'vue';
import { liveQuery } from 'dexie';
import { db } from '../services/localDb';
import { apiService } from '../services/api';
import { syncService } from '../services/syncService';
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
const dropTargetId = ref<string | null>(null);
const dropPosition = ref<'before' | 'after' | 'inside' | null>(null);
const draggedItem = ref<any>(null);
const collapsedFolders = ref<Set<string>>(new Set());

// EPUB Reader State
const epubRendition = ref<any>(null);
const epubBook = ref<any>(null);
const epubViewerRef = ref<HTMLElement | null>(null);
const isEpubLoading = ref(false);
const epubError = ref<string | null>(null);
let observer: MutationObserver | null = null;

let bookcaseSub: any = null;
let notesSub: any = null;

const fetchBookcase = async () => {
  isLoading.value = true;
  try {
    const saved = localStorage.getItem('kb_custom_folders');
    if (saved) customFolders.value = JSON.parse(saved);
    await syncService.refreshBookcase();
  } catch (e) {
    console.error('Bookcase background sync failed');
  } finally {
    isLoading.value = false;
  }
};

onMounted(() => {
  bookcaseSub = liveQuery(() => db.bookcase.toArray()).subscribe(val => {
     books.value = val;
     if (books.value.length > 0 && !activeBook.value) selectBook(books.value[0]);
  });
  
  fetchBookcase();
  if (!document.getElementById('jszip-js')) {
    const j = document.createElement('script'); j.id = 'jszip-js'; j.src = 'https://unpkg.com/jszip/dist/jszip.min.js'; j.async = true; document.head.appendChild(j);
    const s = document.createElement('script'); s.id = 'epub-js'; s.src = 'https://unpkg.com/epubjs/dist/epub.min.js'; s.async = true; document.head.appendChild(s);
  }
});

onUnmounted(() => {
  if (bookcaseSub) bookcaseSub.unsubscribe();
  if (notesSub) notesSub.unsubscribe();
  cleanupEpub();
});

const selectBook = async (book: any) => {
  cleanupEpub();
  activeBook.value = { ...book };
  activeNote.value = null;
  bookNotes.value = [];
  epubError.value = null;
  
  if (notesSub) notesSub.unsubscribe();
  notesSub = liveQuery(() => db.bookNotes.where('bookId').equals(book.id).toArray()).subscribe(val => {
     bookNotes.value = val;
     if (bookNotes.value.length > 0 && !activeNote.value) {
        activeNote.value = { ...bookNotes.value[0] };
     } else if (bookNotes.value.length === 0) {
        createNewNote();
     }
  });

  syncService.refreshBookNotes(book.id).catch(() => {});
    
  if (isEPUB(activeBook.value) && viewMode.value !== 'notes') {
    isEpubLoading.value = true;
    nextTick(() => initEpubReader());
  }
};

const cleanupEpub = () => {
  if (epubRendition.value) { try { epubRendition.value.destroy(); } catch(e){} epubRendition.value = null; }
  if (epubBook.value) { try { epubBook.value.destroy(); } catch(e){} epubBook.value = null; }
  if (observer) { observer.disconnect(); observer = null; }
  isEpubLoading.value = false;
};

const waitForLibs = () => {
  return new Promise((resolve, reject) => {
    let attempts = 0;
    const check = () => {
      // @ts-ignore
      if (typeof ePub !== 'undefined' && typeof JSZip !== 'undefined') resolve(true);
      else if (attempts > 50) reject(new Error("Core engines (EPub/JSZip) timeout."));
      else { attempts++; setTimeout(check, 250); }
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

    // Kill any existing observer
    if (observer) observer.disconnect();
    observer = new MutationObserver((mutations) => {
       mutations.forEach((m) => {
          m.addedNodes.forEach((node: any) => {
             if (node.tagName === 'IFRAME') {
                node.removeAttribute('sandbox');
                node.style.background = 'transparent';
                node.allow = "autoplay; encrypted-media; fullscreen"; 
             }
          });
       });
    });
    if (epubViewerRef.value) observer.observe(epubViewerRef.value, { childList: true, subtree: true });

    // @ts-ignore
    epubBook.value = ePub(buffer);
    
    // We use default manager with scrolled if possible for better scrolling, but user likes paginated
    epubRendition.value = epubBook.value.renderTo(epubViewerRef.value, { 
       width: "100%", height: "100%", flow: "paginated"
    });
    
    // DEEP CLEANSE HOOK: Critical for both normal and cleaned epubs
    epubRendition.value.hooks.content.register((contents: any) => {
       const doc = contents.document;
       
       // Force a global theme override directly in the iframe
       const style = doc.createElement('style');
       style.textContent = `
          body { 
             font-family: 'Outfit', system-ui, sans-serif !important;
             color: #cbd5e1 !important;
             background: transparent !important;
             margin: 2rem !important;
             line-height: 1.8 !important;
          }
          * { max-width: 100%; border-color: #334155 !important; }
          a { color: #fbbf24 !important; }
          @font-face { display: none !important; }
       `;
       doc.head.appendChild(style);

       // Sanitization sweep
       doc.querySelectorAll('script').forEach((s: any) => s.remove());
       doc.querySelectorAll('*').forEach((el: any) => {
          for (let i = 0; i < el.attributes.length; i++) {
             const attr = el.attributes[i];
             if (attr.name.startsWith('on')) { el.removeAttribute(attr.name); i--; }
          }
       });
    });

    await epubRendition.value.display();
    
    // Force first page visible and clear loader
    isEpubLoading.value = false;
    epubError.value = null;
    
    // Trigger one final resize to fix black screen artifacts
    setTimeout(() => {
       if(epubRendition.value) epubRendition.value.resize();
    }, 500);

  } catch (e: any) { 
    console.error('Reader Intel Error:', e);
    epubError.value = e.message || "Engine hardware synchronization failure."; 
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
      await syncService.addBookNote(activeBook.value.id, payload);
    } else { 
      await syncService.updateBookNote(activeNote.value.id, payload); 
    }
  } catch (e) { alert('Commit fail'); } finally { isSaving.value = false; }
};

const deleteNote = async (id: string) => {
  if (id.startsWith('temp-')) { activeNote.value = null; return; }
  if (!confirm('Eliminate data?')) return;
  try {
    await syncService.removeBookNote(id);
  } catch (e) { alert('Operation aborted'); }
};

const removeBookStatus = async (id: string) => {
  if (!confirm('Detach?')) return;
  try { 
    await syncService.removeBook(id); 
    activeBook.value = null; 
  } catch (e) { alert('Operation fail'); }
};

const importBook = async (res: any) => {
  try {
    await syncService.addBookToBookcase({ 
      storeId: res.id, 
      title: res.title || res.caption || 'Source', 
      category: res.mediaType?.toUpperCase() || 'VOLUME' 
    });
    showAddModal.value = false;
  } catch (e) { alert('Import error'); }
};


const moveUp = async (book: any, folderBooks: any[]) => {
  const index = folderBooks.findIndex(b => b.id === book.id);
  if (index <= 0) return;
  
  const prevBook = folderBooks[index - 1];
  const newOrder = (prevBook.sortOrder || 0) - 1;
  await syncService.moveBook(book.id, newOrder);
};

const moveDown = async (book: any, folderBooks: any[]) => {
  const index = folderBooks.findIndex(b => b.id === book.id);
  if (index < 0 || index >= folderBooks.length - 1) return;
  
  const nextBook = folderBooks[index + 1];
  const newOrder = (nextBook.sortOrder || 0) + 1;
  await syncService.moveBook(book.id, newOrder);
};

const folders = computed(() => {
  const groups: Record<string, any[]> = { 'Uncategorized': [] };
  customFolders.value.forEach(f => { if (!groups[f]) groups[f] = []; });
  
  // Sort all books by sortOrder before grouping
  const sortedBooks = [...books.value].sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));

  sortedBooks.forEach(book => {
    if (searchTerm.value && !book.title?.toLowerCase().includes(searchTerm.value.toLowerCase())) return;
    const f = book.folder || 'Uncategorized';
    if (!groups[f]) groups[f] = []; 
    groups[f].push(book);
  });
  return groups;
});

const onDragStart = (item: any) => {
  draggedItem.value = item;
};

const handleDragOver = (event: DragEvent, target: any, explicitPosition?: 'inside') => {
  event.preventDefault();
  dropTargetId.value = target.id || target; 
  
  if (explicitPosition) {
    dropPosition.value = explicitPosition;
  } else {
    const targetElement = event.currentTarget as HTMLElement | null;
    if (targetElement) {
      const rect = targetElement.getBoundingClientRect();
      const mid = rect.top + rect.height / 2;
      dropPosition.value = event.clientY < mid ? 'before' : 'after';
    }
  }
};

const handleDragLeave = () => {
    dropTargetId.value = null;
    dropPosition.value = null;
};

const handleReorder = async (data: { targetBook: any, position: 'before' | 'after' | 'inside' | null }) => {
    if (!draggedItem.value || !data.position || data.position === 'inside') return;
    const target = data.targetBook;
    if (draggedItem.value.id === target.id) return;

    try {
        isLoading.value = true;
        const folder = target.folder || '';
        const siblings = [...books.value]
            .filter(b => (b.folder || '') === folder && b.id !== draggedItem.value.id)
            .sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0));
        
        const targetIdx = siblings.findIndex(b => b.id === target.id);
        const insertIdx = data.position === 'before' ? targetIdx : targetIdx + 1;

        // Create a clean version of the dragged book to avoid Vue proxy issues in the array
        const cleanDragged = { ...draggedItem.value, folder: folder };
        siblings.splice(insertIdx, 0, cleanDragged);

        const updates = [];
        for (let i = 0; i < siblings.length; i++) {
            updates.push(syncService.moveBook(siblings[i].id, i));
            // Also ensure the folder is updated if moved between folders via reorder
            if ((siblings[i].folder || '') !== folder) {
                updates.push(syncService.updateBookFolder(siblings[i].id, folder));
            }
        }

        await Promise.all(updates);
    } catch (err) {
        console.error("Book reorder failed:", err);
    } finally {
        isLoading.value = false;
        dropTargetId.value = null;
        dropPosition.value = null;
        draggedItem.value = null;
    }
};

const handleFolderDrop = async (folderName: string) => {
  const folder = folderName === 'Uncategorized' ? '' : folderName;
  if (!draggedItem.value) return;
  
  try { 
    await syncService.updateBookFolder(draggedItem.value.id, folder); 
  } catch (e) { 
    console.error("Folder update failed", e);
  } finally {
    dragOverFolder.value = null;
    draggedItem.value = null;
  }
};

const getFileUrl = (book: any) => { if (!book || !book.storeId) return ''; return `${import.meta.env.VITE_API_URL}/api/storehouse/file/${book.storeId}`; };
const isEPUB = (book: any) => { if (!book) return false; return (book.title || '').toLowerCase().endsWith('.epub'); };

watch(customFolders, (newVal) => { localStorage.setItem('kb_custom_folders', JSON.stringify(newVal)); }, { deep: true });
</script>

<template>
  <div class="bookcase-v2">
    <aside class="sidebar">
      <div class="sidebar-header"><input v-model="searchTerm" placeholder="Filter Intel..." /><button @click="showAddModal = true" class="add-btn">+</button></div>
      <div class="folder-list">
        <div v-for="(folderBooks, folderName) in folders" :key="folderName" class="folder-group" 
             :class="{ 'drop-over': dropTargetId === String(folderName) && dropPosition === 'inside' }" 
             @dragover.prevent="handleDragOver($event, String(folderName), 'inside')" 
             @dragleave="handleDragLeave" 
             @drop="handleFolderDrop(String(folderName))">
          <div class="folder-header" @click="collapsedFolders.has(String(folderName)) ? collapsedFolders.delete(String(folderName)) : collapsedFolders.add(String(folderName))">
            <span class="fold-arrow">{{ collapsedFolders.has(String(folderName)) ? '▶' : '▼' }}</span>
            <span class="folder-name">{{ folderName }}</span>
            <span class="count">{{ folderBooks.length }}</span>
          </div>
          <div v-show="!collapsedFolders.has(String(folderName))" class="folder-content">
            <div 
              v-for="book in folderBooks" 
              :key="book.id" 
              class="book-item" 
              :class="{ 
                active: activeBook?.id === book.id,
                'drop-before': dropTargetId === book.id && dropPosition === 'before',
                'drop-after': dropTargetId === book.id && dropPosition === 'after',
                'is-dragging': draggedItem?.id === book.id
              }" 
              draggable="true" 
              @dragstart="onDragStart(book)" 
              @dragover.prevent="handleDragOver($event, book)" 
              @dragleave="handleDragLeave"
              @drop="handleReorder({ targetBook: book, position: dropPosition })"
              @click="selectBook(book)"
            >
              <div class="item-icon">🔖</div>
              <div class="item-info">
                <div class="item-title">{{ book.title }}</div>
                <div class="item-meta">{{ book.category }}</div>
              </div>
              <div class="sort-actions" @click.stop>
                <button @click="moveUp(book, folderBooks)">▴</button>
                <button @click="moveDown(book, folderBooks)">▾</button>
              </div>
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
          <iframe v-if="activeBook.title?.toLowerCase().endsWith('.pdf')" :src="getFileUrl(activeBook)" class="pdf-frame"></iframe>
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
              <textarea v-if="activeNote.noteType !== 'md'" v-model="activeNote.content" placeholder="Data entry..." />
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
.book-item { padding: 0.65rem 0.65rem 0.65rem 1.5rem; display: flex; gap: 0.6rem; cursor: pointer; border-radius: 6px; margin-bottom: 2px; transition: all 0.2s; position: relative; }
.book-item.active { background: #d977061a; border: 1px solid #d9770633; color: #fff; }
.book-item.is-dragging { opacity: 0.4; background: #222; }
.book-item.drop-before { border-top: 2px solid #fbbf24; }
.book-item.drop-after { border-bottom: 2px solid #fbbf24; }
.folder-group.drop-over { background: rgba(217, 119, 6, 0.1); }
.item-title { font-size: 0.8rem; text-align: left; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; }
.item-meta { font-size: 0.6rem; opacity: 0.4; }
.sort-actions { display: none; flex-direction: column; gap: 0; margin-left: auto; }
.book-item:hover .sort-actions { display: flex; }
.sort-actions button { background: transparent; border: none; color: #555; cursor: pointer; padding: 0 4px; font-size: 0.9rem; line-height: 1; }
.sort-actions button:hover { color: #fbbf24; }
.workspace { flex: 1; display: flex; flex-direction: column; }
.active-book-info { display: flex; align-items: center; gap: 0.8rem; overflow: hidden; flex: 1; }
.active-book-info h2 { font-size: 1.1rem; color: #fff; margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ws-header { height: 64px; padding: 0 1.25rem; display: flex; align-items: center; gap: 1.5rem; justify-content: space-between; border-bottom: 1px solid #1a1e23; background: #000; }
.badge { font-size: 0.6rem; background: #fbbf2422; color: #fbbf24; padding: 2px 6px; border-radius: 4px; font-weight: 800; flex-shrink: 0; }
.tabs-nav { display: flex; gap: 4px; background: #11151a; padding: 4px; border-radius: 8px; flex-shrink: 0; }
.tabs-nav button { padding: 0.4rem 1rem; border: none; background: transparent; color: #666; font-size: 0.75rem; font-weight: 700; border-radius: 6px; cursor: pointer; transition: all 0.2s; white-space: nowrap; }
.tabs-nav button.active { background: #d97706; color: #fff; }
.detach-btn { background: rgba(255, 87, 87, 0.1); border: 1px solid rgba(255, 87, 87, 0.2); color: #ff5757; padding: 0.4rem; border-radius: 6px; cursor: pointer; flex-shrink: 0; transition: all 0.2s; }
.detach-btn:hover { background: #ff5757; color: #fff; }

.ws-body { flex: 1; display: flex; overflow: hidden; }
.preview-pane { flex: 1.4; position: relative; background: #121519; border-right: 1px solid #1a1e23; overflow: hidden; }
.pdf-frame { width: 100%; height: 100%; border: none; background: #1a1e23; }
.epub-reader { width:100%; height:100%; position:relative; background: #000; }
.epub-canvas { width:100%; height:100%; position: absolute; top:0; left:0; }
:deep(.epub-canvas iframe) { width: 100% !important; height: 100% !important; border: none !important; }
.reader-controls { position: absolute; bottom: 2rem; left: 50%; transform: translateX(-50%); display: flex; gap: 1rem; z-index: 1000; }
.nav-btn { background: #d97706DD; backdrop-filter: blur(8px); color: #fff; border: 1px solid #fbbf2444; padding: 0.6rem 1.25rem; border-radius: 4px; cursor: pointer; font-size: 0.8rem; font-weight: 900; }
.notes-pane { flex: 1; display: flex; flex-direction: column; background: #0a0c10; min-width: 400px; }
.note-tabs { display: flex; padding: 0.6rem 1rem 0; border-bottom: 1px solid #1a1e23; overflow-x: auto; gap: 4px; scrollbar-width: none; }
.note-tabs::-webkit-scrollbar { display: none; }
.note-tab { padding: 0.5rem 1.25rem; font-size: 0.8rem; background: #11151a; border-radius: 6px 6px 0 0; cursor: pointer; color: #666; white-space: nowrap; }
.note-tab.active { background: #1a1e23; color: #fbbf24; }
.new-btn { background: transparent; border: 1px dashed #333; color: #555; font-size: 0.7rem; padding: 0 0.8rem; border-radius: 4px; margin-bottom: 4px; cursor: pointer; white-space: nowrap; }
.new-btn:hover { color: #fff; border-color: #555; }

.ed-toolbar { padding: 1rem; display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #1a1e23; gap: 1rem; }
.title-in { flex: 1; background: transparent; border: none; color: #fff; font-size: 1.1rem; font-weight: 700; outline: none; overflow: hidden; text-overflow: ellipsis; }
.actions { display: flex; align-items: center; gap: 0.5rem; flex-shrink: 0; }
.cycle-btn, .commit-btn, .pin-btn, .del-btn { 
  padding: 0.4rem 0.8rem; font-size: 0.75rem; font-weight: 900; cursor: pointer; border-radius: 4px; border: none; white-space: nowrap; height: 32px; display: flex; align-items: center; justify-content: center;
}
.cycle-btn { background: #11151a; color: #666; border: 1px solid #222; }
.cycle-btn:hover { color: #fff; border-color: #444; }
.commit-btn { background: #d97706; color: #fff; }
.commit-btn:hover { filter: brightness(1.1); }
.pin-btn { background: #11151a; border: 1px solid #222; }
.del-btn { background: #11151a; border: 1px solid #222; }
.del-btn:hover { background: #ff5757; color: #fff; border-color: #ff5757; }

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
