<script setup lang="ts">
import { ref, watch } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';

const props = defineProps<{
  show: boolean;
  item: any | null; // The desk item or chat message being edited
  details: any | null; // Additional data (like remark items)
  loading?: boolean;
}>();

const emit = defineEmits(['close', 'save', 'zoom']);

const isFullScreen = ref(false);
const editMode = ref<'edit' | 'preview'>('preview');
const saving = ref(false);
const editBuffer = ref({ title: '', content: '' });

watch(() => props.item, (newVal) => {
  if (newVal) {
    editBuffer.value = {
      title: newVal.title || newVal.name || '',
      content: newVal.content || newVal.notes || ''
    };
    editMode.value = 'preview';
  }
}, { immediate: true });

const handleSave = async () => {
  if (!props.item) return;
  saving.value = true;
  try {
    emit('save', { ...editBuffer.value });
  } finally {
    saving.value = false;
  }
};

const getStorehouseUrl = (mediaId: string, platform?: string) => {
  return apiService.getStorehouseFileUrl(mediaId, platform || 'line');
};

const getThumbnail = (item: any, large = false) => {
  if (item.type === 'media' && item.refId) {
    const baseUrl = apiService.getStorehouseFileUrl(item.refId, item.source);
    return `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}${large ? 'w=1024' : 'w=256'}`;
  }
  return null;
};
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay editor-modal-overlay" @click.self="emit('close')">
      <div class="editor-pane shadow-2xl" :class="{ 'is-full': isFullScreen }">
        <div class="editor-header">
          <div class="type-badge">{{ item?.type?.toUpperCase() || 'REMARK' }} EDITOR</div>
          
          <div class="unified-controls">
             <div class="mode-capsule" v-if="item?.type === 'remark' || item?.type === 'snippet' || !item?.type">
                <button @click="editMode = 'preview'" :class="{ active: editMode === 'preview' }">MD PREVIEW</button>
                <button @click="editMode = 'edit'" :class="{ active: editMode === 'edit' }">TXT / EDIT</button>
             </div>
             <div class="action-set">
                <button @click="isFullScreen = !isFullScreen" class="action-item" title="Fullscreen">
                  {{ isFullScreen ? '❐' : '⛶' }}
                </button>
                <button @click="emit('close')" class="action-item close">✕</button>
             </div>
          </div>
        </div>

        <div class="editor-body custom-scrollbar">
          <!-- Big Media Preview -->
          <div v-if="getThumbnail(item)" class="media-large-preview">
            <img :src="getThumbnail(item, true) || ''" />
          </div>

          <div class="field">
            <label>Title / Category Name</label>
            <input v-model="editBuffer.title" placeholder="e.g., Project Workspace" />
          </div>

          <div class="field fill">
            <label>Notes & Summary (Markdown Supported)</label>
            <div v-if="editMode === 'preview'" class="md-preview-area" v-html="marked.parse(editBuffer.content || '')"></div>
            <textarea v-else v-model="editBuffer.content" placeholder="Paste or type details here..."></textarea>
          </div>

          <!-- Quoted Items Grid -->
          <div v-if="item?.type === 'remark' || details?.items" class="nested-remark-items">
            <label class="section-label">📚 Quoted Items (引用項目)</label>
            <div v-if="loading" class="modal-item-loader"><span class="spinner"></span> Loading items...</div>
            <div v-else class="quoted-items-grid">
              <div v-for="it in (details?.items || [])" :key="it.id" class="quoted-item-card">
                <div class="item-meta-top">
                  <span class="p-slug">{{ it.log?.platform }}</span>
                  <span class="p-user">{{ it.log?.senderName }}</span>
                </div>
                <div v-if="it.log?.mediaId && (['image', 'photo'].includes(it.log?.msgType) || it.log?.content?.includes('[Image]'))" class="item-img-box" @click="emit('zoom', getStorehouseUrl(it.log.mediaId, it.log.platform))">
                  <img :src="getStorehouseUrl(it.log.mediaId, it.log.platform)" />
                </div>
                <div v-else class="item-text-box">
                  <p>{{ it.log?.content }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="editor-footer">
          <button @click="emit('close')" class="cancel-btn">Discard</button>
          <button @click="handleSave" class="save-btn" :disabled="saving">
            {{ saving ? 'Saving...' : '✅ Save Changes' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.editor-modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.85); backdrop-filter: blur(15px); display: flex; align-items: center; justify-content: center; z-index: 3000; }
.editor-pane { background: var(--card-bg); width: 1000px; max-width: 95vw; max-height: 92vh; border-radius: 32px; border: 1px solid rgba(var(--primary-rgb), 0.3); display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 30px 80px rgba(0,0,0,0.8), inset 0 0 100px rgba(var(--primary-rgb), 0.05); }
.editor-pane.is-full { width: 100vw; height: 100vh; max-height: 100vh; border-radius: 0; border: none; }

.editor-header { padding: 1.2rem 2.5rem; display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.05); }
.type-badge { font-size: 0.7rem; font-weight: 900; letter-spacing: 2px; color: var(--primary-color); opacity: 0.8; }

.unified-controls { display: flex; align-items: center; gap: 0.8rem; }
.mode-capsule { display: flex; background: rgba(0,0,0,0.4); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.mode-capsule button { background: none; border: none; color: #fff; padding: 6px 14px; border-radius: 9px; font-size: 0.75rem; font-weight: 800; cursor: pointer; opacity: 0.4; transition: all 0.2s; }
.mode-capsule button.active { background: var(--primary-color); opacity: 1; box-shadow: 0 2px 8px rgba(var(--primary-rgb), 0.4); }

.action-set { display: flex; background: rgba(255,255,255,0.05); padding: 4px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.05); }
.action-item { background: none; border: none; color: #fff; width: 34px; height: 34px; border-radius: 9px; font-size: 1.1rem; cursor: pointer; opacity: 0.6; transition: all 0.2s; display: flex; align-items: center; justify-content: center; }
.action-item:hover { background: rgba(255,255,255,0.1); opacity: 1; }
.action-item.close:hover { background: #e74c3c; }

.editor-body { flex: 1; overflow-y: auto; padding: 2.5rem; display: flex; flex-direction: column; gap: 2rem; }
.media-large-preview { width: 100%; max-height: 400px; border-radius: 20px; overflow: hidden; background: #000; border: 1px solid rgba(255,255,255,0.1); margin-bottom: 1rem; }
.media-large-preview img { width: 100%; height: 100%; object-fit: contain; }

.field { display: flex; flex-direction: column; gap: 0.8rem; }
.field label { font-size: 0.75rem; font-weight: 900; text-transform: uppercase; color: var(--primary-color); opacity: 0.6; letter-spacing: 2px; }
.field input, .field textarea { background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.08); border-radius: 16px; padding: 1.4rem; color: #fff; width: 100%; outline: none; transition: border-color 0.2s; font-size: 1.05rem; }
.field textarea { min-height: 250px; resize: vertical; }
.field input:focus, .field textarea:focus { border-color: var(--primary-color); }

.md-preview-area { background: rgba(0,0,0,0.35); padding: 2.5rem; border-radius: 16px; border: 1px solid rgba(255,255,255,0.05); min-height: 350px; color: #eee; line-height: 1.8; font-size: 1.1rem; overflow-y: auto; }
.md-preview-area :deep(h1) { color: var(--primary-color); margin: 1.5rem 0 1rem; }
.md-preview-area :deep(p) { margin-bottom: 1.2rem; }
.md-preview-area :deep(code) { background: rgba(255,255,255,0.1); padding: 2px 6px; border-radius: 4px; font-family: monospace; }
.md-preview-area :deep(pre) { background: rgba(0,0,0,0.4); padding: 1.5rem; border-radius: 12px; overflow-x: auto; margin: 1.5rem 0; }

.nested-remark-items { margin-top: 2rem; padding-top: 2rem; border-top: 1px solid rgba(255,255,255,0.05); }
.section-label { font-size: 0.8rem; font-weight: 900; color: var(--primary-color); letter-spacing: 1.5px; margin-bottom: 1.5rem; display: block; }
.quoted-items-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 1.5rem; }
.quoted-item-card { background: rgba(255,255,255,0.03); border-radius: 20px; border: 1px solid rgba(255,255,255,0.06); overflow: hidden; display: flex; flex-direction: column; transition: all 0.3s; }
.quoted-item-card:hover { transform: translateY(-5px); border-color: var(--primary-color); }

.item-meta-top { background: rgba(0,0,0,0.35); padding: 0.8rem 1.2rem; display: flex; justify-content: space-between; font-size: 0.7rem; font-weight: 800; align-items: center; }
.p-slug { opacity: 0.5; letter-spacing: 1px; text-transform: uppercase; }
.p-user { color: var(--primary-color); }
.item-img-box { height: 200px; cursor: zoom-in; overflow: hidden; background: #000; }
.item-img-box img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.5s; }
.quoted-item-card:hover .item-img-box img { transform: scale(1.1); }
.item-text-box { padding: 1.5rem; font-size: 1rem; color: #ddd; line-height: 1.6; max-height: 200px; overflow-y: auto; }

.editor-footer { padding: 1.5rem 2.5rem; display: flex; justify-content: flex-end; gap: 1.2rem; background: rgba(0,0,0,0.25); border-top: 1px solid rgba(255,255,255,0.05); }
.cancel-btn { background: rgba(255,255,255,0.05); color: #fff; padding: 0.8rem 2rem; border-radius: 14px; font-weight: 700; cursor: pointer; border: 1px solid rgba(255,255,255,0.1); }
.save-btn { background: var(--primary-color); color: #fff; padding: 0.8rem 3rem; border-radius: 14px; font-weight: 800; cursor: pointer; box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.3); border: none; }
.save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.custom-scrollbar::-webkit-scrollbar { width: 8px; height: 8px; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 10px; }
</style>
