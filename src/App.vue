<script setup lang="ts">
import { onMounted, ref } from 'vue';
import StyleSwitcher from './components/StyleSwitcher.vue';
import AdminDashboard from './components/AdminDashboard.vue';
import ClipBoard from './components/ClipBoard.vue';
import { useTheme } from './composables/useTheme';
import { auth, googleProvider, signInWithPopup, signOut } from './firebaseConfig';
import { onAuthStateChanged } from 'firebase/auth';
import { apiService, socket } from './services/api';
import { v4 as uuidv4 } from 'uuid';

// Initialize theme
useTheme();

const loading = ref(true);
const deviceId = ref(localStorage.getItem('kitty_device_id') || '');
const deviceStatus = ref<'pending' | 'approved' | 'revoked' | 'unknown'>('unknown');
const showAdminLogin = ref(false);
const adminUser = ref<any>(null);

const ADMIN_EMAIL = 'toydogcat@gmail.com';

onMounted(async () => {
  // 1. Handle Device Identity
  if (!deviceId.value) {
    deviceId.value = uuidv4();
    localStorage.setItem('kitty_device_id', deviceId.value);
  }

  // 2. Register/Check device status with our PG backend
  try {
    const device = await apiService.registerDevice(deviceId.value, navigator.userAgent);
    deviceStatus.value = device.status;
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

  // 4. Handle Admin Session (Still using Google Auth for convenience)
  onAuthStateChanged(auth, (currentUser) => {
    if (currentUser && currentUser.email === ADMIN_EMAIL) {
      adminUser.value = currentUser;
    } else {
      adminUser.value = null;
    }
  });
});

const loginAsAdmin = async () => {
  try {
    await signInWithPopup(auth, googleProvider);
  } catch (error) {
    console.error("Admin login failed:", error);
  }
};

const logout = () => {
  signOut(auth).then(() => {
    adminUser.value = null;
  });
};
</script>

<template>
  <div class="container">
    <header>
      <h1>🐱 kitty-help</h1>
      <p>Cross-device Auxiliary Communication (PG Edition)</p>
      <div v-if="adminUser" class="admin-bar">
        <span>Admin: {{ adminUser.email }}</span>
        <button @click="logout" class="logout-btn">Logout</button>
      </div>
    </header>

    <main v-if="!loading">
      <StyleSwitcher />
      
      <!-- Admin View -->
      <template v-if="adminUser">
        <AdminDashboard />
        <ClipBoard :device-id="deviceId" />
      </template>

      <template v-else>
        <!-- App Content (only for approved devices) -->
        <div v-if="deviceStatus === 'approved'" class="app-container">
          <ClipBoard :device-id="deviceId" />
        </div>

        <!-- Unauthorized View -->
        <div v-else-if="deviceStatus === 'pending'" class="card welcome-card">
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

        <!-- Fallback for connection issues -->
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
      <div class="loader">Connecting to kitty-help backend...</div>
    </main>
  </div>
</template>

<style>
@import './assets/themes.css';

.container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  padding-bottom: 5rem;
}

header h1 {
  font-size: 3rem;
  margin-bottom: 0.5rem;
  color: var(--primary-color);
}

.welcome-card {
  margin-top: 2rem;
  max-width: 600px;
  margin-left: auto;
  margin-right: auto;
}

.status-badge {
  display: inline-block;
  padding: 0.5rem 1rem;
  border-radius: 999px;
  font-weight: bold;
  margin-top: 1rem;
}

.status-badge.pending {
  background-color: var(--secondary-color);
  color: white;
}

.loader {
  font-size: 1.2rem;
  color: var(--secondary-color);
  margin-top: 5rem;
}

code {
  background-color: var(--border-color);
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  font-family: monospace;
  word-break: break-all;
}

.hint {
  font-size: 0.9rem;
  color: var(--secondary-color);
  margin-top: 1rem;
}

.admin-entry {
  margin-top: 2rem;
  border-top: 1px solid var(--border-color);
  padding-top: 1rem;
}

.text-btn {
  background: none;
  border: none;
  color: var(--secondary-color);
  text-decoration: underline;
  cursor: pointer;
  font-size: 0.8rem;
  opacity: 0.5;
}

.text-btn:hover {
  opacity: 1;
}

.error-text {
  color: #ef4444;
  font-size: 0.9rem;
  margin: 1rem 0;
}

.admin-bar {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 1rem;
  margin-top: 1rem;
  font-size: 0.9rem;
}

.logout-btn {
  background: none;
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.2rem 0.6rem;
  border-radius: 4px;
  cursor: pointer;
}

.admin-login-box {
  margin-top: 1rem;
}

.google-btn {
  background-color: white;
  color: #757575;
  border: 1px solid #ddd;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-weight: bold;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.google-btn:hover {
  background-color: #f5f5f5;
}
</style>
