<script setup lang="ts">
import { useTheme, type Theme } from '../composables/useTheme';

const { currentTheme, setTheme } = useTheme();

defineProps<{
  minimal?: boolean;
}>();

const themes: { id: Theme; label: string; icon: string }[] = [
  { id: 'futurist', label: 'Futurist', icon: '🚀' },
  { id: 'cyberpunk', label: 'Cyberpunk', icon: '🌌' },
  { id: 'forest', label: 'Forest', icon: '🌿' },
  { id: 'ocean', label: 'Ocean', icon: '🌊' },
  { id: 'sunset', label: 'Sunset', icon: '🌅' },
];
</script>

<template>
  <div :class="['style-switcher', { card: !minimal, 'minimal-mode': minimal }]">
    <h3 v-if="!minimal">Choose Your Style</h3>
    <div class="theme-options">
      <button
        v-for="theme in themes"
        :key="theme.id"
        @click="setTheme(theme.id)"
        :class="['theme-btn', { active: currentTheme === theme.id, 'mini-btn': minimal }]"
        :title="theme.label"
      >
        <span class="theme-icon">{{ theme.icon }}</span>
        <span v-if="!minimal" class="theme-label">{{ theme.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.style-switcher:not(.minimal-mode) {
  margin: 1rem 0;
  max-width: 400px;
  margin-left: auto;
  margin-right: auto;
}

.minimal-mode {
  display: flex;
  align-items: center;
}

.theme-options {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
  justify-content: flex-start;
}

.theme-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border: 1px solid rgba(var(--primary-rgb), 0.2);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.05);
  cursor: pointer;
  transition: all 0.2s ease;
  padding: 0;
}

.mini-btn {
  width: 48px;
  height: 48px;
  border-radius: 10px;
}

.theme-btn:hover {
  transform: translateY(-2px);
  border-color: var(--primary-color);
  background: rgba(var(--primary-rgb), 0.1);
}

.theme-btn.active {
  background: var(--primary-color);
  border-color: var(--primary-color);
  box-shadow: 0 0 15px rgba(var(--primary-rgb), 0.5);
}

.theme-icon {
  font-size: 1.5rem;
  line-height: 1;
}

.theme-label {
  font-size: 0.75rem;
  font-weight: 600;
  margin-top: 0.3rem;
}

/* Specific styling for the mini version (dashboard header) */
.mini-btn .theme-label {
  display: none;
}

.mini-btn .theme-icon {
  font-size: 1.25rem;
}

@media (max-width: 600px) {
  .theme-btn {
    width: 42px !important;
    height: 42px !important;
  }
  .theme-icon {
    font-size: 1.2rem !important;
  }
  .theme-options {
    gap: 0.4rem;
  }
}
</style>
