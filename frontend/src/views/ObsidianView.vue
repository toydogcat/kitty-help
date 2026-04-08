<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';

const currentPath = ref('');
const files = ref<any[]>([]);
const loading = ref(true);
const selectedFile = ref<any | null>(null);
const markdownContent = ref('');
const fetchingContent = ref(false);
const showHidden = ref(false);
const errorMsg = ref('');

const loadFiles = async (path: string = '') => {
  loading.value = true;
  errorMsg.value = '';
  try {
    const data = await apiService.listObsidianFiles(path, showHidden.value);
    currentPath.value = data.currentPath || '';
    files.value = (data.files || []).sort((a: any, b: any) => {
      if (a.isDir && !b.isDir) return -1;
      if (!a.isDir && b.isDir) return 1;
      return a.name.localeCompare(b.name);
    });
  } catch (err: any) {
    console.error("Failed to load obsidian files:", err);
    errorMsg.value = `載入列表失敗: ${err.response?.data?.error || err.message}`;
  } finally {
    loading.value = false;
  }
};

watch(showHidden, () => {
  loadFiles(currentPath.value);
});

const navigateTo = (file: any) => {
  if (file.isDir) {
    loadFiles(file.path);
  } else if (file.name.toLowerCase().endsWith('.md')) {
    openFile(file);
  }
};

const goBack = () => {
  if (!currentPath.value) return;
  const parts = currentPath.value.split('/');
  parts.pop();
  loadFiles(parts.join('/'));
};

const openFile = async (file: any) => {
  selectedFile.value = file;
  fetchingContent.value = true;
  markdownContent.value = '';
  try {
    const text = await apiService.getObsidianFileContent(file.path);
    let content = typeof text === 'string' ? text : JSON.stringify(text, null, 2);

    // 1. Resolve Obsidian Wikilinks [[Note Name]]
    // For now, we just highlight them. Real navigation would require a map.
    content = content.replace(/\[\[([^\]]+)\]\]/g, '<strong>🔗 $1</strong>');

    // 2. Setup Custom Renderer for Images
    const renderer = new marked.Renderer();
    const noteDir = file.path.split('/').slice(0, -1).join('/');

    renderer.image = (href, title, text) => {
      if (!href.startsWith('http')) {
        // Resolve relative paths for images in Obsidian
        const fullImagePath = noteDir ? `${noteDir}/${href}` : href;
        const proxiedUrl = apiService.getStorehouseFileUrl(fullImagePath, 'local');
        return `<img src="${proxiedUrl}" alt="${text || ''}" title="${title || ''}" />`;
      }
      return `<img src="${href}" alt="${text || ''}" />`;
    };

    markdownContent.value = marked.parse(content, { renderer });
  } catch (err: any) {
    console.error("Failed to fetch obsidian content:", err);
    const detail = err.response?.data?.error || err.message;
    markdownContent.value = `<p class="error-box">⚠️ 無法載入筆記內容<br/><small>${detail}</small></p>`;
  } finally {
    fetchingContent.value = false;
  }
};

onMounted(() => {
  loadFiles();
});
</script>

<template>
  <div class="obsidian-view">
    <header class="view-header">
      <div class="header-content">
        <h2>📑 Obsidian Vault</h2>
        <div class="header-meta">
          <p>本地知識庫直接存取 (Path: /root/obsidian/{{ currentPath }})</p>
          <label class="toggle-hidden">
            <input type="checkbox" v-model="showHidden" />
            顯示隱藏檔案 (.)
          </label>
        </div>
      </div>
      <button v-if="currentPath" @click="goBack" class="btn-back">⬅️ 返回上一層</button>
    </header>

    <div class="layout-container">
      <!-- File List -->
      <aside class="file-browser card glow">
        <div v-if="loading" class="loader">
          <div class="spinner"></div>
        </div>
        <div v-else-if="errorMsg" class="error-msg">{{ errorMsg }}</div>
        <div v-else class="file-list">
          <div 
            v-for="file in files" 
            :key="file.path" 
            class="file-item" 
            :class="{ active: selectedFile?.path === file.path }"
            @click="navigateTo(file)"
          >
            <span class="file-icon">{{ file.isDir ? '📁' : '📄' }}</span>
            <span class="file-name">{{ file.name }}</span>
          </div>
          <div v-if="files && files.length === 0" class="empty-msg">這層目錄是空的</div>
        </div>
      </aside>

      <!-- Content Preview -->
      <main class="content-preview card glow">
        <div v-if="fetchingContent" class="loader">
          <div class="spinner"></div>
          <p>正在通靈筆記內容...</p>
        </div>
        <div v-else-if="selectedFile" class="markdown-body">
          <div class="file-header">
            <h3>{{ selectedFile.name }}</h3>
            <span class="file-meta">最後修改: {{ new Date(selectedFile.modTime).toLocaleString() }}</span>
          </div>
          <hr />
          <div class="markdown-content" v-html="markdownContent"></div>
        </div>
        <div v-else class="welcome-msg">
          <div class="big-icon">🏮</div>
          <h3>請從左側選擇一份筆記</h3>
          <p>在此直接讀取你的本地知識庫</p>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.obsidian-view {
  display: flex; flex-direction: column; gap: 2rem; height: calc(100vh - 120px);
}

.view-header { display: flex; justify-content: space-between; align-items: flex-end; }
.header-meta { display: flex; align-items: center; gap: 2rem; font-size: 0.9rem; opacity: 0.8; }
.toggle-hidden { display: flex; align-items: center; gap: 0.5rem; cursor: pointer; color: var(--primary-color); }

.btn-back {
  padding: 0.6rem 1.2rem; border-radius: 12px;
  background: rgba(var(--primary-rgb), 0.1); border: 1px solid var(--primary-color);
  color: white; cursor: pointer; transition: all 0.3s;
}
.btn-back:hover { background: var(--primary-color); }

.layout-container {
  display: grid; grid-template-columns: 350px 1fr; gap: 1.5rem; flex: 1; min-height: 0;
}

@media (max-width: 900px) {
  .layout-container { grid-template-columns: 1fr; }
  .file-browser { display: none; }
}

.file-browser { 
  overflow-y: auto; padding: 1rem; background: rgba(var(--primary-rgb), 0.05);
}

.file-list { display: flex; flex-direction: column; gap: 0.5rem; }
.file-item {
  display: flex; align-items: center; gap: 0.8rem; padding: 0.8rem;
  border-radius: 10px; cursor: pointer; transition: all 0.2s;
  border: 1px solid transparent; text-align: left;
}
.file-item:hover { background: rgba(255,255,255,0.05); border-color: rgba(var(--primary-rgb), 0.3); }
.file-item.active { background: var(--primary-color); border-color: white; }
.file-icon { font-size: 1.2rem; }
.file-name { font-size: 0.95rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.content-preview { 
  overflow-y: auto; padding: 0; display: flex; flex-direction: column; background: rgba(var(--primary-rgb), 0.02);
}

.markdown-body { padding: 3rem; text-align: left; }
.file-header { margin-bottom: 1.5rem; }
.file-header h3 { margin: 0 0 0.5rem 0; color: var(--primary-color); font-size: 1.8rem; }
.file-meta { font-size: 0.85rem; opacity: 0.6; }

.error-box { padding: 2rem; background: rgba(255, 0, 0, 0.1); border: 1px solid red; border-radius: 10px; color: #ff6666; text-align: center; }
.error-msg { padding: 1rem; color: #ff6666; font-size: 0.9rem; text-align: center; }
.file-item:hover { background: rgba(255,255,255,0.05); border-color: rgba(var(--primary-rgb), 0.3); }
.file-item.active { background: var(--primary-color); border-color: white; }
.file-icon { font-size: 1.2rem; }
.file-name { font-size: 0.95rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.content-preview { 
  overflow-y: auto; padding: 0; display: flex; flex-direction: column;
}

.markdown-body { padding: 3rem; text-align: left; }
.file-header { margin-bottom: 1.5rem; }
.file-header h3 { margin: 0 0 0.5rem 0; color: var(--primary-color); font-size: 1.8rem; }
.file-meta { font-size: 0.85rem; opacity: 0.6; }

.markdown-content :deep(h1), .markdown-content :deep(h2) {
  border-bottom: 1px solid rgba(255,255,255,0.1); padding-bottom: 0.5rem;
  color: var(--primary-color); margin-top: 2rem;
}
.markdown-content :deep(pre) { background: #000; padding: 1.2rem; border-radius: 10px; overflow-x: auto; }
.markdown-content :deep(code) { background: rgba(255,255,255,0.1); padding: 0.2rem 0.4rem; border-radius: 4px; }
.markdown-content :deep(img) { max-width: 100%; border-radius: 12px; }
.markdown-content :deep(blockquote) { border-left: 4px solid var(--primary-color); padding-left: 1rem; opacity: 0.8; }

.welcome-msg, .empty-msg {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  opacity: 0.5; height: 100%;
}
.big-icon { font-size: 5rem; margin-bottom: 1rem; }

.loader { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 5rem; }
</style>
