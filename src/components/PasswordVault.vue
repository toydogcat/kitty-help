<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
  userId: string;
}>();

const passwords = ref<any[]>([]);
const isLoading = ref(false);
const showAddModal = ref(false);
const newPassword = ref({
  siteName: '',
  account: '',
  passwordRaw: '',
  category: 'General',
  notes: ''
});
const currentCategory = ref('All');
const categories = ['General', 'Work', 'Social', 'Admin', 'Finance', 'Game'];

const filteredPasswords = computed(() => {
  if (currentCategory.value === 'All') return passwords.value;
  return passwords.value.filter(p => p.category === currentCategory.value);
});

const fetchPasswords = async () => {
  isLoading.value = true;
  try {
    const res = await apiService.getPasswords();
    passwords.value = res || [];
  } catch (err) {
    console.error('Failed to fetch passwords:', err);
    passwords.value = [];
  } finally {
    isLoading.value = false;
  }
};

const handleAddPassword = async () => {
  if (!newPassword.value.siteName || !newPassword.value.account || !newPassword.value.passwordRaw) {
    alert('Please fill in all required fields');
    return;
  }

  try {
    await apiService.addPassword({
      ...newPassword.value
    });
    showAddModal.value = false;
    newPassword.value = { siteName: '', account: '', passwordRaw: '', category: 'General', notes: '' };
    fetchPasswords();
  } catch (err) {
    console.error('Failed to add password:', err);
    alert('Failed to add password entry');
  }
};

const deletePassword = async (id: string) => {
  if (confirm('Are you sure you want to delete this password entry? This will unlink it from any bookmarks.')) {
    try {
      await apiService.deletePassword(id);
      fetchPasswords();
    } catch (err) {
      console.error('Failed to delete password:', err);
    }
  }
};

const copyToClipboard = (text: string) => {
  navigator.clipboard.writeText(text);
};

onMounted(fetchPasswords);
</script>

<template>
    <div class="password-explorer-container">
      <!-- Sidebar Categories as Folders -->
      <div class="tree-sidebar">
        <div 
          class="sidebar-header-root" 
          :class="{ active: currentCategory === 'All' }"
          @click="currentCategory = 'All'"
        >
          🔒 All Credentials
        </div>
        <div class="tree-body">
          <div 
            v-for="cat in categories" 
            :key="cat" 
            class="category-node"
            :class="{ active: currentCategory === cat }"
            @click="currentCategory = cat"
          >
            <span class="type-icon">📁</span>
            {{ cat }}
          </div>
        </div>
      </div>

      <!-- Main Workspace -->
      <div class="main-panel">
        <div class="section-header">
          <div class="title-group">
            <h3>🔑 Password Vault</h3>
            <p class="subtitle">Securely manage your accounts and credentials</p>
          </div>
          <button class="add-btn" @click="showAddModal = true">
            <span>+</span> Add Credential
          </button>
        </div>

        <div v-if="isLoading" class="loading-state">
          <div class="spinner"></div>
          <p>Loading your vault...</p>
        </div>
        
        <div v-else-if="filteredPasswords.length === 0" class="empty-state">
          <div class="empty-icon">🔓</div>
          <p>No credentials found in this category.</p>
        </div>

        <div v-else class="password-grid">
          <div v-for="pw in filteredPasswords" :key="pw.id" class="password-card card">
            <div class="card-header">
              <div class="favicon-wrapper">🔑</div>
              <span class="category-tag">{{ pw.category }}</span>
            </div>
            <div class="card-body">
              <h4 class="pw-title">{{ pw.siteName }}</h4>
              <p class="pw-account">{{ pw.account }}</p>
            </div>
            <div class="card-actions">
              <button class="action-btn" @click="copyToClipboard(pw.account)">👤 Copy</button>
              <button class="action-btn" @click="copyToClipboard(pw.passwordRaw)">🔑 Pass</button>
              <button class="action-btn delete" @click="deletePassword(pw.id)">🗑️</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Password Modal -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
      <div class="modal-content card glow">
        <div class="modal-header">
          <h3>Add New Credential</h3>
          <p>This information is stored securely in your dashboard vault.</p>
        </div>
        
        <div class="form-grid">
          <div class="form-group">
            <label>Site/Service Name*</label>
            <input v-model="newPassword.siteName" placeholder="e.g. GitHub, Netflix" />
          </div>
          <div class="form-group">
            <label>Category</label>
            <select v-model="newPassword.category">
              <option v-for="cat in categories" :key="cat" :value="cat">{{ cat }}</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label>Account/Username*</label>
          <input v-model="newPassword.account" placeholder="e.g. toydogcat@gmail.com" />
        </div>

        <div class="form-group">
          <label>Password*</label>
          <input v-model="newPassword.passwordRaw" type="password" placeholder="••••••••" />
        </div>

        <div class="form-group">
          <label>Notes (Optional)</label>
          <textarea v-model="newPassword.notes" placeholder="Any extra info..."></textarea>
        </div>

        <div class="modal-actions">
          <button class="btn-cancel" @click="showAddModal = false">Cancel</button>
          <button class="btn-confirm" @click="handleAddPassword">Save to Vault</button>
        </div>
      </div>
    </div>
</template>

<style scoped>
/* Unified Explorer Layout - Matching Bookmarks and Snippets */
.password-explorer-container {
  display: flex;
  gap: 1.25rem;
  height: calc(100vh - 280px);
  min-height: 550px;
}

.tree-sidebar {
  width: 250px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  backdrop-filter: blur(10px);
}

.sidebar-header-root {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.75rem 1rem;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 800;
  transition: all 0.2s;
  color: var(--secondary-color);
  margin-bottom: 1.25rem;
  border-bottom: 1px solid var(--border-color);
}

.sidebar-header-root.active {
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
}

.category-node {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.8rem;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 0.95rem;
  margin-bottom: 0.2rem;
  color: var(--text-color);
  text-align: left;
}

.category-node:hover {
  background: rgba(255, 255, 255, 0.05);
}

.category-node.active {
  background: rgba(var(--primary-rgb), 0.15);
  color: var(--primary-color);
  font-weight: bold;
}

.main-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 1.5rem;
  padding-left: 1.2rem;
}

.password-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
  padding: 1.5rem;
  overflow-y: auto;
}

.password-card {
  height: 140px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  transition: all 0.3s;
}

.password-card:hover {
  border-color: #10b981;
  transform: translateY(-3px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.favicon-wrapper {
  font-size: 1.2rem;
}

.card-body {
  text-align: left;
}

.pw-title {
  font-size: 1.1rem;
  margin: 0;
  font-weight: 700;
}

.pw-account {
  font-size: 0.85rem;
  opacity: 0.6;
  font-family: monospace;
}

.card-actions {
  display: flex;
  gap: 0.5rem;
}

.action-btn {
  flex: 1;
  padding: 0.35rem;
  font-size: 0.8rem;
  border-radius: 6px;
  border: 1px solid rgba(255,255,255,0.1);
  background: rgba(255,255,255,0.05);
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background: #10b981;
  border-color: transparent;
  color: white;
}

.action-btn.delete {
  flex: 0 0 36px;
}

.action-btn.delete:hover {
  background: #ef4444;
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0,0,0,0.85);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2100;
}

.modal-content {
  width: 90%;
  max-width: 500px;
  padding: 2.5rem;
  background: #1e1e24;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
}

.modal-header {
  text-align: left;
  margin-bottom: 2rem;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.8rem;
}

.modal-header p {
  font-size: 0.9rem;
  opacity: 0.5;
  margin-top: 0.5rem;
}

.spinner {
  width: 30px;
  height: 30px;
  border: 3px solid rgba(16, 185, 129, 0.1);
  border-top-color: #10b981;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 1rem;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
}

.form-group {
  margin-bottom: 1.5rem;
  text-align: left;
}

.form-group label {
  display: block;
  font-size: 0.85rem;
  font-weight: 700;
  margin-bottom: 0.6rem;
  opacity: 0.8;
}

input, select, textarea {
  width: 100%;
  padding: 0.8rem;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0,0,0,0.2);
  color: white;
}

textarea {
  height: 80px;
  resize: vertical;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 2rem;
}

.btn-cancel {
  background: transparent;
  color: white;
  border: 1px solid rgba(255,255,255,0.1);
  padding: 0.8rem 1.5rem;
  border-radius: 10px;
}

.btn-confirm {
  background: #10b981;
  color: white;
  border: none;
  padding: 0.8rem 1.5rem;
  border-radius: 10px;
  font-weight: bold;
}

.loading-state, .empty-state {
  padding: 4rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  opacity: 0.5;
}

.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 600px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
