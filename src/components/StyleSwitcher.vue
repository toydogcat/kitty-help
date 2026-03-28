<script setup lang="ts">
import { useTheme, type Theme } from '../composables/useTheme';

const { currentTheme, setTheme } = useTheme();

const themes: { id: Theme; label: string; icon: string }[] = [
  { id: 'modern', label: 'Modern', icon: '✨' },
  { id: 'retro', label: 'Retro', icon: '👾' },
  { id: 'futurist', label: 'Futurist', icon: '🚀' },
];
</script>

<template>
  <div class="style-switcher card">
    <h3>Choose Your Style</h3>
    <div class="theme-options">
      <button
        v-for="theme in themes"
        :key="theme.id"
        @click="setTheme(theme.id)"
        :class="['theme-btn', { active: currentTheme === theme.id }]"
      >
        <span class="icon">{{ theme.icon }}</span>
        <span class="label">{{ theme.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.style-switcher {
  margin: 1rem 0;
  max-width: 400px;
  margin-left: auto;
  margin-right: auto;
}

.theme-options {
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin-top: 1rem;
}

.theme-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1rem;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--card-bg);
  cursor: pointer;
  transition: all var(--transition-speed);
  color: var(--text-color);
  min-width: 80px;
}

.theme-btn:hover {
  transform: translateY(-2px);
  border-color: var(--primary-color);
}

.theme-btn.active {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.icon {
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
}

.label {
  font-size: 0.8rem;
  font-weight: 600;
}
</style>
