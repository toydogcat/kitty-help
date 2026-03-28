<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  node: any;
  currentId: string | 'root';
}>();

const emit = defineEmits(['select']);

const isOpen = ref(false);

const toggle = () => {
  if (props.node.is_folder) {
    isOpen.value = !isOpen.value;
  }
};

const select = () => {
  emit('select', props.node);
};
</script>

<template>
  <div class="tree-node">
    <div 
      class="node-content" 
      :class="{ active: node.id === currentId, 'is-folder': node.is_folder }"
      @click="select"
    >
      <span class="toggle-icon" @click.stop="toggle" v-if="node.is_folder">
        {{ isOpen ? '▼' : '▶' }}
      </span>
      <span class="type-icon">{{ node.is_folder ? '📁' : '📄' }}</span>
      <span class="node-name">{{ node.name }}</span>
    </div>
    
    <div v-if="isOpen && node.children && node.children.length > 0" class="node-children">
      <SnippetTreeNode 
        v-for="child in node.children" 
        :key="child.id" 
        :node="child"
        :current-id="currentId"
        @select="(n) => emit('select', n)"
      />
    </div>
  </div>
</template>

<style scoped>
.tree-node {
  user-select: none;
}

.node-content {
  display: flex;
  align-items: center;
  padding: 0.4rem 0.6rem;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 0.9rem;
  gap: 0.4rem;
}

.node-content:hover {
  background: rgba(255, 255, 255, 0.05);
}

.node-content.active {
  background: rgba(var(--primary-rgb), 0.2);
  color: var(--primary-color);
  font-weight: bold;
}

.toggle-icon {
  width: 1rem;
  font-size: 0.7rem;
  opacity: 0.5;
  display: flex;
  justify-content: center;
}

.type-icon {
  font-size: 1rem;
}

.node-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-children {
  margin-left: 1.2rem;
  border-left: 1px solid rgba(255, 255, 255, 0.05);
  padding-left: 0.2rem;
}
</style>
