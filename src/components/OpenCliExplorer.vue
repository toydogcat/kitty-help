<script setup lang="ts">
import { ref, computed } from 'vue';
import { apiService } from '../services/api';
import { marked } from 'marked';

const args = ref('hackernews top');
const loading = ref(false);
const result = ref<any>(null);
const rawOutput = ref<string>('');
const error = ref<string | null>(null);
const viewMode = ref<'pretty' | 'raw'>('pretty');

const ALL_PLATFORMS = [
  // Fixed Top 3
  { name: 'HN Top', cmd: 'hackernews top', color: '#ff6600' },
  { name: 'HN AI 搜尋', cmd: 'hackernews search AI', color: '#00d2ff' },
  { name: 'BBC News', cmd: 'bbc news', color: '#ff0000' },
  
  // News & Dev
  { name: 'V2EX(熱門)', cmd: 'v2ex hot' },
  { name: 'V2EX(最新)', cmd: 'v2ex latest' },
  { name: 'ProductHunt', cmd: 'producthunt today' },
  { name: 'Lobsters', cmd: 'lobsters hot' },
  { name: 'GitHub(趨勢)', cmd: 'github trending' },
  { name: 'Arxiv AI', cmd: 'arxiv cs.AI' },
  { name: 'Arxiv ML', cmd: 'arxiv stat.ML' },
  { name: 'Dev.to Top', cmd: 'devto top' },
  { name: 'HackerNews New', cmd: 'hackernews newest' },
  { name: 'StackOverflow', cmd: 'stackoverflow newest' },
  
  // Tech & Finance
  { name: '36Kr 最新', cmd: '36kr latest' },
  { name: 'Bloomberg', cmd: 'bloomberg news' },
  { name: 'Reuters World', cmd: 'reuters world' },
  { name: 'Yahoo Finance', cmd: 'yahoo-finance trending' },
  { name: 'Xueqiu Hot', cmd: 'xueqiu hot' },
  
  // Entertainment & Social
  { name: 'Bilibili Hot', cmd: 'bilibili hot' },
  { name: 'Douyin Hot', cmd: 'douyin hot' },
  { name: 'TikTok Trend', cmd: 'tiktok trending' },
  { name: 'YouTube Hot', cmd: 'youtube hot' },
  { name: 'Bluesky Hot', cmd: 'bluesky trending' },
  { name: 'Reddit Tech', cmd: 'reddit technology' },
  { name: 'Reddit AI', cmd: 'reddit artificial' },
  { name: 'Weibo Hot', cmd: 'weibo hot' },
  { name: 'Tieba Hot', cmd: 'tieba top' },
  { name: 'Zhihu Hot', cmd: 'zhihu hoy' },
  { name: 'Smzdm Ranking', cmd: 'smzdm ranking' },
  { name: 'Jike Rec', cmd: 'jike recommendation' },
  
  // Specialized & Misc
  { name: 'IMDb Top', cmd: 'imdb top' },
  { name: 'Douban Movie', cmd: 'douban movie' },
  { name: 'HF Trending', cmd: 'hf trending' },
  { name: 'Pixiv Daily', cmd: 'pixiv daily' },
  { name: 'Steam Top', cmd: 'steam top-sellers' },
  { name: 'Substack Top', cmd: 'substack top' },
  { name: 'Wikipedia', cmd: 'wikipedia featured' },
  { name: 'Xiaohongshu', cmd: 'xiaohongshu hot' },
  { name: 'Xiaoyuzhou', cmd: 'xiaoyuzhou trending' },
  { name: 'WeRead Rank', cmd: 'weread ranking' },
  { name: 'Boss Jobs', cmd: 'boss ai-jobs' },
  { name: 'Arxiv CV', cmd: 'arxiv cs.CV' },
  { name: 'Google News', cmd: 'google news' },
  { name: 'Medium Trend', cmd: 'medium trending' },
  { name: 'Vercel Info', cmd: 'vercel info' },
  { name: 'PaperReview', cmd: 'paperreview' },
  { name: 'Zsxq Top', cmd: 'zsxq top' },
  { name: 'Band News', cmd: 'band news' },
  { name: 'Coupang Hot', cmd: 'coupang hot' }
];

const fixedPlatforms = ALL_PLATFORMS.slice(0, 3);
const dynamicPlatforms = ref<any[]>([]);

const shuffle = () => {
    const remaining = ALL_PLATFORMS.slice(3);
    const shuffled = [...remaining].sort(() => 0.5 - Math.random());
    dynamicPlatforms.value = shuffled.slice(0, 6);
};

// Initial shuffle
shuffle();

const isJsonArray = computed(() => Array.isArray(result.value));

// Web Reader Section
const readerUrl = ref('');
const readerResult = ref('');
const readerLoading = ref(false);
const readerError = ref<string | null>(null);

const runReader = async () => {
    if (!readerUrl.value) return;
    
    readerLoading.value = true;
    readerError.value = null;
    readerResult.value = '';
    
    try {
        const cmd = `web read --url "${readerUrl.value}" --content -f md --strategy public`;

        console.log("Executing Reader Command:", cmd);
        
        const res = await apiService.runOpenCLI(cmd);
        console.log("Reader Response:", res);
        
        if (res.status === 'success') {
            readerResult.value = res.stdout;
        } else {
            readerError.value = res.error || 'Failed to read URL';
        }
    } catch (err: any) {
        console.error("Reader Exception:", err);
        readerError.value = `Error: ${err.response?.data?.error || err.message}`;
    } finally {
        readerLoading.value = false;
    }
};

const renderedMarkdown = computed(() => {
    if (!readerResult.value) return '';
    return marked.parse(readerResult.value);
});

// Smart Parser for Markdown/ASCII tables
const parsedData = computed(() => {
  if (!rawOutput.value) return null;
  const lines = rawOutput.value.split('\n');
  
  // Find lines that look like table rows (must contain at least 2 pipes)
  const pipeLines = lines.filter(l => l.includes('|'));
  if (pipeLines.length < 2) return null;

  try {
    // Filter out separator lines that look like |---+---| or +---+
    const dataLines = pipeLines.filter(l => {
      const clean = l.trim();
      // Ignore if it's purely symbols like +---+--- or |---|---
      if (!/[a-zA-Z0-9]/.test(clean)) return false; 
      return true;
    });

    if (dataLines.length < 2) return null;

    // The first data line is likely our header
    const headers = dataLines[0].split('|').map(h => h.trim()).filter(h => h !== '');
    
    const rows = dataLines.slice(1).map(line => {
      // Split by pipe and remove empty strings from ends
      const parts = line.split('|').map(p => p.trim());
      // If line started/ended with |, the first/last parts will be empty
      const cleanParts = line.trim().startsWith('|') ? parts.slice(1) : parts;
      
      const obj: any = {};
      headers.forEach((h, i) => {
        const key = h.toLowerCase();
        obj[key] = cleanParts[i] || '';
      });
      return obj;
    });

    return { headers, rows };
  } catch (e) {
    console.error('Table parsing failed:', e);
    return null;
  }
});

const runCommand = async (customArgs?: string) => {
  let finalArgs = (customArgs || args.value).trim();
  if (!finalArgs) return;

  // Auto-append -f json if it's a known exploration command and no format specified
  if (!finalArgs.includes('-f ') && !finalArgs.includes('--format') && !finalArgs.includes('--help') && !finalArgs.includes('-h')) {
    // Basic whitelist: if command starts with something other than 'opencli', assume it's a plugin cmd
    // Or just append it if it looks like a news/list command
    finalArgs += ' -f json';
  }
  
  loading.value = true;
  error.value = null;
  result.value = null;
  rawOutput.value = '';
  
  try {
    const res = await apiService.runOpenCLI(finalArgs);
    if (res.status === 'success') {
      rawOutput.value = res.stdout;
      try {
        // Try to parse JSON from stdout
        const parsed = JSON.parse(res.stdout);
        // Ensure it's what we want (an array or object)
        result.value = parsed;
      } catch {
        // Fallback to raw text
        result.value = res.stdout || res.stderr;
      }
    } else {
      error.value = res.error || 'Command failed';
      rawOutput.value = res.stderr || 'No output';
    }
  } catch (err: any) {
    error.value = err.response?.data?.error || err.message;
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <div class="opencli-explorer card glow">
    <div class="explorer-header">
      <div class="header-titles">
        <div class="title-row">
          <h3>🤖 AI Info Explorer</h3>
          <div class="view-toggle" v-if="parsedData">
            <button @click="viewMode = 'pretty'" :class="{ active: viewMode === 'pretty' }">Pretty</button>
            <button @click="viewMode = 'raw'" :class="{ active: viewMode === 'raw' }">Raw</button>
          </div>
        </div>
        <p class="subtitle">Discovery wall powered by Document Chicken</p>
      </div>
      <div class="header-actions">
        <button @click="shuffle" class="refresh-btn" :disabled="loading">
            <span class="refresh-icon">🔄</span> Change / 換一換
        </button>
      </div>
    </div>

    <div class="discovery-grid">
        <button 
          v-for="(p, idx) in [...fixedPlatforms, ...dynamicPlatforms]" 
          :key="p.name"
          @click="runCommand(p.cmd)"
          class="discovery-chip"
          :class="{ 'fixed-chip': idx < 3 }"
          :style="idx < 3 ? { '--chip-color': p.color, '--chip-border': p.color + '44' } : {}"
          :disabled="loading"
        >
          {{ p.name }}
        </button>
    </div>

    <div class="content-view" v-if="result || error || loading">
      <!-- Loading State -->
      <div v-if="loading && !rawOutput" class="loading-state">
        <div class="pulse-loader"></div>
        <span>AI is scanning the web...</span>
      </div>

      <!-- Error State -->
      <div v-if="error" class="error-box">
        <span class="error-type">⚠️ Worker Exception</span>
        <pre>{{ error }}</pre>
      </div>

      <!-- Structured JSON List View -->
      <div v-if="isJsonArray && viewMode === 'pretty'" class="pretty-list">
        <div v-for="(item, idx) in result" :key="idx" class="news-card">
          <div class="card-rank">{{ Number(item.rank || item.Rank || 0) + Number(idx + 1) }}</div>
          <div class="card-body">
            <div class="title-meta-row">
                 <h4 class="card-title">{{ item.title || item.Title || item.name || 'Untitled' }}</h4>
                 <a v-if="item.url || item.Url" :href="item.url || item.Url" target="_blank" class="link-icon" title="Open Source">🔗</a>
            </div>
            <p class="card-desc" v-if="item.description || item.Description || item.summary">
              {{ item.description || item.Description || item.summary }}
            </p>
            <div class="card-meta">
              <span v-if="item.score || item.Score">🔥 {{ item.score || item.Score }}</span>
              <span v-if="item.author || item.Author">👤 {{ item.author || item.Author }}</span>
              <span v-if="item.comments || item.Comments">💬 {{ item.comments || item.Comments }}</span>
              <span v-if="item.time || item.Time" class="time">🕒 {{ item.time || item.Time }}</span>
              <span v-if="item.node_type" class="type-tag">{{ item.node_type }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Fallback: Multi-line ASCII Table Parsing (Existing) -->
      <div v-else-if="parsedData && viewMode === 'pretty'" class="pretty-list">
        <div v-for="(item, idx) in parsedData.rows" :key="idx" class="news-card">
          <div class="card-rank">{{ item.rank || item['#'] || (idx + 1) }}</div>
          <div class="card-body">
            <h4 class="card-title">{{ item.title || item.name }}</h4>
            <p class="card-desc" v-if="item.description || item.subtitle || item.summary">
              {{ item.description || item.subtitle || item.summary }}
            </p>
            <div class="card-meta">
              <span v-if="item.score">🔥 {{ item.score }}</span>
              <span v-if="item.author">👤 {{ item.author }}</span>
              <span v-if="item.comments">💬 {{ item.comments }}</span>
              <span v-if="item.time" class="time">🕒 {{ item.time }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Raw/JSON Mode -->
      <div v-else-if="rawOutput && !loading" class="raw-viewer">
        <div class="viewer-tabs">
          <span>Terminal Output</span>
          <button @click="rawOutput = ''; result = null">Clear</button>
        </div>
        <pre class="raw-content"><code>{{ rawOutput }}</code></pre>
      </div>
    </div>

    <!-- Web Reader Section -->
    <div class="web-reader-divider">
        <hr />
        <span>🌐 WEB READER</span>
    </div>

    <div class="web-reader-input">
        <div class="reader-input-wrapper">
             <input 
                v-model="readerUrl" 
                placeholder="Paste URL here (e.g. https://www.sanmin.com.tw/)" 
                @keyup.enter="runReader"
                :disabled="readerLoading"
            />
        </div>
        <button @click="runReader" class="reader-btn" :disabled="readerLoading">
            <span v-if="readerLoading" class="loader small"></span>
            <span v-else>READ</span>
        </button>
    </div>

    <div class="reader-view" v-if="readerResult || readerError || readerLoading">
        <div v-if="readerLoading && !readerResult" class="reader-loading">
             <div class="pulse-loader small"></div>
             <span>Reading web content...</span>
        </div>
        <div v-if="readerError" class="reader-error">
             ⚠️ {{ readerError }}
        </div>
        <div v-if="readerResult" class="markdown-body" v-html="renderedMarkdown"></div>
    </div>
  </div>
</template>

<style scoped>
.opencli-explorer {
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
  background: rgba(var(--primary-rgb), 0.04);
  border: 1px solid rgba(var(--primary-rgb), 0.2);
}

.explorer-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 1rem;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.view-toggle {
  display: flex;
  background: rgba(0,0,0,0.3);
  padding: 2px;
  border-radius: 6px;
}

.view-toggle button {
  padding: 4px 10px;
  font-size: 0.7rem;
  background: transparent;
  border: none;
  color: white;
  opacity: 0.5;
  cursor: pointer;
  transition: all 0.2s;
}

.view-toggle button.active {
  background: var(--primary-color);
  opacity: 1;
  border-radius: 4px;
  font-weight: bold;
}

.subtitle {
  font-size: 0.8rem;
  opacity: 0.6;
}

.header-actions {
  display: flex;
  gap: 0.8rem;
}

.refresh-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 1.2rem;
    background: rgba(var(--primary-rgb), 0.1);
    color: var(--primary-color);
    border: 1px solid rgba(var(--primary-rgb), 0.3);
    border-radius: 10px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.3s;
}

.refresh-btn:hover:not(:disabled) {
    background: var(--primary-color);
    color: white;
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(var(--primary-rgb), 0.3);
}

.refresh-icon {
    font-size: 1rem;
    transition: transform 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.refresh-btn:hover .refresh-icon {
    transform: rotate(180deg);
}

.discovery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 0.8rem;
  margin-top: 0.5rem;
}

.discovery-chip {
  padding: 0.7rem 1rem;
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 12px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.discovery-chip:hover:not(:disabled) {
  background: rgba(255,255,255,0.08);
  border-color: rgba(255,255,255,0.2);
  transform: translateY(-3px);
  box-shadow: 0 4px 15px rgba(0,0,0,0.2);
}

.fixed-chip {
    border-color: var(--chip-border) !important;
    color: var(--chip-color) !important;
    font-weight: 900 !important;
    letter-spacing: 0.5px;
    text-transform: uppercase;
}

.fixed-chip:hover:not(:disabled) {
    background: var(--chip-border) !important;
    box-shadow: 0 4px 15px var(--chip-border) !important;
}

.content-view {
  min-height: 150px;
  max-height: 600px;
  background: rgba(0,0,0,0.2);
  border-radius: 12px;
  overflow-y: auto;
  border: 1px solid rgba(255,255,255,0.05);
}

/* Pretty View List Styles */
.pretty-list {
  display: flex;
  flex-direction: column;
}

.news-card {
  display: flex;
  gap: 1.2rem;
  padding: 1.2rem;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  transition: background 0.2s;
}

.news-card:hover {
  background: rgba(255,255,255,0.02);
}

.card-rank {
  font-size: 1.5rem;
  font-weight: 900;
  color: var(--primary-color);
  opacity: 0.3;
  min-width: 30px;
  text-align: right;
  font-family: 'Outfit', sans-serif;
}

.card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.card-title {
  font-size: 1.1rem;
  line-height: 1.4;
  font-weight: 700;
  color: #f8fafc;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.title-meta-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
}

.link-icon {
    font-size: 1rem;
    text-decoration: none;
    opacity: 0.5;
    transition: opacity 0.2s;
    flex-shrink: 0;
    margin-top: 2px;
}

.link-icon:hover {
    opacity: 1;
}

.card-desc {
  font-size: 0.92rem;
  opacity: 0.7;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: #cbd5e1;
}

.card-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  font-size: 0.75rem;
  font-weight: 600;
  opacity: 0.6;
  margin-top: 0.4rem;
}

.type-tag {
    background: rgba(var(--primary-rgb), 0.15);
    color: var(--primary-color);
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.65rem;
    text-transform: uppercase;
}

/* Raw/Terminal Viewer */
.raw-viewer {
  padding: 0;
}

.viewer-tabs {
  display: flex;
  justify-content: space-between;
  padding: 0.6rem 1rem;
  background: rgba(255,255,255,0.05);
  font-size: 0.7rem;
  opacity: 0.6;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.raw-content {
  margin: 0;
  padding: 1.5rem;
  font-size: 0.85rem;
  color: #ccc;
  font-family: 'Fira Code', 'Courier New', monospace;
  overflow: auto; /* Supports both X and Y scrolling */
  white-space: pre; /* Don't wrap, allow horizontal scrolling */
  min-width: fit-content;
}

.loading-state {
  height: 200px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 1.5rem;
}

.pulse-loader {
  width: 50px;
  height: 50px;
  background: var(--primary-color);
  border-radius: 50%;
  animation: s-pulse 2s infinite ease-in-out;
}

@keyframes s-pulse {
  0% { transform: scale(0.6); opacity: 0.6; }
  50% { transform: scale(1); opacity: 0.2; }
  100% { transform: scale(0.6); opacity: 0.6; }
}

.error-box {
  padding: 2rem;
  background: rgba(255, 77, 77, 0.05);
}

.error-type { color: #fe4a49; font-weight: bold; }

::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-thumb { background: rgba(var(--primary-rgb), 0.2); border-radius: 10px; }
.web-reader-divider {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin: 1.5rem 0 0.5rem;
    opacity: 0.3;
}

.web-reader-divider hr {
    flex: 1;
    border: none;
    border-top: 1px solid white;
}

.web-reader-divider span {
    font-size: 0.7rem;
    font-weight: 900;
    letter-spacing: 2px;
}

.web-reader-input {
    display: flex;
    gap: 0.8rem;
    margin-bottom: 1rem;
}

.reader-input-wrapper {
    flex: 1;
    background: rgba(var(--primary-rgb), 0.05);
    border: 1px solid rgba(var(--primary-rgb), 0.2);
    border-radius: 10px;
    padding: 0 1rem;
}

.reader-input-wrapper input {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    color: white;
    padding: 0.75rem 0;
    font-size: 0.9rem;
}

.reader-btn {
    padding: 0 1.5rem;
    background: var(--primary-color);
    color: white;
    border: none;
    border-radius: 10px;
    font-weight: 800;
    cursor: pointer;
    transition: all 0.3s;
}

.reader-btn:hover:not(:disabled) {
    box-shadow: 0 0 15px rgba(var(--primary-rgb), 0.4);
    transform: translateY(-2px);
}

.reader-view {
    background: rgba(0,0,0,0.3);
    border-radius: 12px;
    border: 1px solid rgba(255,255,255,0.05);
    min-height: 100px;
}

.reader-loading, .reader-error {
    padding: 2rem;
    text-align: center;
}

.reader-error { color: #fe4a49; }

/* Markdown Body Styles */
.markdown-body {
    padding: 2rem;
    color: #e2e8f0;
    line-height: 1.7;
    font-size: 1rem;
}

.markdown-body :deep(h1), .markdown-body :deep(h2), .markdown-body :deep(h3) {
    color: var(--primary-color);
    margin: 1.5rem 0 1rem;
    font-weight: 800;
}

.markdown-body :deep(h1) { font-size: 1.8rem; border-bottom: 1px solid rgba(var(--primary-rgb), 0.3); padding-bottom: 0.5rem; }
.markdown-body :deep(h2) { font-size: 1.4rem; }
.markdown-body :deep(h3) { font-size: 1.2rem; }

.markdown-body :deep(p) { margin-bottom: 1rem; }

.markdown-body :deep(ul), .markdown-body :deep(ol) {
    margin-bottom: 1rem;
    padding-left: 1.5rem;
}

.markdown-body :deep(li) { margin-bottom: 0.5rem; }

.markdown-body :deep(a) {
    color: var(--primary-color);
    text-decoration: underline;
    opacity: 0.8;
}

.markdown-body :deep(a:hover) { opacity: 1; }

.markdown-body :deep(code) {
    background: rgba(255,255,255,0.1);
    padding: 2px 5px;
    border-radius: 4px;
    font-family: 'Fira Code', monospace;
    font-size: 0.9em;
}

.markdown-body :deep(pre) {
    background: #0f172a;
    padding: 1.5rem;
    border-radius: 8px;
    overflow-x: auto;
    margin-bottom: 1.5rem;
    border: 1px solid rgba(255,255,255,0.1);
}

.markdown-body :deep(blockquote) {
    border-left: 4px solid var(--primary-color);
    padding-left: 1.5rem;
    margin: 1.5rem 0;
    opacity: 0.8;
    font-style: italic;
}

.markdown-body :deep(table) {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 1.5rem;
}

.markdown-body :deep(th), .markdown-body :deep(td) {
    border: 1px solid rgba(255,255,255,0.1);
    padding: 0.75rem;
    text-align: left;
}

.markdown-body :deep(th) {
    background: rgba(var(--primary-rgb), 0.1);
    font-weight: 800;
}

.markdown-body :deep(img) {
    max-width: 100%;
    border-radius: 8px;
    margin: 1rem 0;
}

.loader.small { width: 16px; height: 16px; }
.pulse-loader.small { width: 30px; height: 30px; }

@media (max-width: 600px) {
  .opencli-explorer {
    gap: 0.8rem !important;
  }
  
  .subtitle {
    display: none;
  }

  .news-card {
    padding: 0.8rem !important;
    gap: 0.8rem;
  }

  .card-rank {
    font-size: 1.1rem;
    min-width: 20px;
  }

  .card-title {
    font-size: 0.95rem;
  }

  .card-meta {
    gap: 0.6rem;
    font-size: 0.68rem;
  }

  .markdown-body {
    padding: 1rem;
    font-size: 0.9rem;
  }
}
</style>
