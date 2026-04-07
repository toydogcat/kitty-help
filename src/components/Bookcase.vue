<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';
import { usePin } from '../composables/usePin';

const props = defineProps<{
  userId: string;
}>();

const { pinToDesk } = usePin();

const books = ref<any[]>([]);
const availableResources = ref<any[]>([]);
const isLoading = ref(false);
const isSearching = ref(false);
const searchQuery = ref('');
const showAddModal = ref(false);

const activeBook = ref<any>(null);
const showNoteModal = ref(false);
const noteEditorContent = ref('');
const isSavingNote = ref(false);

const fetchBookcase = async () => {
  isLoading.value = true;
  try {
    books.value = await apiService.getBookcase();
  } catch (err) {
    console.error('Failed to fetch bookcase:', err);
  } finally {
    isLoading.value = false;
  }
};

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

const addBook = async (res: any) => {
  try {
    await apiService.addBookToBookcase({
      storeId: res.id,
      title: res.title || res.caption || 'Untitled Book',
      category: res.mediaType?.toUpperCase() || 'BOOK'
    });
    showAddModal.value = false;
    fetchBookcase();
  } catch (err) {
    alert('Failed to add book');
  }
};

const removeBook = async (id: string) => {
  if (confirm('Remove this book from your bookcase?')) {
    try {
      await apiService.removeBook(id);
      fetchBookcase();
    } catch (err) {
      console.error('Remove failed:', err);
    }
  }
};

const openNotes = (book: any) => {
  activeBook.value = book;
  noteEditorContent.value = book.notes || '';
  showNoteModal.value = true;
};

const saveNote = async () => {
  if (!activeBook.value) return;
  isSavingNote.value = true;
  try {
    await apiService.updateBookNotes(activeBook.value.id, noteEditorContent.value);
    activeBook.value.notes = noteEditorContent.value;
    showNoteModal.value = false;
    fetchBookcase();
  } catch (err) {
    alert('Failed to save notes');
  } finally {
    isSavingNote.value = false;
  }
};

const pinBookToDesk = async (book: any) => {
  try {
    await pinToDesk('book', book.id);
    alert(`Pinned ${book.title} notes to Desk!`);
  } catch (err) {
    console.error('Pin failed:', err);
  }
};

onMounted(() => {
  fetchBookcase();
});
</script>

<template>
  <div class="bookcase-container">
    <div class="section-header">
      <div class="title-area">
        <h1>📚 Digital Bookcase</h1>
        <p>Manage your PDF, EPUB, and DJVU collections</p>
      </div>
      <button @click="showAddModal = true; searchAvailable()" class="add-btn">+ Add Book</button>
    </div>

    <!-- Book Grid -->
    <div v-if="isLoading" class="loader">Loading your library...</div>
    <div v-else-if="books.length === 0" class="empty-state">
      <div class="icon">📖</div>
      <p>Your bookcase is empty. Add some books from your store!</p>
    </div>
    <div v-else class="book-grid">
      <div v-for="book in books" :key="book.id" class="book-card">
        <div class="book-type-badge">{{ book.category }}</div>
        <div class="book-info">
          <h3 class="book-title">{{ book.title }}</h3>
          <p class="book-meta">Added on {{ new Date(book.createdAt).toLocaleDateString() }}</p>
        </div>
        <div class="book-actions">
          <button @click="openNotes(book)" class="action-btn note-btn">📝 Notes</button>
          <button @click="pinBookToDesk(book)" class="action-btn pin-btn">📌 Pin</button>
          <button @click="removeBook(book.id)" class="action-btn delete-btn">🗑️</button>
        </div>
      </div>
    </div>

    <!-- Add Book Modal -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content add-book-modal">
        <div class="modal-header">
          <h2>Select Book from Store</h2>
          <button @click="showAddModal = false" class="close-btn">&times;</button>
        </div>
        <div class="search-bar">
          <input 
            v-model="searchQuery" 
            placeholder="Search by title..." 
            @input="searchAvailable"
          />
        </div>
        <div class="available-list">
          <div v-if="isSearching" class="mini-loader">Searching...</div>
          <div v-else-if="availableResources.length === 0" class="no-results">No documents found.</div>
          <div 
            v-for="res in availableResources" 
            :key="res.id" 
            class="resource-item"
            @click="addBook(res)"
          >
            <span class="res-type">[{{ res.mediaType }}]</span>
            <span class="res-title">{{ res.title || res.caption || 'Scan Item' }}</span>
            <span class="res-date">{{ new Date(res.createdAt).toLocaleDateString() }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Notes Editor Modal -->
    <div v-if="showNoteModal" class="modal-overlay" @click.self="showNoteModal = false">
      <div class="modal-content note-modal">
        <div class="modal-header">
          <h2>Notes: {{ activeBook?.title }}</h2>
          <button @click="showNoteModal = false" class="close-btn">&times;</button>
        </div>
        <div class="editor-layout">
          <div class="editor-pane">
            <textarea 
              v-model="noteEditorContent" 
              placeholder="Write your study notes in Markdown..."
            ></textarea>
          </div>
          <div class="preview-pane markdown-body" v-html="marked(noteEditorContent)"></div>
        </div>
        <div class="modal-footer">
          <button @click="saveNote" :disabled="isSavingNote" class="save-btn">
            {{ isSavingNote ? 'Saving...' : 'Save & Close' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bookcase-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  background: rgba(255, 255, 255, 0.02);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.title-area h1 {
  margin: 0;
  font-size: 1.8rem;
  background: linear-gradient(135deg, #fbbf24 0%, #d97706 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.title-area p {
  margin: 0.2rem 0 0;
  opacity: 0.5;
  font-size: 0.9rem;
}

.add-btn {
  padding: 0.6rem 1.2rem;
  background: #d97706;
  border: none;
  border-radius: 8px;
  color: white;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.add-btn:hover {
  background: #fbbf24;
  transform: translateY(-2px);
}

.book-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1.5rem;
  padding: 2rem;
  overflow-y: auto;
}

.book-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  position: relative;
  transition: all 0.3s;
}

.book-card:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: #d97706;
}

.book-type-badge {
  position: absolute;
  top: 1rem;
  right: 1rem;
  background: rgba(217, 119, 6, 0.2);
  color: #fbbf24;
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.05rem;
}

.book-info {
  margin-bottom: 1.5rem;
  text-align: left;
}

.book-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.book-meta {
  margin: 0.4rem 0 0;
  font-size: 0.8rem;
  opacity: 0.4;
}

.book-actions {
  display: flex;
  gap: 0.6rem;
}

.action-btn {
  flex: 1;
  padding: 0.5rem;
  border: none;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.note-btn { background: rgba(var(--primary-rgb), 0.1); color: var(--primary-color); }
.pin-btn { background: rgba(255, 255, 255, 0.05); color: white; }
.delete-btn { width: 40px; flex: none; background: rgba(239, 68, 68, 0.1); color: #ef4444; }

.action-btn:hover { transform: scale(1.05); }

/* Modals */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background: #1a1a1a;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 20px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
}

.add-book-modal {
  width: 500px;
  max-height: 80vh;
}

.note-modal {
  width: 90vw;
  height: 90vh;
}

.modal-header {
  padding: 1.5rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.modal-header h2 { margin: 0; font-size: 1.3rem; }

.close-btn {
  background: none;
  border: none;
  color: white;
  font-size: 1.5rem;
  cursor: pointer;
  opacity: 0.5;
}

.search-bar { padding: 1rem 1.5rem; }
.search-bar input {
  width: 100%;
  padding: 0.8rem 1rem;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  color: white;
}

.available-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 1rem 1rem;
}

.resource-item {
  padding: 0.8rem 1rem;
  margin-bottom: 0.5rem;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.8rem;
  transition: all 0.2s;
  text-align: left;
}

.resource-item:hover {
  background: rgba(217, 119, 6, 0.1);
  transform: translateX(5px);
}

.res-type { color: #fbbf24; font-size: 0.7rem; font-weight: 800; min-width: 60px; }
.res-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.res-date { font-size: 0.75rem; opacity: 0.3; }

.editor-layout {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.editor-pane {
  flex: 1;
  border-right: 1px solid rgba(255, 255, 255, 0.05);
}

.editor-pane textarea {
  width: 100%;
  height: 100%;
  padding: 2rem;
  background: transparent;
  border: none;
  color: #e5e7eb;
  font-family: 'JetBrains Mono', monospace;
  font-size: 1.1rem;
  line-height: 1.6;
  resize: none;
  outline: none;
}

.preview-pane {
  flex: 1;
  padding: 2rem;
  overflow-y: auto;
  text-align: left;
}

.modal-footer {
  padding: 1.5rem;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.save-btn {
  padding: 0.8rem 2rem;
  background: #d97706;
  border: none;
  border-radius: 10px;
  color: white;
  font-weight: 700;
  cursor: pointer;
}

.save-btn:hover { background: #fbbf24; }
.save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.loader, .empty-state, .no-results, .mini-loader {
  padding: 4rem;
  text-align: center;
  opacity: 0.5;
}

.empty-state .icon { font-size: 4rem; margin-bottom: 1rem; }
</style>
