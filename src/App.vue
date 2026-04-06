<script setup lang="ts">
import { onMounted, ref, computed } from 'vue';
import { useTheme } from './composables/useTheme';
import { auth, googleProvider, signInWithPopup, getRedirectResult, signOut } from './firebaseConfig';
import { onAuthStateChanged } from 'firebase/auth';
import { apiService, socket, setAuthToken } from './services/api';
import { v4 as uuidv4 } from 'uuid';


import Navbar from './components/Navbar.vue';

// Initialize theme
useTheme();

const loading = ref(true);
const deviceId = ref(localStorage.getItem('kitty_device_id') || '');
const deviceStatus = ref<'pending' | 'approved' | 'revoked' | 'unknown'>('unknown');
const userRole = ref<string>('user');
const userName = ref<string>('');

const adminUser = ref<any>(null);

// Bot Auth (Kitty-Auth) State
const joinToken = ref('');
const botRequestStatus = ref<'idle' | 'verifying' | 'submitted' | 'error'>('idle');
const botRequestInfo = ref<any>(null);
const botRequestError = ref('');

// Performance Monitoring State
const latency = ref<number | null>(null);

const ADMIN_EMAIL = import.meta.env.VITE_ADMIN_EMAIL || 'toydogcat@gmail.com';

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

  // 1.5 Handle Join Token from URL
  const urlParams = new URLSearchParams(window.location.search);
  const tokenFromUrl = urlParams.get('join');
  if (tokenFromUrl) {
    joinToken.value = tokenFromUrl;
    verifyToken();
  }

  // 2. Register/Check device status with our PG backend
  try {
    const device = await apiService.registerDevice(deviceId.value, navigator.userAgent);
    deviceStatus.value = device.status;
    userRole.value = device.userRole || 'user';
    localStorage.setItem('kitty_user_role', userRole.value);
    userName.value = device.userName || '';
  } catch (err) {
    console.error("Failed to connect to backend:", err);
  } finally {
    loading.value = false;
  }

  // 3. Listen for status updates via Socket.io
  socket.on('connect', () => {
    console.log("🟢 [Socket] Connected! ID:", socket.id);
    socket.emit('join', 'kitty-room');
    console.log("📢 [Socket] Joined 'kitty-room'");
  });

  socket.on('connect_error', (err: any) => {
    console.error("🔴 [Socket] Connection Error:", err.message);
    // @ts-ignore - Bypass private access for debug
    console.log("🔗 [Socket] Attempting URL:", (socket.io as any).uri || 'Check your .env');
  });

  socket.on('disconnect', (reason: any) => {
    console.warn("🟠 [Socket] Disconnected:", reason);
  });

  socket.on('deviceStatusUpdate', (data: any) => {
    console.log("📱 [Socket] Device Update received:", data);
    if (data.id === deviceId.value) {
      deviceStatus.value = data.status;
    }
  });

  onAuthStateChanged(auth, async (curr) => {
    if (curr) {
      adminUser.value = curr;
      // Force sync with backend to get correct role and set Axios headers
      try {
        const idToken = await curr.getIdToken();
        const backendAuth = await apiService.verifyToken(idToken);
        userRole.value = backendAuth.user.role || 'user';
        localStorage.setItem('kitty_user_role', userRole.value);
        userName.value = backendAuth.user.name || curr.displayName || '';
      } catch (err) {
        console.error("Backend session sync failed:", err);
      }
    } else {
      adminUser.value = null;
      userRole.value = '';
      localStorage.removeItem('kitty_user_role');
      setAuthToken(null);
    }
  });

  // 5. Latency Monitoring Heartbeat
  setInterval(() => {
    if (socket.connected) {
      const pingTime = Date.now();
      socket.emit('clientPing', { time: pingTime });
      // console.log("📡 [Heartbeat] Ping sent...");
    } else {
      // console.warn("❌ [Heartbeat] Skipped: Socket not connected");
    }
  }, 5000);

  socket.on('serverPong', (data: any) => {
    const now = Date.now();
    latency.value = now - data.clientTime;
    console.log(`⏱️ [Latency] Round-trip: ${latency.value}ms`);
  });
});

const loginAsAdmin = async () => {
  try {
    const result = await signInWithPopup(auth, googleProvider);
    if (result.user) {
      const idToken = await result.user.getIdToken();
      const backendAuth = await apiService.verifyToken(idToken);
      
      adminUser.value = result.user;
      userRole.value = backendAuth.user.role || 'user';
      localStorage.setItem('kitty_user_role', userRole.value);
      userName.value = backendAuth.user.name || result.user.displayName || '';
    }
  } catch (error) {
    console.error("Login failed:", error);
  }
};

const logout = () => {
  signOut(auth).then(() => {
    adminUser.value = null;
    setAuthToken(null);
  });
};

const verifyToken = async () => {
  if (!joinToken.value || joinToken.value.length < 8) return;
  botRequestStatus.value = 'verifying';
  botRequestError.value = '';
  
  try {
    // 1. Ensure user is logged in with Google first (Sign-up requirement)
    if (!auth.currentUser) {
      if (!confirm('To enroll, you must first link your Google account. Triggering Google login...')) {
        botRequestStatus.value = 'error';
        botRequestError.value = 'Google sign-in required for enrollment.';
        return;
      }
      await loginAsAdmin();
      if (!auth.currentUser) throw new Error('Google Sign-in failed or cancelled.');
    }
    
    // 2. Submit the 8-digit code with current session
    await apiService.linkBotAccount(joinToken.value, deviceId.value);
    
    botRequestStatus.value = 'submitted';
    botRequestInfo.value = { 
      name: 'Platform Account', 
      platform: 'Requested Platform', 
      token: joinToken.value 
    };
    joinToken.value = '';
  } catch (err: any) {
    console.error('Verification error:', err);
    botRequestStatus.value = 'error';
    botRequestError.value = err.response?.data?.error || err.message || 'Verification failed.';
  }
};

const isAuthorizedUI = computed(() => {
  const role = (userRole.value || '').toLowerCase();
  return ['user', 'vip', 'admin', 'superadmin', 'toby'].includes(role) || adminUser.value?.email === ADMIN_EMAIL;
});

const isAdminUI = computed(() => {
  const role = (userRole.value || '').toLowerCase();
  const isAdminEmail = adminUser.value?.email === ADMIN_EMAIL;
  return role === 'superadmin' || role === 'admin' || isAdminEmail;
});

const isTobyUI = computed(() => {
  const role = (userRole.value || '').toLowerCase();
  return role === 'toby' || isAdminUI.value;
});
</script>

<template>
  <div class="container">
    <header>
      <div class="brand">
        <h1>🐱 kitty-help</h1>
        <p>Cross-device Auxiliary Communication (PG Edition)</p>
      </div>
      <div v-if="adminUser" class="admin-auth-info">
        <span class="badge admin-badge">{{ (userRole || 'USER').toUpperCase() }}</span>
        <span class="email">{{ adminUser?.email || userName }}</span>
        <button @click="logout" class="logout-btn">Logout</button>
      </div>
      <div v-else class="admin-auth-info">
        <span class="badge visitor-badge">GUEST VIEWER</span>
        <button @click="loginAsAdmin" class="login-btn-sm">
          <img src="https://www.gstatic.com/firebasejs/ui/2.0.0/images/auth/google.svg" alt="" />
          Login
        </button>
      </div>
    </header>

    <main v-if="!loading">
      <!-- App Router Content (only for authorized roles or approved devices) -->
      <template v-if="isAuthorizedUI || deviceStatus === 'approved'">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component 
              :is="Component" 
              :device-id="deviceId" 
              :admin-email="adminUser?.email" 
              :user-role="userRole"
              :is-admin="isAdminUI"
              :is-toby="isTobyUI"
              :latency="latency"
            />
          </transition>
        </router-view>
        <Navbar :is-admin="isAdminUI" :is-toby="isTobyUI" :user-role="userRole" @login-requested="loginAsAdmin" />
      </template>

      <!-- Unauthorized View -->
      <template v-else>
        <!-- Enrollment Path (For New Platform Users / Sign-up) -->
        <div class="card enrollment-card">
          <div class="card-header">
            <span class="icon">🚀</span>
            <h2>Platform Enrollment</h2>
          </div>
          <p class="desc">Link your Discord, Telegram, or LINE to Kitty-Help.</p>
          
          <div v-if="botRequestStatus === 'submitted'" class="success-box">
            <div class="huge-check">✅</div>
            <h3>Request Submitted!</h3>
            <p>Linking <strong>{{ botRequestInfo.name }}</strong> ({{ botRequestInfo.platform }})</p>
            <div class="status-summary">
              ⏳ Waiting for AdminToby to grant access.<br>
              <span class="small">Check back in a few minutes.</span>
            </div>
          </div>
          
          <div v-else class="enroll-form">
            <div class="instruction">
              <span class="step">1</span>
              <span>Ask the bot for "**我請求加入**"</span>
            </div>
            <div class="instruction">
              <span class="step">2</span>
              <span>Paste the 8-digit code below (valid for 30m)</span>
            </div>
            
            <div class="input-portal">
              <input v-model="joinToken" placeholder="8-digit code..." @keyup.enter="verifyToken" :disabled="botRequestStatus === 'verifying'" maxlength="8" class="huge-input" />
              <button @click="verifyToken" :disabled="botRequestStatus === 'verifying' || joinToken.length < 8" class="primary-btn-lg">
                {{ botRequestStatus === 'verifying' ? 'Verifying...' : 'Link with Google & Enroll' }}
              </button>
            </div>
            <p v-if="botRequestStatus === 'error'" class="error-msg">{{ botRequestError }}</p>
          </div>
        </div>

        <!-- Authorized Access Path (Login) -->
        <div class="card login-access-card">
          <div class="card-header">
            <span class="icon">🔑</span>
            <h2>Registered Staff Login</h2>
          </div>
          <p class="desc">AdminToby and Family members with existing access.</p>
          
          <div class="login-actions">
            <button @click="loginAsAdmin" class="google-btn-lg">
              <img src="https://www.gstatic.com/firebasejs/ui/2.0.0/images/auth/google.svg" alt="Google" />
              Sign in with Google
            </button>
          </div>

          <div v-if="deviceStatus === 'pending'" class="device-alert pending">
            <span class="alert-icon">⏳</span>
            <div class="alert-text">
              <strong>Device Pending</strong>
              <span>This device (<code>{{ deviceId.substring(0, 8) }}</code>) needs approval.</span>
            </div>
          </div>
          <div v-if="deviceStatus === 'revoked'" class="device-alert revoked">
            <span class="alert-icon">🚫</span>
            <div class="alert-text">
              <strong>Access Revoked</strong>
              <span>This device has been restricted.</span>
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

@media (max-width: 600px) {
  header {
    flex-direction: column;
    gap: 1.5rem;
    padding-bottom: 1.5rem;
    text-align: center;
  }
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

/* Enrollment & Login Cards */
.enrollment-card, .login-access-card {
  max-width: 500px;
  width: 100%;
  margin: 1.5rem auto;
  text-align: left;
  border: 1px solid rgba(var(--primary-rgb), 0.1);
  background: var(--card-bg);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.5rem;
}

.card-header .icon { font-size: 2rem; }
.card-header h2 { margin: 0; font-size: 1.5rem; color: var(--primary-color); }

.enroll-form {
  margin-top: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.instruction {
  display: flex;
  align-items: center;
  gap: 1rem;
  font-size: 1rem;
  color: var(--secondary-color);
}

.step {
  background: var(--primary-color);
  color: white;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.9rem;
  font-weight: bold;
  flex-shrink: 0;
}

.input-portal {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.huge-input {
  background: rgba(0,0,0,0.4) !important;
  border: 2px solid var(--border-color) !important;
  border-radius: 12px;
  padding: 1.2rem;
  font-size: 2.2rem !important;
  text-align: center;
  color: var(--primary-color) !important;
  font-family: 'Outfit', monospace !important;
  letter-spacing: 8px;
  width: 100%;
}

.huge-input:focus {
  border-color: var(--primary-color) !important;
  box-shadow: 0 0 25px rgba(var(--primary-rgb), 0.4);
}

.primary-btn-lg {
  background: var(--primary-color);
  color: white;
  border: none;
  padding: 1.2rem;
  border-radius: 12px;
  font-size: 1.2rem;
  font-weight: 800;
  cursor: pointer;
  transition: all 0.3s;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.primary-btn-lg:hover:not(:disabled) {
  transform: translateY(-2px);
  filter: brightness(1.2);
  box-shadow: 0 4px 20px rgba(var(--primary-rgb), 0.4);
}

.google-btn-lg {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  background: white;
  color: #333;
  border: 1px solid #ddd;
  padding: 1rem;
  border-radius: 10px;
  font-weight: 700;
  font-size: 1.1rem;
  cursor: pointer;
  transition: all 0.3s;
}

.google-btn-lg img { width: 24px; }
.google-btn-lg:hover { background: #fdfdfd; transform: translateY(-1px); box-shadow: 0 4px 15px rgba(0,0,0,0.1); }

.device-alert {
  margin-top: 2rem;
  padding: 1.2rem;
  border-radius: 12px;
  display: flex;
  gap: 1rem;
  align-items: center;
}

.device-alert.pending { background: rgba(234, 179, 8, 0.12); border: 1px solid rgba(234, 179, 8, 0.4); color: #fbbf24; }
.device-alert.revoked { background: rgba(239, 68, 68, 0.12); border: 1px solid rgba(239, 68, 68, 0.4); color: #f87171; }

.alert-text { display: flex; flex-direction: column; gap: 0.2rem; }
.alert-text strong { font-size: 1.1rem; }
.alert-text span { font-size: 0.85rem; opacity: 0.9; }

.huge-check { font-size: 5rem; text-align: center; margin-bottom: 1.5rem; animation: bounceIn 0.8s cubic-bezier(0.68, -0.55, 0.265, 1.55); }
@keyframes bounceIn { from { transform: scale(0); } to { transform: scale(1); } }

.success-box {
  text-align: center;
  padding: 1rem 0;
}

.status-summary { 
  margin-top: 1.5rem; 
  background: rgba(var(--primary-rgb), 0.1);
  padding: 1rem;
  border-radius: 8px;
  font-size: 0.95rem;
  line-height: 1.6;
}

.error-msg {
  background: rgba(231, 76, 60, 0.1);
  color: #ff6b6b;
  padding: 0.8rem;
  border-radius: 8px;
  font-size: 0.9rem;
  margin-top: 1rem;
  border-left: 3px solid #e74c3c;
}
.login-btn-sm {
  background: white;
  color: #333;
  border: 1px solid #ddd;
  padding: 0.4rem 0.8rem;
  border-radius: 6px;
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  transition: all 0.2s;
}

.login-btn-sm:hover {
  background: #fdfdfd;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  transform: translateY(-1px);
}

.login-btn-sm img { width: 14px; }

.visitor-badge {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255,255,255,0.2);
  color: rgba(255,255,255,0.6);
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.5px;
}
</style>
