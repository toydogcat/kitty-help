<script setup lang="ts">
import { ref, onMounted, watch, defineAsyncComponent } from 'vue';
const BookmarkGrid = defineAsyncComponent(() => import('../components/BookmarkGrid.vue'));
const PasswordVault = defineAsyncComponent(() => import('../components/PasswordVault.vue'));
const SecurityTimer = defineAsyncComponent(() => import('../components/SecurityTimer.vue'));
const SecurityModal = defineAsyncComponent(() => import('../components/SecurityModal.vue'));
const ClipBoard = defineAsyncComponent(() => import('../components/ClipBoard.vue'));
const Bookcase = defineAsyncComponent(() => import('../components/Bookcase.vue'));
import { auth } from '../firebaseConfig';
import { onAuthStateChanged } from 'firebase/auth';
import { apiService } from '../services/api';

const currentUser = ref<any>(null);
const loading = ref(true);
const deviceId = ref('');
const hasSecurityTrust = ref(false);
const showSecurityModal = ref(false);
const activeTab = ref(localStorage.getItem('personal_active_tab') || 'bookmarks');

const props = defineProps<{
  isToby?: boolean;
}>();

const getDeviceId = () => {
  let id = localStorage.getItem('kitty_device_id');
  if (!id) {
    id = Math.random().toString(36).substring(2, 11) + Date.now().toString(36).substring(8);
    localStorage.setItem('kitty_device_id', id);
  }
  return id;
};

const verifyStatus = async () => {
  if (!currentUser.value) return;
  try {
    const res = await apiService.getSecurityStatus(currentUser.value.uid, deviceId.value);
    hasSecurityTrust.value = res.data?.status === 'granted';
  } catch (err) {
    console.warn('Security status check failed');
  }
};

const handleVerified = () => {
  hasSecurityTrust.value = true;
  showSecurityModal.value = false;
};

// Add watcher for activeTab
watch(activeTab, (newVal) => {
  localStorage.setItem('personal_active_tab', newVal);
});

onMounted(() => {
  deviceId.value = getDeviceId();
  onAuthStateChanged(auth, (user) => {
    currentUser.value = user;
    loading.value = false;
    if (user) verifyStatus();
  });
});
</script>

<template>
  <div class="personal-view">
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <p>Loading your workspace...</p>
    </div>
    
    <template v-else-if="currentUser">
      <div class="personal-header">
        <div class="welcome-section">
          <h1>Hi, <span class="gradient-text">{{ isToby || currentUser.email?.includes('toymsi') ? 'Master Admin' : (currentUser.displayName || 'User') }}</span></h1>
          <p class="subtitle">Your private workspace and secure storage</p>
        </div>

        <SecurityTimer 
          v-if="currentUser && deviceId"
          :user-id="currentUser.uid" 
          :device-id="deviceId" 
          @expired="hasSecurityTrust = false"
          @granted="hasSecurityTrust = true"
        />
      </div>

      <!-- Tab Navigation -->
      <div class="tab-navigation">
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'bookmarks' }"
          @click="activeTab = 'bookmarks'"
        >
          🌟 Cloud Bookmarks
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'passwords' }"
          @click="activeTab = 'passwords'"
        >
          🔑 Password Vault
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'clipboard' }"
          @click="activeTab = 'clipboard'"
        >
          📋 Clipboard
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'bookcase' }"
          @click="activeTab = 'bookcase'"
        >
          📚 Bookcase
        </button>
      </div>

      <!-- Unified Explorer Area -->
      <div class="explorer-body" :class="{ 'no-padding': activeTab === 'bookcase' }">
        <Transition name="fade-slide" mode="out-in">
          <div :key="activeTab" class="unified-content">
            <BookmarkGrid 
              v-if="activeTab === 'bookmarks'" 
              :user-id="currentUser.uid"
              :has-security-trust="hasSecurityTrust"
              :device-id="deviceId"
              @request-verify="showSecurityModal = true"
            />
            <PasswordVault 
              v-else-if="activeTab === 'passwords'" 
              :user-id="currentUser.uid"
            />
            <ClipBoard 
              v-else-if="activeTab === 'clipboard'" 
              :is-toby="isToby || currentUser.email?.includes('toymsi')" 
              :device-id="deviceId" 
            />
            <Bookcase 
              v-else-if="activeTab === 'bookcase'"
              :user-id="currentUser.uid"
            />
          </div>
        </Transition>
      </div>
      
      <!-- Security Challenge Modal -->
      <SecurityModal 
        v-if="showSecurityModal"
        :user-id="currentUser.uid"
        :device-id="deviceId"
        @granted="handleVerified"
        @close="showSecurityModal = false"
      />
    </template>

    <div v-else class="login-prompt card glow">
      <div class="lock-icon">🔒</div>
      <h2>Secure Access Required</h2>
      <p>Please log in to access your personal dashboard and secrets.</p>
      <router-link to="/" class="btn-primary">Go to Login</router-link>
    </div>
  </div>
</template>

<style scoped>
.personal-view {
  padding: 2rem;
  max-width: 1400px;
  margin: 0 auto;
  min-height: 100vh;
  animation: fadeIn 0.8s ease-out;
}

.personal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2.5rem;
}

.welcome-section {
  text-align: left;
}

.welcome-section h1 {
  font-size: 2.8rem;
  margin: 0;
  font-weight: 800;
  letter-spacing: -1px;
}

.gradient-text {
  background: linear-gradient(135deg, var(--primary-color) 0%, #a78bfa 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  filter: drop-shadow(0 0 10px rgba(139, 92, 246, 0.3));
}

.subtitle {
  margin: 0.5rem 0 0 0;
  opacity: 0.5;
  font-size: 1.1rem;
}

/* Tab Navigation */
.tab-navigation {
  display: flex;
  gap: 0.8rem;
  margin-bottom: 1.5rem;
  padding: 0.4rem;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 16px;
  width: fit-content;
  border: 1px solid rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(10px);
}

.tab-btn {
  padding: 0.8rem 1.6rem;
  border-radius: 12px;
  border: none;
  background: transparent;
  color: white;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  gap: 0.8rem;
  opacity: 0.5;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  opacity: 0.8;
}

.tab-btn.active {
  background: rgba(255, 255, 255, 0.1);
  color: var(--primary-color);
  opacity: 1;
  box-shadow: 0 4px 20px rgba(0,0,0,0.3);
}

.explorer-body {
  flex: 1;
  background: rgba(255, 255, 255, 0.01);
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  margin-top: 1.5rem;
  overflow: hidden;
  position: relative;
  padding: 2rem;
  display: flex;
  flex-direction: column;
}

.explorer-body.no-padding {
  padding: 0;
  border: none;
  background: transparent;
}

.unified-content {
  height: 100%;
  animation: slideIn 0.4s ease-out;
}

@keyframes slideIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Loading & Login Prompt */
.loading-state {
  height: 60vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1.5rem;
  opacity: 0.7;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid rgba(139, 92, 246, 0.1);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.login-prompt {
  margin: 10vh auto;
  max-width: 450px;
  padding: 3.5rem;
  text-align: center;
}

.lock-icon {
  font-size: 4rem;
  margin-bottom: 1.5rem;
  filter: drop-shadow(0 0 15px rgba(139, 92, 246, 0.5));
}

.btn-primary {
  display: inline-block;
  margin-top: 1.5rem;
  padding: 1rem 2rem;
  background: var(--primary-color);
  color: white;
  text-decoration: none;
  border-radius: 12px;
  font-weight: 700;
  box-shadow: 0 8px 25px rgba(139, 92, 246, 0.4);
  transition: all 0.3s;
}

.btn-primary:hover {
  transform: translateY(-3px);
  box-shadow: 0 12px 30px rgba(139, 92, 246, 0.5);
}

/* Transitions */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .personal-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 2rem;
  }
  
  .welcome-section h1 {
    font-size: 2rem;
  }
}
</style>
