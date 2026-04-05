<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import StyleSwitcher from '../components/StyleSwitcher.vue';
import CountdownTimer from '../components/CountdownTimer.vue';
import Calculator from '../components/Calculator.vue';
import OpenCliExplorer from '../components/OpenCliExplorer.vue';
import { apiService, socket } from '../services/api';

const bulletin = ref('Loading notice...');
const fontSize = ref(localStorage.getItem('kitty_font_size') || 'normal');

const handleBulletinUpdate = (data: any) => {
  bulletin.value = data.message;
};

onMounted(async () => {
  try {
    const res = await apiService.getBulletin();
    bulletin.value = res.message;
  } catch (err) {
    console.error('Failed to load bulletin:', err);
  }

  socket.on('bulletinUpdate', handleBulletinUpdate);
});

onUnmounted(() => {
  socket.off('bulletinUpdate', handleBulletinUpdate);
});

const setFontSize = (size: string) => {
  fontSize.value = size;
  localStorage.setItem('kitty_font_size', size);
  document.documentElement.setAttribute('data-font-size', size);
};

// Today's Date Logic
const today = new Date().toLocaleDateString('en-US', { 
  weekday: 'long', 
  year: 'numeric', 
  month: 'long', 
  day: 'numeric' 
});

// Initialize font size on load
document.documentElement.setAttribute('data-font-size', fontSize.value);
</script>

<template>
  <div class="home-view">
    <!-- REFACTORED: Top Settings Bar -->
    <header class="card settings-bar glow">
      <div class="settings-group theme-ctrl">
        <label>🎨 Theme</label>
        <StyleSwitcher :minimal="true" />
      </div>

      <div class="settings-divider"></div>

      <div class="settings-group font-ctrl">
        <label>🔠 Font</label>
        <div class="size-options">
          <button 
            v-for="size in ['small', 'normal', 'large', 'xlarge', 'huge']" 
            :key="size"
            @click="setFontSize(size)"
            :class="['size-btn', { active: fontSize === size }]"
          >
            {{ size === 'xlarge' ? 'XL' : (size === 'huge' ? 'HUGE' : size.toUpperCase()[0]) }}
          </button>
        </div>
      </div>
    </header>

    <div class="dashboard-grid">
      <!-- FLATTENED GRID: No more dashboard-left/right wrappers -->
      
      <section class="card notice-board grid-notice">
        <div class="notice-header">
          <h3>📢 Notice Board</h3>
          <span class="today-date">{{ today }}</span>
        </div>
        <div class="bulletin-content">
          {{ bulletin }}
        </div>
      </section>

      <section class="timer-section grid-timer">
        <CountdownTimer />
      </section>

      <section class="calculator-section grid-calc">
        <Calculator />
      </section>

      <div class="grid-ai">
        <OpenCliExplorer />
      </div>

    </div>
  </div>
</template>

<style scoped>
.home-view {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

/* Settings Bar Base */
.settings-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
  background: rgba(var(--primary-rgb), 0.05);
  border: 1px solid rgba(var(--primary-rgb), 0.2);
  border-left: 4px solid var(--primary-color);
  gap: 2rem;
}

.settings-divider {
  width: 1px;
  height: 30px;
  background: rgba(255,255,255,0.1);
}

.size-options { display: flex; gap: 0.5rem; }

.size-btn {
  width: 38px; height: 38px;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  background: rgba(0,0,0,0.2);
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.2s;
  font-size: 0.75rem;
  font-weight: 700;
  display: flex; align-items: center; justify-content: center;
}

.size-btn.active {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
  box-shadow: 0 4px 10px rgba(var(--primary-rgb), 0.4);
}

/* FLATTENED DASHBOARD GRID SYSTEM */
.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 3.5fr) minmax(320px, 1fr);
  grid-template-areas: 
    "notice timer"
    "ai     calc";
  gap: 1.5rem;
  align-items: start;
}

/* Assign Divisions */
.grid-notice { grid-area: notice; }
.grid-ai     { grid-area: ai; }
.grid-timer  { grid-area: timer; }
.grid-calc   { grid-area: calc; }

/* Notice Board Custom */
.notice-board {
  border-left: 5px solid var(--primary-color);
  background: linear-gradient(135deg, rgba(var(--primary-rgb), 0.05) 0%, transparent 100%);
  border: 1px solid rgba(var(--primary-rgb), 0.1);
}
.notice-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.today-date { font-size: 0.8rem; font-weight: 600; color: var(--primary-color); opacity: 0.8; }
.bulletin-content { font-size: 1.3rem; padding: 0.5rem; line-height: 1.6; color: var(--text-color); white-space: pre-wrap; }

/* MOBILE RWD SLIMMING & REORDERING */
@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
    grid-template-areas: 
      "notice"
      "timer"
      "calc"
      "ai";
    gap: 1rem;
  }

  .settings-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 1rem;
    padding: 1rem 0.8rem;
  }
  .settings-divider { display: none; }
  .settings-group { flex-direction: column; align-items: flex-start; gap: 0.6rem; }
  .settings-group label { font-size: 0.75rem; opacity: 0.8; }
  
  .size-btn {
    width: 32px !important; height: 32px !important;
    font-size: 0.65rem !important;
  }

  .bulletin-content {
    font-size: 1rem !important;
    padding: 0.4rem !important;
    line-height: 1.4 !important;
  }
}

.settings-group { display: flex; align-items: center; gap: 1.5rem; flex: 1; }
.settings-group label { font-size: 0.85rem; font-weight: bold; opacity: 0.7; white-space: nowrap; }
</style>
