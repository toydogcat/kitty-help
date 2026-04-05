<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  node: any;
  currentId: string | 'root';
}>();

const emit = defineEmits(['select', 'drop-on-node', 'drag-start', 'drag-end']);

const isOpen = ref(false);

const toggle = () => {
  if (props.node.isFolder) {
    isOpen.value = !isOpen.value;
  }
};

const select = () => {
  emit('select', props.node);
};

const isDropOver = ref(false);

const handleDragOver = (e: DragEvent) => {
  if (!props.node.isFolder) return;
  e.preventDefault();
  isDropOver.value = true;
};

const handleDragLeave = () => {
  isDropOver.value = false;
};

const handleDragStart = (_e: DragEvent) => {
  emit('drag-start', props.node);
};

const handleDrop = (_e: DragEvent) => {
  if (!props.node.isFolder) return;
  isDropOver.value = false;
  // Emit event to parent (SnippetExplorer) to handle the actual move
  emit('drop-on-node', { targetNode: props.node });
};

const handleDragEnd = () => {
  isDropOver.value = false;
  emit('drag-end');
};
</script>

<template>
  <div class="tree-node">
    <div 
      class="node-content" 
      :class="{ 
        active: node.id === currentId, 
        'is-folder': node.isFolder,
        'drop-over': isDropOver
      }"
      draggable="true"
      @click="select"
      @dragstart="handleDragStart"
      @dragover="handleDragOver"
      @dragleave="handleDragLeave"
      @drop="handleDrop"
      @dragend="handleDragEnd"
    >
      <span class="toggle-icon" @click.stop="toggle" v-if="node.isFolder">
        {{ isOpen ? '▼' : '▶' }}
      </span>
      <span class="type-icon">{{ node.isFolder ? '📁' : '📄' }}</span>
      <span class="node-name">{{ node.name }}</span>
    </div>
    
    <div v-if="isOpen && node.children && node.children.length > 0" class="node-children">
      <SnippetTreeNode 
        v-for="child in node.children" 
        :key="child.id" 
        :node="child"
        :current-id="currentId"
        @select="(n) => emit('select', n)"
        @drop-on-node="(data) => emit('drop-on-node', data)"
        @drag-start="(n) => emit('drag-start', n)"
        @drag-end="() => emit('drag-end')"
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

.node-content.drop-over {
  background: rgba(var(--primary-rgb), 0.3) !important;
  border: 1px dashed var(--primary-color);
  transform: scale(1.05);
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
