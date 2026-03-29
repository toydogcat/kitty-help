<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import StyleSwitcher from '../components/StyleSwitcher.vue';
import FamilyCalendar from '../components/FamilyCalendar.vue';
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

// Initialize font size on load
document.documentElement.setAttribute('data-font-size', fontSize.value);
</script>

<template>
  <div class="home-view">
    <section class="card notice-board">
      <h3>📢 Notice Board</h3>
      <div class="bulletin-content">
        {{ bulletin }}
      </div>
    </section>

    <section class="calendar-section">
      <FamilyCalendar mode="home" />
    </section>

    <section class="settings-grid">
      <div class="card">
        <h3>🎨 Theme Style</h3>
        <p class="desc">Select your favorite visual appearance.</p>
        <StyleSwitcher />
      </div>

      <div class="card">
        <h3>🔠 Font Size</h3>
        <p class="desc">Adjust the legibility of the interface.</p>
        <div class="size-options">
          <button 
            v-for="size in ['small', 'normal', 'large', 'xlarge', 'huge']" 
            :key="size"
            @click="setFontSize(size)"
            :class="['size-btn', { active: fontSize === size }]"
          >
            {{ size === 'xlarge' ? 'XL' : (size === 'huge' ? 'HUGE' : size.toUpperCase()) }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.notice-board {
  margin-bottom: 2rem;
  border-left: 5px solid var(--primary-color);
  background: linear-gradient(to right, var(--card-bg), transparent);
}

.calendar-section {
  margin-bottom: 2rem;
}

.bulletin-content {
  font-size: 1.25em; /* Scale relatively to parent base size */
  padding: 1rem;
  line-height: 1.6;
  color: var(--text-color);
  white-space: pre-wrap;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 2rem;
}

.desc {
  font-size: 0.85rem;
  color: var(--secondary-color);
  margin-bottom: 1.5rem;
}

.size-options {
  display: flex;
  justify-content: center;
  gap: 1rem;
}

.size-btn {
  padding: 0.6rem 1.2rem;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--card-bg);
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 600;
}

.size-btn.active {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}
</style>
