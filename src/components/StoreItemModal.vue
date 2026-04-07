<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
  item: any | null;
  show: boolean;
}>();

const emit = defineEmits(['close', 'updated']);

const editingTitle = ref('');
const editingNotes = ref('');
const saving = ref(false);
const indexing = ref(false);

watch(() => props.item, (newItem) => {
  if (newItem) {
    editingTitle.value = newItem.title || '';
    editingNotes.value = newItem.notes || '';
  }
}, { immediate: true });

const handleSave = async () => {
  if (!props.item) return;
  saving.value = true;
  try {
    await apiService.updateStorehouseItem(props.item.id, {
      title: editingTitle.value,
      notes: editingNotes.value
    });
    emit('updated');
    emit('close');
  } catch (err) {
    console.error("Failed to update item:", err);
    alert("儲存失敗，請檢查網路連線。");
  } finally {
    saving.value = false;
  }
};

const handleIndex = async () => {
  if (!props.item) return;
  indexing.value = true;
  try {
    await apiService.indexStorehouseItem(props.item.id);
    emit('updated');
    // We don't close, just update status
  } catch (err) {
    console.error("Indexing failed:", err);
    alert("AI 索引失敗，請稍後再試。");
  } finally {
    indexing.value = false;
  }
};

const getFileUrl = (download: boolean = false) => {
  if (!props.item) return '';
  return apiService.getStorehouseFileUrl(props.item.file_id, props.item.source, download);
};

const isImage = () => props.item?.category === 'photo' || props.item?.category === 'image';
const isVideo = () => props.item?.category === 'video';
const isAudio = () => props.item?.category === 'audio';

const statusText = computed(() => {
  if (!props.item) return '';
  switch(props.item.index_status) {
    case 'indexed': return '✅ 已建立 AI 語意索引';
    case 'indexing': return '⏳ AI 正在理解內容...';
    case 'failed': return '❌ 索引失敗';
    case 'not_indexed': return '📄 尚未索引';
    default: return '';
  }
});

</script>

<template>
  <Transition name="modal-fade">
    <div v-if="show && item" class="modal-overlay" @click.self="emit('close')">
      <div class="modal-content card glow">
        <button class="close-btn" @click="emit('close')">✕</button>
        
        <div class="modal-body">
          <div class="media-preview">
            <template v-if="isImage()">
              <img :src="getFileUrl()" :alt="item.title" class="preview-img" />
            </template>
            <template v-else-if="isVideo()">
              <video controls class="preview-video">
                <source :src="getFileUrl()" type="video/mp4">
                Your browser does not support the video tag.
              </video>
            </template>
            <template v-else-if="isAudio()">
              <div class="audio-container">
                <div class="audio-icon">🎵</div>
                <audio controls class="preview-audio">
                  <source :src="getFileUrl()" type="audio/mpeg">
                </audio>
              </div>
            </template>
            <template v-else>
              <div class="file-placeholder">
                <span class="p-icon">📄 {{ item.category }}</span>
                <p>此類型檔案不支援在線預覽</p>
                <a :href="getFileUrl(true)" class="btn btn-primary" download>直接下載</a>
              </div>
            </template>
          </div>

          <div class="item-details">
            <div class="form-group">
              <label>標題</label>
              <input v-model="editingTitle" type="text" placeholder="輸入資源名稱..." />
            </div>

            <div class="form-group">
              <label>備註與描述</label>
              <textarea v-model="editingNotes" rows="4" placeholder="增加一些詳細描述或語意標籤..."></textarea>
            </div>

            <div class="ai-indexing card mini">
              <div class="ai-header">
                <span class="ai-status">{{ statusText }}</span>
                <span v-if="item.index_status === 'indexed'" class="ai-model">Gemini-2</span>
              </div>
              <button 
                v-if="item.is_indexable && item.index_status !== 'indexed'"
                class="index-btn" 
                :disabled="indexing || item.index_status === 'indexing'"
                @click="handleIndex"
              >
                {{ indexing ? '正在運算向量...' : '✨ 生成 AI 語意索引' }}
              </button>
              <p v-else-if="!item.is_indexable" class="hint-text">⚠️ 系統不支援對此檔案格式建立 AI 索引</p>
            </div>

            <div class="meta-info">
              <p><span>來源：</span>{{ item.source.toUpperCase() }}</p>
              <p><span>上傳者：</span>{{ item.sender || '未知' }}</p>
              <p><span>時間：</span>{{ new Date(item.created_at).toLocaleString() }}</p>
            </div>

            <div class="actions">
              <a :href="getFileUrl(true)" class="download-link btn" download>⬇️ 下載原始檔案</a>
              <button class="save-btn btn btn-primary" :disabled="saving" @click="handleSave">
                {{ saving ? '儲存中...' : '儲存修改' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0; left: 0; width: 100%; height: 100%;
  background: rgba(0, 0, 0, 0.85);
  display: flex; justify-content: center; align-items: center;
  z-index: 2000; backdrop-filter: blur(8px);
}

.modal-content {
  width: 90%; max-width: 1100px; max-height: 90vh;
  position: relative; overflow: hidden;
  border: 1px solid rgba(var(--primary-rgb), 0.3);
  padding: 0;
}

.close-btn {
  position: absolute; top: 1rem; right: 1rem;
  background: rgba(255,255,255,0.1); border: none;
  color: white; font-size: 1.5rem; cursor: pointer; z-index: 10;
  width: 40px; height: 40px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
}

.modal-body {
  display: grid; grid-template-columns: 1fr 400px;
  height: 100%;
}

@media (max-width: 900px) {
  .modal-body { grid-template-columns: 1fr; overflow-y: auto; }
}

.media-preview {
  background: #000; display: flex; align-items: center; justify-content: center;
  min-height: 400px; max-height: 90vh;
}

.preview-img, .preview-video {
  max-width: 100%; max-height: 80vh; object-fit: contain;
}

.item-details {
  padding: 2rem; display: flex; flex-direction: column; gap: 1.5rem;
  background: var(--card-bg); overflow-y: auto;
}

.form-group { display: flex; flex-direction: column; gap: 0.5rem; }
.form-group label { font-weight: bold; opacity: 0.8; }
.form-group input, .form-group textarea {
  background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px; padding: 0.8rem; color: white;
}

.ai-indexing {
  background: rgba(var(--primary-rgb), 0.05);
  border: 1px dashed rgba(var(--primary-rgb), 0.3);
  padding: 1rem; border-radius: 12px;
}

.ai-header { display: flex; justify-content: space-between; margin-bottom: 0.8rem; align-items: center; }
.ai-status { font-size: 0.9rem; font-weight: bold; }
.ai-model { font-size: 0.7rem; background: var(--primary-color); padding: 2px 6px; border-radius: 4px; }

.index-btn {
  width: 100%; padding: 0.7rem; border-radius: 8px;
  background: linear-gradient(135deg, var(--primary-color), #a855f7);
  border: none; color: white; font-weight: bold; cursor: pointer;
  transition: all 0.3s;
}

.index-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.index-btn:hover:not(:disabled) { transform: scale(1.02); filter: brightness(1.2); }

.hint-text { font-size: 0.8rem; opacity: 0.5; margin: 0; }

.meta-info { font-size: 0.9rem; opacity: 0.7; }
.meta-info span { font-weight: bold; }

.actions { margin-top: auto; display: flex; flex-direction: column; gap: 0.8rem; }
.btn {
  text-align: center; padding: 0.8rem; border-radius: 8px;
  text-decoration: none; font-weight: bold; border: none; cursor: pointer;
}
.btn-primary { background: var(--primary-color); color: white; }
.download-link { background: rgba(255,255,255,0.1); color: white; }

.modal-fade-enter-active, .modal-fade-leave-active { transition: opacity 0.3s; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
</style>
