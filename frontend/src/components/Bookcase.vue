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
const dropTargetId = ref<string | null>(null);
const dropPosition = ref<'before' | 'after' | 'inside' | null>(null);
const draggedItem = ref<any>(null);
const collapsedFolders = ref<string[]>([]);
const isSorting = ref(false);
const isSidebarCollapsed = ref(window.innerWidth < 1024);

// Handle Responsive Auto-collapse
const handleResize = () => {
  if (window.innerWidth < 1024) isSidebarCollapsed.value = true;
};

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
  window.addEventListener('resize', handleResize);
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
  window.removeEventListener('resize', handleResize);
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

// --- Computed Helpers ---
const activeBookIsEpub = computed(() => isEPUB(activeBook.value));
const activeBookIsPdf = computed(() => activeBook.value?.title?.toLowerCase().endsWith('.pdf') ?? false);
const activeBookFileUrl = computed(() => getFileUrl(activeBook.value));
const renderedMarkdown = computed(() => marked(activeNote.value?.content || ''));

const sortedBooks = computed(() => 
  [...books.value].sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0))
);

const folders = computed(() => {
  const groups: Record<string, any[]> = { 'Uncategorized': [] };
  customFolders.value.forEach(f => { if (!groups[f]) groups[f] = []; });
  
  const term = searchTerm.value.toLowerCase();
  sortedBooks.value.forEach(book => {
    if (term && !book.title?.toLowerCase().includes(term)) return;
    const f = book.folder || 'Uncategorized';
    if (!groups[f]) groups[f] = []; 
    groups[f].push(book);
  });
  return groups;
});

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
    const payload = { 
      title: activeNote.value.title, 
      content: activeNote.value.content, 
      noteType: activeNote.value.noteType === 'txt' ? 'txt' : 'markdown' 
    };
    if (activeNote.value.id.startsWith('temp-')) {
      await syncService.addBookNote(activeBook.value.id, payload);
    } else { 
      await syncService.updateBookNote(activeNote.value.id, payload); 
    }
  } catch (e) { 
    console.error('Save failed', e);
  } finally { 
    isSaving.value = false; 
  }
};

let autoSaveTimer: any = null;
watch(() => activeNote.value?.content, () => {
  if (!activeNote.value || activeNote.value.id.startsWith('temp-')) return;
  if (autoSaveTimer) clearTimeout(autoSaveTimer);
  autoSaveTimer = setTimeout(() => {
    saveCurrentNote();
  }, 2000);
});

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
    const remaining = books.value.filter(b => b.id !== id);
    if (remaining.length > 0) {
      selectBook(remaining[0]);
    } else {
      activeBook.value = null; 
    }
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
  if (isSorting.value) return;
  const index = folderBooks.findIndex(b => b.id === book.id);
  if (index <= 0) return;
  
  try {
    isSorting.value = true;
    const siblings = [...folderBooks];
    // 記憶體中交換位置
    [siblings[index - 1], siblings[index]] = [siblings[index], siblings[index - 1]];
    
    // 全量更新此資料夾內的所有書籍 sortOrder
    const updates = siblings.map((b, i) => syncService.moveBook(b.id, i));
    await Promise.all(updates);
  } catch (err) {
    console.error("Move up failed:", err);
  } finally {
    isSorting.value = false;
  }
};

const moveDown = async (book: any, folderBooks: any[]) => {
  if (isSorting.value) return;
  const index = folderBooks.findIndex(b => b.id === book.id);
  if (index < 0 || index >= folderBooks.length - 1) return;
  
  try {
    isSorting.value = true;
    const siblings = [...folderBooks];
    // 記憶體中交換位置
    [siblings[index], siblings[index + 1]] = [siblings[index + 1], siblings[index]];
    
    // 全量更新此資料夾內的所有書籍 sortOrder
    const updates = siblings.map((b, i) => syncService.moveBook(b.id, i));
    await Promise.all(updates);
  } catch (err) {
    console.error("Move down failed:", err);
  } finally {
    isSorting.value = false;
  }
};

const addFolder = () => {
  const name = newFolderName.value.trim();
  if (!name || name === 'Uncategorized' || customFolders.value.includes(name)) return;
  customFolders.value.push(name);
  newFolderName.value = '';
};

const toggleFolder = (name: string) => {
  const idx = collapsedFolders.value.indexOf(name);
  if (idx >= 0) {
    collapsedFolders.value.splice(idx, 1);
  } else {
    collapsedFolders.value.push(name);
  }
};

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
    dropTargetId.value = null;
    dropPosition.value = null;
    draggedItem.value = null;
  }
};

let searchTimer: any = null;
const searchAvailableBooks = () => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(async () => {
    availableResources.value = await apiService.getAvailableBooks(searchQuery.value);
  }, 400);
};

const getFileUrl = (book: any) => { 
  if (!book || !book.storeId) return ''; 
  // 修正 Chromebook 下載問題：網址末端附加書名與 .pdf 擴展名，誘導瀏覽器使用內建預覽器
  const baseUrl = apiService.getAbsoluteUrl(`/api/storehouse/file/${book.storeId}`);
  const safeTitle = encodeURIComponent(book.title || 'document').replace(/%20/g, '+');
  return `${baseUrl}/${safeTitle}`;
};
const isEPUB = (book: any) => { if (!book) return false; return (book.title || '').toLowerCase().endsWith('.epub'); };

watch(customFolders, (newVal) => { localStorage.setItem('kb_custom_folders', JSON.stringify(newVal)); }, { deep: true });
</script>

<template>
  <div class="bookcase-v2" :class="{ 'sb-collapsed': isSidebarCollapsed }">
    <aside class="sidebar" :class="{ collapsed: isSidebarCollapsed }">
      <div class="sidebar-header">
        <input v-model="searchTerm" placeholder="Filter Intel..." />
        <button @click="showAddModal = true" class="add-btn">+</button>
      </div>
      <div class="folder-list">
        <div v-for="(folderBooks, folderName) in folders" :key="folderName" class="folder-group" 
             :class="{ 'drop-over': dropTargetId === String(folderName) && dropPosition === 'inside' }" 
             @dragover.prevent="handleDragOver($event, String(folderName), 'inside')" 
             @dragleave="handleDragLeave" 
             @drop="handleFolderDrop(String(folderName))">
          <div class="folder-header" @click="toggleFolder(String(folderName))">
            <span class="fold-arrow">{{ collapsedFolders.includes(String(folderName)) ? '▶' : '▼' }}</span>
            <span class="folder-name">{{ folderName }}</span>
            <span class="count">{{ folderBooks.length }}</span>
          </div>
          <div v-show="!collapsedFolders.includes(String(folderName))" class="folder-content">
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
              <div class="sort-actions" @click.stop v-if="!isSorting">
                <button @click="moveUp(book, folderBooks)">▴</button>
                <button @click="moveDown(book, folderBooks)">▾</button>
              </div>
              <div class="sort-actions loading" v-else>...</div>
            </div>
          </div>
        </div>
        <div class="new-folder-area"><input v-model="newFolderName" placeholder="+ Cluster" @keyup.enter="addFolder" /></div>
      </div>
    </aside>

    <main v-if="activeBook" class="workspace">
      <header class="ws-header">
        <button class="toggle-sidebar-btn" @click="isSidebarCollapsed = !isSidebarCollapsed">
          {{ isSidebarCollapsed ? '📂' : '⬅️' }}
        </button>
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
          <iframe v-if="activeBookIsPdf && activeBookFileUrl" :src="activeBookFileUrl" class="pdf-frame"></iframe>
          <div v-else-if="activeBookIsEpub" class="epub-reader">
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
              <div v-if="activeNote.noteType !== 'txt'" class="markdown-body" v-html="renderedMarkdown" />
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
          <input v-model="searchQuery" placeholder="Filter Sources..." @input="searchAvailableBooks" />
          <div class="items">
            <div v-for="res in availableResources" :key="res.id" @click="importBook(res)"><span>[{{ res.mediaType }}]</span>{{ res.title || res.caption }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-v2 { 
  display: flex; 
  height: calc(100vh - 240px); /* Account for dashboard header + tabs */
  min-height: 600px;
  background: #0a0c10; 
  color: #a0a8b1; 
  font-family: 'Outfit', sans-serif;
  border-radius: 0 0 24px 24px;
}
.detach-btn:hover { background: #ff5757; color: #fff; }

/* 🚀 Sidebar Collapse & Responsive Logic */
.toggle-sidebar-btn {
  background: #1a1e23;
  border: 1px solid #334155;
  color: #fff;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.1rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  flex-shrink: 0;
}
.toggle-sidebar-btn:hover { background: #334155; border-color: #fbbf24; }

.sidebar { 
  width: 280px; 
  background: #000; 
  border-right: 1px solid #1a1e23; 
  display: flex; 
  flex-direction: column; 
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.sidebar-header { padding: 1.25rem; display: flex; gap: 0.5rem; border-bottom: 1px solid #111; }
.sidebar-header input { flex: 1; background: #000; border: 1px solid #1a1e23; padding: 0.6rem; border-radius: 6px; color: #fff; font-size: 0.8rem; outline: none; }
.add-btn { background: #d97706; border: none; color: #fff; width: 32px; height: 32px; border-radius: 6px; cursor: pointer; font-weight: 900; }

.folder-list { flex: 1; overflow-y: auto; padding-bottom: 5rem; }
.folder-group { margin-bottom: 1px; }
.folder-header { padding: 0.8rem 1.25rem; display: flex; align-items: center; gap: 0.8rem; background: #050505; cursor: pointer; transition: 0.2s; border-left: 3px solid transparent; }
.folder-header:hover { background: #0a0a0a; border-left-color: #fbbf24; }
.fold-arrow { font-size: 0.6rem; opacity: 0.4; transition: 0.3s; }
.folder-name { flex: 1; font-weight: 800; font-size: 0.75rem; letter-spacing: 0.5px; text-transform: uppercase; color: #fff; }
.count { font-size: 0.65rem; background: #111; padding: 2px 8px; border-radius: 10px; opacity: 0.5; }

.book-item { padding: 0.75rem 1.25rem 0.75rem 3rem; display: flex; align-items: center; gap: 1rem; cursor: pointer; position: relative; transition: 0.2s; border-bottom: 1px solid #080808; }
.book-item:hover { background: #0a0c10; }
.book-item.active { background: #1a1e23; border-left: 3px solid #fbbf24; }
.book-item.is-dragging { opacity: 0.4; }
.item-icon { font-size: 1.1rem; filter: grayscale(1); opacity: 0.3; }
.active .item-icon { filter: none; opacity: 1; }
.item-info { flex: 1; overflow: hidden; text-align: left;}
.item-title { font-size: 0.85rem; font-weight: 600; color: #cbd5e1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.active .item-title { color: #fff; }
.item-meta { font-size: 0.65rem; opacity: 0.4; text-transform: uppercase; letter-spacing: 0.5px; }

.sort-actions { display: none; flex-direction: column; opacity: 0.5; transition: 0.2s; }
.book-item:hover .sort-actions { display: flex; }
.sort-actions button { background: none; border: none; color: #fff; cursor: pointer; font-size: 0.9rem; padding: 0 4px; }
.sort-actions button:hover { color: #fbbf24; opacity: 1; }

.new-folder-area { padding: 1.25rem; }
.new-folder-area input { width: 100%; background: transparent; border: 1px dashed #222; padding: 0.6rem; border-radius: 6px; font-size: 0.7rem; color: #444; outline: none; transition: 0.2s; }
.new-folder-area input:focus { border-color: #555; color: #888; }
.sidebar.collapsed {
  width: 0;
  border-right: none;
  opacity: 0;
  pointer-events: none;
}

@media (max-width: 1024px) {
  .sidebar {
    position: absolute;
    z-index: 1000;
    height: 100%;
    box-shadow: 20px 0 50px rgba(0,0,0,0.8);
  }
  .sidebar.collapsed {
    transform: translateX(-100%);
    width: 280px; /* Keep width for animation but hide via transform */
  }
  .preview-pane { flex: 1 !important; } /* Balance PDF on tablets */
  .notes-pane { min-width: 320px !important; }
}

@media (max-width: 768px) {
  .ws-body.mode-mixed { flex-direction: column; }
  .preview-pane { height: 50vh; flex: none !important; border-bottom: 2px solid #1a1e23; border-right: none; }
  .notes-pane { flex: 1; min-width: 0 !important; }
  .active-book-info h2 { font-size: 0.9rem; }
  .tabs-nav button { padding: 0.4rem 0.6rem; font-size: 0.65rem; }
}

/* 🛠️ Workspace Layout Expansion Fix */
.workspace {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #000;
  position: relative;
}

.ws-header {
  height: 72px;
  padding: 0 1.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #050505;
  border-bottom: 1px solid #1a1e23;
  gap: 1.5rem;
  flex-shrink: 0;
}

.active-book-info {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
  min-width: 0;
}

.active-book-info h2 {
  font-size: 1.1rem;
  font-weight: 800;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
}

.badge {
  background: #1e293b;
  color: #94a3b8;
  padding: 0.2rem 0.6rem;
  border-radius: 4px;
  font-size: 0.65rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.tabs-nav {
  display: flex;
  gap: 4px;
  background: #111;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid #222;
}

.tabs-nav button {
  padding: 0.5rem 1rem;
  font-size: 0.75rem;
  font-weight: 800;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: #555;
  cursor: pointer;
  transition: 0.2s;
  white-space: nowrap;
}

.tabs-nav button.active {
  background: #d97706;
  color: #fff;
}

.detach-btn {
  background: transparent;
  border: 1px solid #222;
  color: #555;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  cursor: pointer;
  transition: 0.2s;
}

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
