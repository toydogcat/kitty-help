<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  node: any;
  currentId: string | 'root';
}>();

const emit = defineEmits(['select', 'drop-on-node', 'drop-reorder', 'drag-start', 'drag-end']);

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
const dropPosition = ref<'inside' | 'before' | 'after'>('inside');

const handleDragOver = (e: DragEvent) => {
  e.preventDefault();
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const y = e.clientY - rect.top;
  const threshold = rect.height / 3;

  if (y < threshold) {
    dropPosition.value = 'before';
  } else if (y > rect.height - threshold) {
    dropPosition.value = 'after';
  } else {
    // Only allow 'inside' if it's a folder
    if (props.node.isFolder) {
      dropPosition.value = 'inside';
    } else {
      dropPosition.value = y < rect.height / 2 ? 'before' : 'after';
    }
  }
  isDropOver.value = true;
};

const handleDragLeave = () => {
  isDropOver.value = false;
};

const handleDragStart = (_e: DragEvent) => {
  emit('drag-start', props.node);
};

const handleDrop = (e: DragEvent) => {
  isDropOver.value = false;
  if (dropPosition.value === 'inside') {
    emit('drop-on-node', { targetNode: props.node });
  } else {
    emit('drop-reorder', { 
        targetNode: props.node, 
        position: dropPosition.value 
    });
  }
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
        'drop-over': isDropOver,
        'drop-before': isDropOver && dropPosition === 'before',
        'drop-after': isDropOver && dropPosition === 'after',
        'drop-inside': isDropOver && dropPosition === 'inside'
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
      <span class="type-icon">{{ node.isFolder ? '📁' : '🔗' }}</span>
      <span class="node-name">{{ node.title }}</span>
    </div>
    
    <div v-if="isOpen && node.children && node.children.length > 0" class="node-children">
      <BookmarkTreeNode 
        v-for="child in node.children" 
        :key="child.id" 
        :node="child"
        :current-id="currentId"
        @select="(n) => emit('select', n)"
        @drop-on-node="(data) => emit('drop-on-node', data)"
        @drop-reorder="(data) => emit('drop-reorder', data)"
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
  font-size: 0.95rem;
  gap: 0.4rem;
  text-align: left;
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
  background: rgba(255, 255, 255, 0.05);
}

.node-content.drop-inside {
  background: rgba(var(--primary-rgb), 0.3) !important;
  border: 1px dashed var(--primary-color);
  transform: scale(1.05);
}

.node-content.drop-before {
  border-top: 2px solid var(--primary-color);
}

.node-content.drop-after {
  border-bottom: 2px solid var(--primary-color);
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
