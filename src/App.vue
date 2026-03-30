<script setup lang="ts">
import { onMounted, ref, computed } from 'vue';
import { useTheme } from './composables/useTheme';
import { auth, googleProvider, signInWithPopup, getRedirectResult, signOut } from './firebaseConfig';
import { onAuthStateChanged } from 'firebase/auth';
import { apiService, socket } from './services/api';
import { v4 as uuidv4 } from 'uuid';

import Navbar from './components/Navbar.vue';

// Initialize theme
useTheme();

const loading = ref(true);
const deviceId = ref(localStorage.getItem('kitty_device_id') || '');
const deviceStatus = ref<'pending' | 'approved' | 'revoked' | 'unknown'>('unknown');
const userRole = ref<string>('user');
const userName = ref<string>('');
const showAdminLogin = ref(false);
const adminUser = ref<any>(null);

// Performance Monitoring State
const latency = ref<number | null>(null);

const ADMIN_EMAIL = 'toydogcat@gmail.com';

// Initialize Font Size from localStorage
onMounted(() => {
  const savedSize = localStorage.getItem('kitty_font_size') || 'normal';
  document.documentElement.setAttribute('data-font-size', savedSize);
});

onMounted(async () => {
  // 0. Handle Redirect Result (from Google login)
  try {
    const result = await getRedirectResult(auth);
    if (result?.user && result.user.email === ADMIN_EMAIL) {
      adminUser.value = result.user;
    }
  } catch (error) {
    console.error("Redirect login handle failed:", error);
  }

  // 1. Handle Device Identity
  if (!deviceId.value) {
    deviceId.value = uuidv4();
    localStorage.setItem('kitty_device_id', deviceId.value);
  }

  // 2. Register/Check device status with our PG backend
  try {
    const device = await apiService.registerDevice(deviceId.value, navigator.userAgent);
    deviceStatus.value = device.status;
    userRole.value = device.user_role || 'user';
    userName.value = device.user_name || '';
  } catch (err) {
    console.error("Failed to connect to backend:", err);
  } finally {
    loading.value = false;
  }

  // 3. Listen for status updates via Socket.io
  socket.on('deviceStatusUpdate', (data) => {
    if (data.id === deviceId.value) {
      deviceStatus.value = data.status;
    }
  });

  // 4. Handle Admin Session
  onAuthStateChanged(auth, (currentUser) => {
    if (currentUser && currentUser.email === ADMIN_EMAIL) {
      adminUser.value = currentUser;
      userRole.value = 'admin';
    } else {
      adminUser.value = null;
    }
  });

  // 5. Latency Monitoring Heartbeat
  setInterval(() => {
    socket.emit('clientPing', { time: Date.now() });
  }, 5000);

  socket.on('serverPong', (data) => {
    const now = Date.now();
    // Round-trip latency
    latency.value = now - data.clientTime;
  });
});

const loginAsAdmin = async () => {
  try {
    const result = await signInWithPopup(auth, googleProvider);
    if (result.user && result.user.email === ADMIN_EMAIL) {
      adminUser.value = result.user;
      userRole.value = 'admin';
    }
  } catch (error) {
    console.error("Admin login failed:", error);
  }
};

const logout = () => {
  signOut(auth).then(() => {
    adminUser.value = null;
  });
};

const isAdminUI = computed(() => {
  const role = (userRole.value || '').toLowerCase();
  const isAdminEmail = adminUser.value?.email === ADMIN_EMAIL;
  return isAdminEmail || role === 'admin' || role === 'subadmin';
});
</script>

<template>
  <div class="container">
    <header>
      <div class="brand">
        <h1>🐱 kitty-help</h1>
        <p>Cross-device Auxiliary Communication (PG Edition)</p>
      </div>
      <div v-if="adminUser || userRole === 'admin' || userRole === 'subadmin'" class="admin-auth-info">
        <span class="badge admin-badge">{{ (userRole || 'USER').toUpperCase() }}</span>
        <span class="email">{{ adminUser?.email || userName }}</span>
        <button v-if="adminUser" @click="logout" class="logout-btn">Logout</button>
      </div>
    </header>

    <main v-if="!loading">
      <!-- App Router Content (only for approved devices or admin) -->
      <template v-if="isAdminUI || deviceStatus === 'approved'">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component 
              :is="Component" 
              :device-id="deviceId" 
              :admin-email="adminUser?.email" 
              :user-role="userRole"
              :latency="latency"
            />
          </transition>
        </router-view>
        <Navbar :is-admin="isAdminUI" />
      </template>

      <!-- Unauthorized View -->
      <template v-else>
        <div v-if="deviceStatus === 'pending'" class="card welcome-card">
          <h2>Welcome to kitty-help</h2>
          <p>Your Device ID: <code>{{ deviceId.substring(0, 8) }}...</code></p>
          <div class="status-badge pending">
            ⏳ Waiting for Administrator Approval
          </div>
          <p class="hint">Ask the admin to approve this device ID in the dashboard.</p>
          
          <div class="admin-entry">
            <button @click="showAdminLogin = !showAdminLogin" class="text-btn">Admin Entry</button>
            <div v-if="showAdminLogin" class="admin-login-box">
              <button @click="loginAsAdmin" class="google-btn">Login with Google</button>
            </div>
          </div>
        </div>

        <div v-else-if="deviceStatus === 'revoked'" class="card welcome-card">
          <h2>Access Denied</h2>
          <p>This device has been revoked access.</p>
        </div>

        <div v-else class="card welcome-card">
          <h2>Connection Error</h2>
          <p>Unable to reach the backend server (PostgreSQL).</p>
          <p class="error-text">Please make sure the Node.js server is running on port 3000.</p>
          <div class="admin-entry">
            <button @click="showAdminLogin = !showAdminLogin" class="text-btn">Admin Entry</button>
            <div v-if="showAdminLogin" class="admin-login-box">
              <button @click="loginAsAdmin" class="google-btn">Login with Google</button>
            </div>
          </div>
        </div>
      </template>
    </main>

    <main v-else>
      <div class="loader-container">
        <div class="spinner-large"></div>
        <div class="loader-text">Connecting to kitty-help backend...</div>
      </div>
    </main>
  </div>
</template>

<style>
@import './assets/themes.css';

/* Global Font Size System */
:root {
  --base-font-size: 16px;
}
html[data-font-size="small"] { --base-font-size: 14px; }
html[data-font-size="normal"] { --base-font-size: 16px; }
html[data-font-size="large"] { --base-font-size: 18px; }
html[data-font-size="xlarge"] { --base-font-size: 22px; }
html[data-font-size="huge"] { --base-font-size: 28px; }

html {
  font-size: var(--base-font-size);
}

body {
  margin: 0;
  padding: 1rem;
  font-family: 'Inter', -apple-system, sans-serif;
  background-color: var(--bg-color);
  color: var(--text-color);
  transition: background-color 0.3s, color 0.3s;
  line-height: 1.5;
}

.container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding-bottom: 8rem; /* Space for bottom navbar */
  min-height: 100vh;
}

header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 0;
  border-bottom: 1px solid rgba(var(--primary-rgb), 0.1);
}

header h1 {
  font-size: 2.2rem;
  margin: 0;
  color: var(--primary-color);
}

header p {
  margin: 0;
  font-size: 0.9rem;
  opacity: 0.7;
}

.admin-auth-info {
  display: flex;
  align-items: center;
  gap: 1rem;
  background: rgba(var(--primary-rgb), 0.05);
  padding: 0.5rem 1rem;
  border-radius: 12px;
}

.admin-badge {
  background: var(--primary-color);
  color: white;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.7rem;
  font-weight: 800;
}

/* Page Transitions */
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.page-fade-enter-from {
  opacity: 0;
  transform: translateX(10px);
}

.page-fade-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}

.loader-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin-top: 8rem;
  gap: 1.5rem;
}

.spinner-large {
  width: 50px;
  height: 50px;
  border: 4px solid rgba(var(--primary-rgb), 0.1);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
