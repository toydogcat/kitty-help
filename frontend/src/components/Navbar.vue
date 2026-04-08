<script setup lang="ts">
import { computed } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useSyncStatus } from '../composables/useSyncStatus';
import { syncService } from '../services/syncService';

const { pendingCount } = useSyncStatus();

const props = defineProps<{ 
  isAdmin: boolean,
  isToby: boolean,
  userRole: string
}>();
const router = useRouter();
const route = useRoute();

const navItems = computed(() => {
  const role = props.userRole.toLowerCase();
  
  const items = [
    { name: 'home', label: 'Home', icon: '🏠', path: '/' },
  ];

  if (!props.userRole) {
    items.push({ name: 'login', label: 'Login', icon: '🔑', path: '/login_trigger' });
  }

  if (role !== 'visitor') {
    items.push({ name: 'chat', label: 'Chat', icon: '💬', path: '/chat' });
  }

  if (['vip', 'admin', 'superadmin', 'toby'].includes(role)) {
    items.push({ name: 'impression', label: 'Impress', icon: '🧠', path: '/impression' });
    items.push({ name: 'personal', label: 'Personal', icon: '📋', path: '/personal' });
    items.push({ name: 'desk', label: 'Desk', icon: '🖥️', path: '/desk' });
  }

  if (['admin', 'superadmin', 'toby'].includes(role)) {
    items.push({ name: 'storehouse', label: 'Store', icon: '📦', path: '/storehouse' });
    items.push({ name: 'obsidian', label: 'Vault', icon: '📑', path: '/obsidian' });
  }

  if (role === 'superadmin' || role === 'toby') {
    items.push({ name: 'admin', label: 'Admin', icon: '⚙️', path: '/admin' });
  }

  return items;
});

const currentRouteName = computed(() => route.name);

const emit = defineEmits(['login-requested']);

const navigate = (path: string) => {
  if (path === '/login_trigger') {
    emit('login-requested');
    return;
  }
  router.push(path);
};

const emergencyReset = async () => {
    await syncService.purgeDatabase();
};

const manualSync = async () => {
    await syncService.syncNow();
};
</script>

<template>
  <nav class="navbar">
    <div class="nav-container">
      <button 
        v-for="item in navItems" 
        :key="item.name"
        @click="navigate(item.path)"
        :class="['nav-item', { active: currentRouteName === item.name }]"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </button>
    </div>
    <div v-if="pendingCount > 0" class="sync-status-floating">
      <button @click="manualSync" class="settle-btn" :title="`點擊結帳: ${pendingCount} 個項目`" :disabled="syncService.isProcessing">
        <span class="sync-icon">{{ syncService.isProcessing ? '🔄' : '💳' }}</span>
        <span class="sync-count">{{ pendingCount }}</span>
      </button>
      <button @click="emergencyReset" class="emergency-btn" title="緊急重置本地資料">🆘</button>
    </div>
    <div v-else class="emergency-sync-static">
       <button @click="emergencyReset" class="emergency-btn-small" title="重置本地資料">🆘</button>
    </div>
  </nav>
</template>

<style scoped>
.navbar {
  position: fixed;
  bottom: 1.5rem;
  left: 50%;
  transform: translateX(-50%);
  z-index: 1000;
  width: auto;
}

.nav-container {
  display: flex;
  background: rgba(var(--primary-rgb), 0.1);
  backdrop-filter: blur(15px);
  padding: 0.5rem;
  border-radius: 20px;
  border: 1px solid rgba(var(--primary-rgb), 0.3);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4);
  gap: 0.5rem;
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0.6rem 1.2rem;
  background: transparent;
  border: none;
  border-radius: 15px;
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  min-width: 80px;
  opacity: 0.6;
}

.nav-item:hover {
  opacity: 1;
  background: rgba(var(--primary-rgb), 0.1);
}

.nav-item.active {
  opacity: 1;
  background: var(--primary-color);
  color: white;
  box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.4);
}

.nav-icon {
  font-size: 1.4rem;
  margin-bottom: 2px;
}

.nav-label {
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.sync-status-floating {
  position: absolute;
  right: -80px; top: 50%;
  transform: translateY(-50%);
  display: flex; align-items: center; gap: 0.5rem;
  background: rgba(var(--primary-rgb), 0.2);
  border: 1px solid var(--primary-color);
  padding: 0.5rem 0.8rem; border-radius: 100px;
  color: white; font-weight: bold; font-size: 0.9rem;
  backdrop-filter: blur(10px);
  animation: slideIn 0.3s ease;
  z-index: 1001;
}

.settle-btn {
  display: flex; align-items: center; gap: 0.5rem;
  background: var(--primary-color);
  border: none; border-radius: 50px;
  padding: 0.4rem 0.8rem;
  color: white; font-weight: bold;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 4px 10px rgba(var(--primary-rgb), 0.3);
}

.settle-btn:hover:not(:disabled) {
  transform: scale(1.05);
  filter: brightness(1.2);
}

.settle-btn:disabled {
  opacity: 0.7;
  cursor: wait;
}

.emergency-btn, .emergency-btn-small {
  background: rgba(255, 0, 0, 0.2);
  border: 1px solid rgba(255, 0, 0, 0.4);
  padding: 2px 6px;
  border-radius: 50%;
  cursor: pointer;
  font-size: 1rem;
  transition: all 0.2s;
}

.emergency-btn:hover, .emergency-btn-small:hover {
  background: red;
  transform: scale(1.2);
}

.emergency-sync-static {
  position: absolute;
  right: -40px; top: 50%;
  transform: translateY(-50%);
}

.sync-icon {
  display: inline-block;
  animation: spin 2s linear infinite;
}

@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
@keyframes slideIn { from { opacity: 0; transform: translateY(-50%) translateX(-20px); } to { opacity: 1; transform: translateY(-50%) translateX(0); } }

@media (max-width: 900px) {
  .sync-status-floating {
    right: 1rem; top: -60px; transform: none;
  }
}

@media (max-width: 600px) {
  .navbar {
    bottom: 0px;
    left: 0;
    right: 0;
    transform: none;
    width: 100%;
  }
  .nav-container {
    border-radius: 0;
    justify-content: space-around;
    padding: 0.5rem 0.2rem calc(0.5rem + env(safe-area-inset-bottom));
    border-left: none;
    border-right: none;
    border-bottom: none;
  }
}
</style>
