<script setup lang="ts">
import { ref, onMounted } from 'vue';
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

const categories = ['General', 'Work', 'Social', 'Admin', 'Finance', 'Game'];

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
  <div class="password-vault">
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
    
    <div v-else-if="!passwords || passwords.length === 0" class="empty-state">
      <div class="empty-icon">🔓</div>
      <p>No credentials saved yet. Start by adding one!</p>
    </div>

    <div v-else class="password-list">
      <div v-for="pw in passwords" :key="pw.id" class="password-item card">
        <div class="pw-info">
          <div class="pw-main">
            <span class="pw-site">{{ pw.siteName }}</span>
            <span class="pw-category">{{ pw.category }}</span>
          </div>
          <div class="pw-account">{{ pw.account }}</div>
        </div>
        <div class="pw-actions">
          <button class="icon-btn" title="Copy Account" @click="copyToClipboard(pw.account)">👤</button>
          <button class="icon-btn" title="Copy Password" @click="copyToClipboard(pw.passwordRaw)">🔑</button>
          <button class="icon-btn delete" title="Delete" @click="deletePassword(pw.id)">🗑️</button>
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
  </div>
</template>

<style scoped>
.password-vault {
  animation: fadeIn 0.4s ease-out;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 2rem;
  border-left: 5px solid #10b981;
  padding-left: 1.2rem;
}

.title-group {
  text-align: left;
}

.section-header h3 {
  color: var(--text-color);
  margin: 0;
  font-size: 1.5rem;
}

.subtitle {
  font-size: 0.85rem;
  opacity: 0.5;
  margin: 0.2rem 0 0 0;
}

.add-btn {
  background: #10b981;
  color: white;
  border: none;
  padding: 0.6rem 1.2rem;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.2s;
  box-shadow: 0 4px 15px rgba(16, 185, 129, 0.3);
}

.add-btn:hover {
  transform: translateY(-2px);
  filter: brightness(1.1);
  box-shadow: 0 6px 20px rgba(16, 185, 129, 0.4);
}

.password-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
}

.password-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.2rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  transition: all 0.2s;
}

.password-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: #10b981;
}

.pw-info {
  text-align: left;
}

.pw-main {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  margin-bottom: 0.3rem;
}

.pw-site {
  font-weight: 700;
  font-size: 1.1rem;
}

.pw-category {
  font-size: 0.65rem;
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
  padding: 0.2rem 0.6rem;
  border-radius: 10px;
  text-transform: uppercase;
  font-weight: 800;
}

.pw-account {
  font-size: 0.85rem;
  opacity: 0.6;
  font-family: monospace;
}

.pw-actions {
  display: flex;
  gap: 0.5rem;
}

.icon-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
}

.icon-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  transform: scale(1.1);
}

.icon-btn.delete:hover {
  background: rgba(239, 68, 68, 0.2);
  color: #ef4444;
  border-color: #ef4444;
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
