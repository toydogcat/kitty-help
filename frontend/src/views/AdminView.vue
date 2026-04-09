<script setup lang="ts">
import { ref, onMounted } from 'vue';
import AdminDashboard from '../components/AdminDashboard.vue';
import { apiService } from '../services/api';

const props = defineProps<{ 
  deviceId: string;
  adminEmail?: string;
  userRole?: string;
  latency?: number | null;
}>();

const bulletinMessage = ref('');
const visitorMode = ref(false);
const saving = ref(false);
const loading = ref(true);

// Bot Management State
const activeBotTab = ref('telegram');
const pendingRequests = ref<any[]>([]);
const authorizedUsers = ref<any[]>([]);
const bots = ref({
  telegram: { name: 'Telegram', icon: '✈️' },
  discord: { name: 'Discord', icon: '🎮' },
  line: { name: 'LINE', icon: '🟢' }
});

const fetchBotData = async () => {
  try {
    const [reqs, users] = await Promise.all([
      apiService.getBotRequests(),
      apiService.getBotUsers()
    ]);
    pendingRequests.value = reqs;
    authorizedUsers.value = users;
  } catch (err) {
    console.error('Failed to fetch bot data:', err);
  }
};

const selectedRoles = ref<Record<string, string>>({});

const handleBotApprove = async (id: string) => {
  const role = selectedRoles.value[id] || 'client';
  try {
    await apiService.approveBotUser(id, role);
    await fetchBotData();
  } catch (err) { alert('Approval failed'); }
};

const handleBotReject = async (id: string) => {
  try {
    await apiService.rejectBotUser(id);
    await fetchBotData();
  } catch (err) { alert('Rejection failed'); }
};

const handleDeleteBotUser = async (id: string) => {
  if (!confirm('Revoke account link? This will clear platform IDs from the user profile.')) return;
  try {
    await apiService.deleteBotUser(id);
    await fetchBotData();
  } catch (err) { alert('Revoke failed'); }
};

onMounted(async () => {
  try {
    const [bullRes, settings] = await Promise.all([
      apiService.getBulletin(),
      apiService.getSettings(),
      fetchBotData()
    ]);
    bulletinMessage.value = bullRes.message;
    visitorMode.value = settings.visitor_mode === 'true';
  } catch (err) {
    console.error('Failed to load admin data:', err);
  } finally {
    loading.value = false;
  }
});

const toggleVisitorMode = async () => {
  try {
    const newVal = !visitorMode.value;
    await apiService.updateSettings('visitor_mode', String(newVal));
    visitorMode.value = newVal;
  } catch (err) {
    alert('Failed to toggle visitor mode.');
  }
};

const saveBulletin = async () => {
  if (!bulletinMessage.value.trim()) return;
  saving.value = true;
  try {
    await apiService.updateBulletin(bulletinMessage.value, props.adminEmail, props.deviceId);
    alert('Bulletin updated successfully! 📢');
  } catch (err) {
    alert('Failed to update bulletin. Please check permissions.');
  } finally {
    saving.value = false;
  }
};
</script>

<template>
  <div class="admin-view" v-if="props.userRole === 'superadmin' || props.adminEmail === 'toydogcat@gmail.com'">
    <AdminDashboard 
      :current-device-id="props.deviceId" 
      :user-role="props.userRole"
      :latency="props.latency"
      :admin-email="props.adminEmail"
    />
    
    <section class="card settings-card">
      <div class="header-with-icon">
        <span class="header-icon">🌍</span>
        <h3>Public Access Control</h3>
      </div>
      <div class="control-row">
        <div class="control-text">
          <h4>Visitor Mode</h4>
          <p class="desc">When ON, guests can view your bulletin and snippets without registration.</p>
        </div>
        <label class="switch">
          <input type="checkbox" :checked="visitorMode" @change="toggleVisitorMode" />
          <span class="slider round"></span>
        </label>
      </div>
    </section>

    <!-- Bot Identity Management (Migrated from ChatView) -->
    <section class="card robot-management">
      <div class="header-with-icon">
        <span class="header-icon">🤖</span>
        <h3>Robot Platform Management</h3>
      </div>
      <p class="desc">Manage authorization requests and connected accounts for Telegram, Discord, and LINE.</p>
      
      <div class="bot-tabs">
        <button v-for="(bot, id) in bots" :key="id" @click="activeBotTab = id" :class="['mini-tab', { active: activeBotTab === id }]">
          {{ bot.icon }} {{ bot.name }}
        </button>
      </div>

      <div class="bot-control-content" v-if="(bots as any)[activeBotTab]">
        <div class="auth-box full-width">
          <div class="header-with-tag">
            <h5 class="section-title">🗝️ Pending Requests & Sign-ups</h5>
            <span class="count-badge">{{ pendingRequests.filter(r => r.platform === activeBotTab).length }}</span>
          </div>
          
          <div v-if="pendingRequests.filter(r => r.platform === activeBotTab).length === 0" class="empty-hint">
            No pending enrollment requests.
          </div>
          
          <div class="request-grid">
            <div v-for="req in pendingRequests.filter(r => r.platform === activeBotTab)" :key="req.id" class="request-card">
              <div class="req-header">
                <span class="platform-name"><strong>{{ req.account_name }}</strong></span>
                <span class="platform-id">ID: {{ req.account_id }}</span>
              </div>
              <div class="user-link-info" v-if="req.user_email">
                <span class="label">Google Identity:</span>
                <span class="email">{{ req.user_email }}</span>
                <span class="name">({{ req.user_name }})</span>
              </div>
              <div class="user-link-info" v-else>
                <span class="label status-warn">⚠️ User not yet linked to Google ID</span>
              </div>
              
              <div class="action-footer">
                <div class="role-assign">
                  <label>Role:</label>
                  <select v-model="selectedRoles[req.id]" class="mini-select">
                    <option value="client">Client</option>
                    <option value="vip">VIP</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
                <div class="mini-actions">
                  <button @click="handleBotApprove(req.id)" class="approve-btn-sm">Approve</button>
                  <button @click="handleBotReject(req.id)" class="reject-btn-sm">Reject</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="auth-box full-width">
          <h5 class="section-title">👥 Authorized Platform Accounts</h5>
          <div class="authorized-list">
            <div v-for="user in authorizedUsers.filter(u => u.platform === activeBotTab)" :key="user.id" class="auth-user-row">
              <div class="user-id-info">
                <strong>{{ user.account_name }}</strong>
                <span class="small-id">{{ user.account_id }}</span>
                <span class="role-tag" :class="user.role">{{ (user.role || 'user').toUpperCase() }}</span>
              </div>
              <button @click="handleDeleteBotUser(user.id)" class="mini-btn delete">Revoke Access</button>
            </div>
          </div>
          <div v-if="authorizedUsers.filter(u => u.platform === activeBotTab).length === 0" class="empty-hint">No authorized accounts for this platform.</div>
        </div>
      </div>
    </section>

    <section class="card bulletin-editor">
      <div class="header-with-icon">
        <span class="header-icon">✍️</span>
        <h3>Edit Bulletin Board</h3>
      </div>
      <p class="desc">Update the global announcement visible to all family members on the Home page.</p>
      
      <div class="input-group">
        <textarea 
          v-model="bulletinMessage" 
          placeholder="Type your family announcement here..."
          :disabled="saving"
        ></textarea>
        <div class="char-count">{{ (bulletinMessage || '').length }} characters</div>
      </div>

      <button @click="saveBulletin" :disabled="saving || !(bulletinMessage || '').trim()" class="save-btn">
        <span v-if="saving" class="spinner"></span>
        {{ saving ? 'Updating...' : '📢 Publish Announcement' }}
      </button>
    </section>
  </div>
  <div v-else-if="!loading" class="card error-card">
    <h2>🚫 Access Restricted</h2>
    <p>Only the super-administrator (toydogcat@gmail.com) can access this dashboard.</p>
  </div>
</template>

<style scoped>
.admin-view {
  animation: slideUp 0.6s ease-out;
}

@keyframes slideUp {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

.bulletin-editor {
  margin-top: 2rem;
  background: linear-gradient(135deg, var(--card-bg), rgba(var(--primary-rgb), 0.05));
}

.header-with-icon {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  margin-bottom: 0.5rem;
}

.header-icon {
  font-size: 1.5rem;
}

.desc {
  font-size: 0.9rem;
  color: var(--secondary-color);
  margin-bottom: 1.5rem;
}

.input-group {
  position: relative;
  margin-bottom: 1.5rem;
}

textarea {
  width: 100%;
  min-height: 180px;
  padding: 1.2rem;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-color);
  border: 1px solid var(--border-color);
  font-family: inherit;
  font-size: 1rem;
  line-height: 1.5;
  transition: all 0.3s;
  resize: vertical;
}

textarea:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 15px rgba(var(--primary-rgb), 0.2);
}

.char-count {
  position: absolute;
  bottom: 0.8rem;
  right: 1rem;
  font-size: 0.75rem;
  color: var(--secondary-color);
  opacity: 0.6;
}

.save-btn {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 1rem 2rem;
  border-radius: 10px;
  cursor: pointer;
  font-weight: 700;
  font-size: 1rem;
  transition: all 0.3s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.8rem;
  box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.3);
}

.save-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(var(--primary-rgb), 0.4);
}

.save-btn:active:not(:disabled) {
  transform: translateY(0);
}

.save-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.control-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 0;
}

.control-text h4 {
  margin: 0;
  color: var(--text-color);
}

.error-card {
  text-align: center;
  padding: 4rem 2rem;
  margin-top: 4rem;
}

.error-card h2 {
  color: #e74c3c;
  margin-bottom: 1rem;
}

/* Toggle Switch Style (Copied from ChatView) */
.switch {
  position: relative;
  display: inline-block;
  width: 50px;
  height: 24px;
}

.switch input { 
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--border-color);
  transition: .4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 4px;
  bottom: 3px;
  background-color: white;
  transition: .4s;
}

input:checked + .slider {
  background-color: var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(24px);
}

.slider.round {
  border-radius: 34px;
}

.slider.round:before {
  border-radius: 50%;
}
.bot-tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
}

.mini-tab {
  padding: 0.5rem 1rem;
  background: rgba(var(--primary-rgb), 0.1);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  color: var(--text-color);
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}

.mini-tab.active {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.bot-control-content {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.auth-box.full-width {
  width: 100%;
}

.section-title {
  font-size: 1rem;
  margin-bottom: 1rem;
  color: var(--primary-color);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.header-with-tag {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.count-badge {
  background: var(--primary-color);
  color: white;
  font-size: 0.75rem;
  padding: 0.2rem 0.6rem;
  border-radius: 99px;
  font-weight: bold;
}

.request-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
}

.request-card {
  background: rgba(var(--primary-rgb), 0.05);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 1.2rem;
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  transition: all 0.3s;
}

.request-card:hover {
  border-color: var(--primary-color);
  background: rgba(var(--primary-rgb), 0.08);
}

.req-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  padding-bottom: 0.5rem;
}

.platform-name { font-size: 1.1rem; }
.platform-id { font-size: 0.75rem; opacity: 0.5; font-family: monospace; }

.user-link-info {
  font-size: 0.85rem;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.user-link-info .label { opacity: 0.6; font-size: 0.75rem; }
.user-link-info .email { color: var(--primary-color); font-weight: bold; }
.user-link-info .status-warn { color: #e74c3c; font-style: italic; }

.action-footer {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding-top: 1rem;
  border-top: 1px solid rgba(255,255,255,0.05);
}

.role-assign {
  display: flex;
  align-items: center;
  gap: 1rem;
  font-size: 0.85rem;
}

.mini-select {
  background: rgba(0,0,0,0.3);
  color: white;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 0.3rem;
  flex: 1;
}

.mini-actions {
  display: flex;
  gap: 0.5rem;
}

.approve-btn-sm {
  flex: 2;
  background: #2ecc71;
  color: white;
  border: none;
  padding: 0.6rem;
  border-radius: 8px;
  font-weight: bold;
  cursor: pointer;
}

.reject-btn-sm {
  flex: 1;
  background: rgba(231, 76, 60, 0.1);
  color: #e74c3c;
  border: 1px solid #e74c3c;
  padding: 0.6rem;
  border-radius: 8px;
  cursor: pointer;
}

.authorized-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.auth-user-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.8rem 1.2rem;
  background: rgba(0,0,0,0.2);
  border-radius: 10px;
}

.user-id-info {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.small-id { font-size: 0.75rem; opacity: 0.4; font-family: monospace; }

.role-tag {
  font-size: 0.65rem;
  font-weight: 900;
  padding: 0.1rem 0.5rem;
  border-radius: 4px;
  text-transform: uppercase;
}

.role-tag.admin { background: #e74c3c; color: white; }
.role-tag.vip { background: #f1c40f; color: black; }
.role-tag.client { background: #3498db; color: white; }

.mini-btn.delete {
  background: transparent;
  border: 1px solid rgba(231, 76, 60, 0.5);
  color: #e74c3c;
  font-size: 0.75rem;
  padding: 0.3rem 0.8rem;
  border-radius: 6px;
  cursor: pointer;
}

.empty-hint {
  text-align: center;
  padding: 2rem;
  opacity: 0.4;
  font-style: italic;
  font-size: 0.9rem;
}
</style>
