<script setup lang="ts">
import { ref, onMounted } from 'vue';
import AdminDashboard from '../components/AdminDashboard.vue';
import { apiService } from '../services/api';

const props = defineProps<{ 
  deviceId: string;
  adminEmail: string;
  userRole?: string;
  latency?: number | null;
}>();

const bulletinMessage = ref('');
const saving = ref(false);

onMounted(async () => {
  try {
    const res = await apiService.getBulletin();
    bulletinMessage.value = res.message;
  } catch (err) {
    console.error('Failed to load bulletin:', err);
  }
});

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
  <div class="admin-view">
    <AdminDashboard 
      :current-device-id="props.deviceId" 
      :user-role="props.userRole"
      :latency="props.latency"
      :admin-email="props.adminEmail"
    />
    
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
        <div class="char-count">{{ bulletinMessage.length }} characters</div>
      </div>

      <button @click="saveBulletin" :disabled="saving || !bulletinMessage.trim()" class="save-btn">
        <span v-if="saving" class="spinner"></span>
        {{ saving ? 'Updating...' : '📢 Publish Announcement' }}
      </button>
    </section>
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
</style>
