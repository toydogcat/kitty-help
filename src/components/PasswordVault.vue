<script setup lang="ts">
import { ref, onMounted, computed, onUnmounted } from 'vue';
import { apiService } from '../services/api';
import { usePin } from '../composables/usePin';

const props = defineProps<{
  userId: string;
}>();

const { pinToDesk } = usePin();

const passwords = ref<any[]>([]);
const isLoading = ref(false);
const showAddModal = ref(false);
const totpStatus = ref({ enabled: false, verified: false, verifiedUntil: 0 });
const showTOTPModal = ref(false);
const showSetupModal = ref(false);
const totpCode = ref('');
const setupData = ref<any>(null);
const totpError = ref('');

const checkTOTP = async () => {
  try {
    const status = await apiService.getTOTPStatus();
    totpStatus.value = status;
    if (status.enabled && status.verified) {
      if (passwords.value.length === 0) fetchPasswords();
    }
  } catch (err) {
    console.warn('[Vault] 2FA check failed');
  }
};

let statusTimer: any = null;
onMounted(() => {
  checkTOTP();
  statusTimer = setInterval(checkTOTP, 30000); // Check every 30s
});
onUnmounted(() => { if (statusTimer) clearInterval(statusTimer); });
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
  if (!totpStatus.value.verified) return;
  isLoading.value = true;
  try {
    const res = await apiService.getPasswords();
    passwords.value = res || [];
  } catch (err: any) {
    if (err.response?.status === 403) {
      totpStatus.value.verified = false;
    }
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
  if (!totpStatus.value.verified) {
    showTOTPModal.value = true;
    return;
  }
  navigator.clipboard.writeText(text);
};

const handlePin = async (pw: any) => {
  try {
    await pinToDesk('password', pw.id);
    alert(`Pinned ${pw.siteName} to Desk!`);
  } catch (err) {
    alert('Pin failed');
  }
};

const handleTOTPVerify = async () => {
  totpError.value = '';
  try {
    await apiService.verifyTOTP(totpCode.value);
    totpCode.value = '';
    showTOTPModal.value = false;
    await checkTOTP();
    fetchPasswords();
  } catch (err: any) {
    totpError.value = err.response?.data?.error || 'Verification failed';
  }
};

const handleStartSetup = async () => {
  try {
    const data = await apiService.setupTOTP();
    setupData.value = data;
    showSetupModal.value = true;
  } catch (err) {
    alert('Setup failed to initialize');
  }
};

const handleCompleteSetup = async () => {
  totpError.value = '';
  try {
    await apiService.enableTOTP(totpCode.value);
    totpCode.value = '';
    showSetupModal.value = false;
    setupData.value = null;
    await checkTOTP();
    fetchPasswords();
  } catch (err: any) {
    totpError.value = err.response?.data?.error || 'Verification failed';
  }
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

        <div v-if="!totpStatus.enabled" class="empty-state auth-wall">
          <div class="empty-icon">🛡️</div>
          <h3>2FA Activation Required</h3>
          <p>Google Authenticator is mandatory to access the Password Vault.</p>
          <button class="primary-btn-lg" @click="handleStartSetup" style="margin-top: 1rem">
            Set Up Now
          </button>
        </div>

        <div v-else-if="!totpStatus.verified" class="empty-state auth-wall">
          <div class="empty-icon">🔒</div>
          <h3>Vault Locked</h3>
          <p>Verification window expired. Please verify to continue.</p>
          <button class="primary-btn-lg" @click="showTOTPModal = true" style="margin-top: 1rem">
            Unlock Vault
          </button>
        </div>

        <div v-else-if="isLoading" class="loading-state">
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
              <div class="meta-actions">
                <button class="pin-badge" @click.stop="handlePin(pw)">📌</button>
                <span class="category-tag">{{ pw.category }}</span>
              </div>
            </div>
            <div class="card-body">
              <h4 class="pw-title">{{ pw.siteName }}</h4>
              <p class="pw-account">{{ pw.account }}</p>
            </div>
            <div class="card-actions">
              <button class="action-btn" @click="copyToClipboard(pw.account)">👤 ID</button>
              <button class="action-btn" @click="copyToClipboard(pw.passwordRaw)">🔑 PASS</button>
              <button class="action-btn delete" @click="deletePassword(pw.id)">🗑️</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 🔐 2FA SETUP MODAL -->
    <div v-if="showSetupModal" class="modal-overlay" @click.self="showSetupModal = false">
      <div class="modal-content card glow auth-setup">
        <div class="modal-header center">
          <h3>Link Google Authenticator</h3>
          <p>Secure your sensitive information with 2FA.</p>
        </div>
        
        <div class="qr-placeholder" v-if="setupData">
          <div class="qr-box">
             <!-- Simplified QR: Use a high-quality Google Charts URL -->
             <img :src="`https://chart.googleapis.com/chart?chs=200x200&chld=M|0&cht=qr&chl=${encodeURIComponent(setupData.url)}`" alt="QR Code" />
          </div>
          <p class="secret-text">Secret Key: <code>{{ setupData.secret }}</code></p>
        </div>

        <div class="form-group center">
          <label>Enter 6-digit Code</label>
          <input v-model="totpCode" class="otp-input" placeholder="000 000" maxlength="6" @keyup.enter="handleCompleteSetup" />
          <p v-if="totpError" class="error-msg">{{ totpError }}</p>
        </div>

        <div class="modal-actions full">
          <button class="btn-confirm big" @click="handleCompleteSetup">Verify & Enable</button>
          <button class="btn-cancel" @click="showSetupModal = false">Cancel</button>
        </div>
      </div>
    </div>

    <!-- 🔐 2FA VERIFY MODAL -->
    <div v-if="showTOTPModal" class="modal-overlay" @click.self="showTOTPModal = false">
      <div class="modal-content card glow auth-verify">
        <div class="modal-header center">
          <div class="icon-circle">🔑</div>
          <h3>Vault Verification</h3>
          <p>Identity verification needed for this session.</p>
        </div>
        
        <div class="form-group center">
          <label>6-digit Verification Code</label>
          <input v-model="totpCode" class="otp-input" placeholder="000 000" maxlength="6" autofocus @keyup.enter="handleTOTPVerify" />
          <p v-if="totpError" class="error-msg">{{ totpError }}</p>
        </div>

        <div class="modal-actions full">
          <button class="btn-confirm big" @click="handleTOTPVerify">Unlock Now</button>
          <button class="btn-cancel" @click="showTOTPModal = false">Dismiss</button>
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

/* 🔐 Additional 2FA Components */
.auth-wall {
  background: rgba(var(--primary-rgb), 0.05) !important;
  border: 1px dashed rgba(var(--primary-rgb), 0.3) !important;
  opacity: 1 !important;
  border-radius: 20px;
}
.center { text-align: center !important; }
.modal-header.center { margin-bottom: 2rem; }
.icon-circle {
  font-size: 2.5rem;
  background: rgba(var(--primary-rgb), 0.1);
  width: 70px; height: 70px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 50%;
  margin: 0 auto 1.5rem;
}

.otp-input {
  font-size: 2rem !important;
  text-align: center !important;
  letter-spacing: 1rem;
  padding: 1rem !important;
  font-weight: 800;
  color: var(--primary-color) !important;
  background: rgba(0,0,0,0.3) !important;
  border: 2px solid var(--border-color) !important;
}

.modal-actions.full { flex-direction: column; width: 100%; }
.btn-confirm.big { width: 100%; padding: 1.2rem; font-size: 1.1rem; }

.qr-placeholder { margin-bottom: 2rem; }
.qr-box { 
  background: white; 
  padding: 1rem; 
  display: inline-block; 
  border-radius: 12px; 
  margin-bottom: 1rem;
}
.secret-text { font-size: 0.8rem; opacity: 0.6; }
.secret-text code { background: rgba(255,255,255,0.1); padding: 0.2rem 0.4rem; border-radius: 4px; }

.error-msg { color: #f87171; font-size: 0.85rem; margin-top: 0.8rem; font-weight: bold; }

.meta-actions { display: flex; align-items: center; gap: 0.5rem; }
.pin-badge {
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  font-size: 0.8rem;
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}
.pin-badge:hover { background: rgba(var(--primary-rgb), 0.2); border-color: var(--primary-color); }
</style>
