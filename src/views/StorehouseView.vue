<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue';
import { apiService, socket } from '../services/api';
import StoreItemModal from '../components/StoreItemModal.vue';

const items = ref<any[]>([]);
const loading = ref(true);
const page = ref(1);
const limit = ref(20);

// Helper for date formatting
const formatDate = (date: Date) => date.toISOString().split('T')[0];

const today = new Date();
const tomorrow = new Date();
tomorrow.setDate(today.getDate() + 1);

const oneMonthAgo = new Date();
oneMonthAgo.setMonth(oneMonthAgo.getMonth() - 1);

// Filters
const filterPlatform = ref('');
const filterStartDate = ref(formatDate(oneMonthAgo));
const filterEndDate = ref(formatDate(tomorrow));
const searchQuery = ref('');
const searchMode = ref('keyword'); // keyword | semantic

// Modal
const selectedItem = ref<any | null>(null);
const showModal = ref(false);

const loadItems = async () => {
  loading.value = true;
  try {
    const params = {
      platform: filterPlatform.value,
      startDate: filterStartDate.value,
      endDate: filterEndDate.value,
      q: searchQuery.value,
      mode: searchMode.value,
      limit: limit.value
    };
    items.value = await apiService.getStorehouseItems(params);
  } catch (err) {
    console.error("Failed to load storehouse items:", err);
  } finally {
    loading.value = false;
  }
};

// Debounced search
let searchTimeout: any;
watch([filterPlatform, filterStartDate, filterEndDate, searchQuery, searchMode, limit], () => {
  clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    page.value = 1;
    loadItems();
  }, 500);
});

onMounted(() => {
  loadItems();
  socket.on('storehouseUpdate', () => {
    loadItems();
  });
});

onUnmounted(() => {
  socket.off('storehouseUpdate');
});

const getCategoryIcon = (category: string) => {
  switch(category) {
    case 'photo': return '🖼️';
    case 'audio': return '🎵';
    case 'video': return '📽️';
    case 'document': return '📄';
    default: return '📦';
  }
};

const openItem = (item: any) => {
  selectedItem.value = item;
  showModal.value = true;
};

const getItemThumbnail = (item: any) => {
  if (item.category === 'photo') {
    return apiService.getStorehouseFileUrl(item.file_id, item.source);
  }
  return null;
};
</script>

<template>
  <div class="storehouse-view">
    <header class="view-header">
      <div class="header-content">
        <h2>📦 Media Storehouse v2.6</h2>
        <p>跨平台媒體備份中心（支援 Gemini 多模態語意搜尋）</p>
      </div>
    </header>

    <div class="filter-controls card glow">
      <div class="search-row">
        <div class="mode-toggle">
          <button 
            :class="{ active: searchMode === 'keyword' }" 
            @click="searchMode = 'keyword'"
          >⌨️ 關鍵字</button>
          <button 
            :class="{ active: searchMode === 'semantic' }" 
            @click="searchMode = 'semantic'"
          >🤖 語意搜尋</button>
        </div>
        <div class="search-input-wrapper">
          <span class="search-icon">{{ searchMode === 'semantic' ? '🧠' : '🔍' }}</span>
          <input 
            v-model="searchQuery" 
            type="text" 
            :placeholder="searchMode === 'semantic' ? '用描述的方式找東西，例如：那張有貓的照片...' : '搜尋標題或備註關鍵字...'" 
          />
        </div>
      </div>
      
      <div class="filter-grid">
        <div class="filter-group">
          <label>來源平台</label>
          <select v-model="filterPlatform">
            <option value="">全部平台</option>
            <option value="telegram">Telegram</option>
            <option value="discord">Discord</option>
            <option value="line">LINE</option>
          </select>
        </div>

        <div class="filter-group">
          <label>開始日期</label>
          <input v-model="filterStartDate" type="date" :disabled="searchMode === 'semantic'" />
        </div>

        <div class="filter-group">
          <label>結束日期</label>
          <input v-model="filterEndDate" type="date" :disabled="searchMode === 'semantic'" />
        </div>

        <div class="filter-group">
          <label>上限筆數</label>
          <select v-model="limit">
            <option :value="5">5 筆</option>
            <option :value="10">10 筆</option>
            <option :value="20">20 筆 (預設)</option>
            <option :value="50">50 筆</option>
          </select>
        </div>
      </div>
    </div>

    <div v-if="loading" class="loader-container">
      <div class="spinner"></div>
      <p>{{ searchMode === 'semantic' ? 'AI 正在分析相似向量...' : '正在同步媒體倉庫...' }}</p>
    </div>

    <div v-else class="content-area">
      <div v-if="items.length === 0" class="empty-state card">
        <div class="empty-icon">📭</div>
        <h3>找不到任何資源</h3>
        <p>試著調整搜尋字眼，或確保你要找的資源已經建立「AI 語意索引」。</p>
      </div>
      
      <div v-else class="items-grid">
        <div 
          v-for="item in items" 
          :key="item.id" 
          class="item-card card clickable animate-pop-in"
          @click="openItem(item)"
        >
          <div class="card-media">
            <template v-if="getItemThumbnail(item)">
              <img :src="getItemThumbnail(item)" loading="lazy" />
            </template>
            <div v-else class="media-placeholder">
              {{ getCategoryIcon(item.category) }}
            </div>
            <div class="item-badge" :class="item.source">{{ item.source }}</div>
            <div v-if="item.index_status === 'indexed'" class="ai-badge" title="AI Indexed">✨</div>
          </div>
          
          <div class="card-content">
            <h4 class="item-title">{{ item.title }}</h4>
            <div class="item-meta">
              <span class="sender">👤 {{ item.sender || 'Unknown' }}</span>
              <span class="date">{{ new Date(item.created_at).toLocaleDateString() }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="items.length > 0 && searchMode !== 'semantic'" class="pagination">
      <button :disabled="page === 1" @click="page--" class="nav-btn">⬅️ 上一頁</button>
      <span class="page-indicator">第 {{ page }} 頁</span>
      <button :disabled="items.length < limit" @click="page++" class="nav-btn">下一頁 ➡️</button>
    </div>

    <StoreItemModal 
      :item="selectedItem" 
      :show="showModal" 
      @close="showModal = false" 
      @updated="loadItems"
    />
  </div>
</template>

<style scoped>
.storehouse-view {
  display: flex; flex-direction: column; gap: 2rem; padding-bottom: 5rem;
}

.view-header h2 {
  font-size: 2rem;
  background: linear-gradient(135deg, var(--primary-color), #a855f7);
  -webkit-background-clip: text; -webkit-text-fill-color: transparent;
  margin-bottom: 0.5rem;
}

.filter-controls {
  padding: 1.5rem; display: flex; flex-direction: column; gap: 1.5rem;
  border: 1px solid rgba(255,255,255,0.05);
  background: rgba(var(--primary-rgb), 0.05);
}

.search-row { display: flex; gap: 1rem; align-items: stretch; margin-bottom: 0.5rem; }

.mode-toggle {
  display: flex; background: rgba(0,0,0,0.3); border-radius: 12px; padding: 4px; gap: 4px;
}

.mode-toggle button {
  border: none; background: transparent; color: white; padding: 0 1rem;
  border-radius: 8px; cursor: pointer; transition: all 0.3s; font-size: 0.9rem;
  white-space: nowrap;
}

.mode-toggle button.active {
  background: var(--primary-color); box-shadow: 0 4px 10px rgba(var(--primary-rgb), 0.4);
}

.search-input-wrapper { position: relative; flex: 1; display: flex; align-items: center; }
.search-icon { position: absolute; left: 1rem; opacity: 0.7; }

.search-input-wrapper input {
  width: 100%; padding: 1rem 1rem 1rem 3rem; border-radius: 12px;
  border: 1px solid rgba(255,255,255,0.1); background: rgba(0,0,0,0.2);
  color: white; font-size: 1.1rem; transition: all 0.3s;
}

.search-input-wrapper input:focus {
  border-color: var(--primary-color); outline: none; background: rgba(var(--primary-rgb), 0.05);
}

.filter-grid { 
  display: grid; 
  grid-template-columns: repeat(4, 1fr); 
  gap: 1.5rem; 
  align-items: flex-end; 
}

@media (max-width: 1000px) {
  .filter-grid { grid-template-columns: repeat(2, 1fr); }
}

@media (max-width: 600px) {
  .filter-grid { grid-template-columns: 1fr; }
  .search-row { flex-direction: column; }
}

.filter-group { display: flex; flex-direction: column; gap: 0.5rem; }
.filter-group label { font-size: 0.85rem; font-weight: bold; opacity: 0.7; pointer-events: none; }
.filter-group select, .filter-group input {
  padding: 0.8rem; border-radius: 10px; background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1); color: white; cursor: pointer;
  transition: all 0.2s;
}

.filter-group select:hover, .filter-group input:hover {
  background: rgba(255,255,255,0.1); border-color: rgba(var(--primary-rgb), 0.5);
}

.filter-group input:disabled { opacity: 0.3; cursor: not-allowed; }

.items-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.5rem;
}

.item-card {
  height: 100%; padding: 0; overflow: hidden;
  border: 1px solid rgba(255,255,255,0.05);
  display: flex; flex-direction: column;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.item-card:hover { rotate: 0.5deg; transform: translateY(-8px); border-color: var(--primary-color); }

.card-media {
  height: 180px; background: #111; position: relative;
  display: flex; align-items: center; justify-content: center; overflow: hidden;
}

.card-media img { width: 100%; height: 100%; object-fit: cover; }
.media-placeholder { font-size: 4rem; opacity: 0.5; }

.item-badge {
  position: absolute; top: 0.8rem; right: 0.8rem;
  padding: 0.3rem 0.6rem; border-radius: 6px; font-size: 0.7rem;
  background: rgba(0,0,0,0.7); backdrop-filter: blur(4px);
}
.item-badge.telegram { color: #26a5e4; border: 1px solid #26a5e4; }

.ai-badge {
  position: absolute; bottom: 0.8rem; right: 0.8rem;
  background: var(--primary-color); color: white;
  width: 24px; height: 24px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.8rem; box-shadow: 0 0 10px var(--primary-color);
}

.card-content { padding: 1.2rem; flex: 1; display: flex; flex-direction: column; gap: 0.8rem; }
.item-title { margin: 0; font-size: 1.1rem; line-height: 1.4; overflow: hidden; }
.item-meta { display: flex; justify-content: space-between; font-size: 0.8rem; opacity: 0.6; }

.loader-container { text-align: center; padding: 5rem; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 2rem; }

.nav-btn {
  padding: 0.8rem 1.5rem; border-radius: 12px;
  background: rgba(var(--primary-rgb), 0.1); border: 1px solid var(--primary-color);
  color: white; cursor: pointer;
}

.animate-pop-in { animation: popIn 0.4s ease-out backwards; }
@keyframes popIn {
  from { opacity: 0; transform: scale(0.9) translateY(20px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}
</style>
