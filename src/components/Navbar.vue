<script setup lang="ts">
import { computed } from 'vue';
import { useRouter, useRoute } from 'vue-router';

const props = defineProps<{ isAdmin: boolean }>();
const router = useRouter();
const route = useRoute();

const navItems = computed(() => {
  const items = [
    { name: 'home', label: 'Home', icon: '🏠', path: '/' },
    { name: 'personal', label: 'Personal', icon: '📋', path: '/personal' },
  ];
  if (props.isAdmin) {
    items.push({ name: 'admin', label: 'Admin', icon: '⚙️', path: '/admin' });
  }
  return items;
});

const currentRouteName = computed(() => route.name);

const navigate = (path: string) => {
  router.push(path);
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
