<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { Network } from 'vis-network';
import { DataSet } from 'vis-data';
import { apiService } from '../services/api';
import { useAuth } from '../composables/useAuth';

const { } = useAuth();
const canvas = ref<HTMLElement | null>(null);
const network = ref<Network | null>(null);
const importFileRef = ref<HTMLInputElement | null>(null);

// Data
const tempItems = ref<any[]>([]);
const centerNodeId = ref<string | null>(localStorage.getItem('impression_last_center') || null);

// UI State
const showAddNodeModal = ref(false);
const selectedTempItem = ref<any>(null);
const newNodeData = ref({ title: '', content: '', nodeType: 'general' });
const isLoading = ref(false);

const searchQ2 = ref('');
const searchResults2 = ref<any[]>([]);
const targetNode = ref<any>(null);
const linkLabel = ref('');

const selectedNodeDetails = ref<any>(null);
const isEditingNode = ref(false);
const editForm = ref({ title: '', content: '', nodeType: 'general' });

const showTempGallery = ref(localStorage.getItem('impression_gallery_show') !== 'false');
const globalSearchQuery = ref('');
const globalSearchResults = ref<any[]>([]);
const isLinkingMode = ref(false);

// Note Editing
const isEditingNoteContent = ref(false);
const linkedNoteData = ref({ id: '', name: '', content: '' });

// Physics and Export
const isPhysicsEnabled = ref(true);
const exportBgColor = ref('#0f172a'); 
const exportBgImage = ref<string | null>(null);
const showExportPanel = ref(false);

const defaultBackgrounds = [
    { name: 'Cosmic', url: 'https://images.unsplash.com/photo-1462331940025-496dfbfc7564?auto=format&fit=crop&w=1920&q=80' },
    { name: 'Quantum', url: 'https://images.unsplash.com/photo-1451187580459-43490279c0fa?auto=format&fit=crop&w=1920&q=80' },
    { name: 'Aurora', url: 'https://images.unsplash.com/photo-1483366759020-137255160894?auto=format&fit=crop&w=1920&q=80' },
    { name: 'Vortex', url: 'https://images.unsplash.com/photo-1543722530-d2c3201371e7?auto=format&fit=crop&w=1920&q=80' }
];

const nodes = new DataSet<any>([]);
const edges = new DataSet<any>([]);

const options = {
  nodes: {
    shape: 'dot', size: 30,
    font: { size: 14, color: '#ffffff', strokeWidth: 0, face: 'Inter, system-ui' },
    borderWidth: 3,
    image: { crossOrigin: 'anonymous' },
    shadow: { enabled: true, color: 'rgba(0,0,0,0.4)', size: 8, x: 0, y: 4 },
  },
  edges: {
    width: 2, color: { color: 'rgba(255,255,255,0.15)', highlight: '#22d3ee' },
    font: { size: 11, color: '#94a3b8', align: 'top' },
    smooth: { enabled: true, type: 'continuous', roundness: 0.5 },
    arrows: { to: { enabled: true, scaleFactor: 0.4 } }
  },
  physics: {
    enabled: true,
    forceAtlas2Based: { gravitationalConstant: -100, springLength: 200, springConstant: 0.04 },
    solver: 'forceAtlas2Based',
    stabilization: { iterations: 100 }
  },
  interaction: { hover: true, navigationButtons: false, keyboard: true, zoomView: true }
};

const saveViewState = () => {
    if (!network.value) return;
    const pos = network.value.getViewPosition();
    const scale = network.value.getScale();
    localStorage.setItem('impression_view_state', JSON.stringify({ x: pos.x, y: pos.y, scale }));
};

const initGraph = () => {
  if (!canvas.value) return;
  network.value = new Network(canvas.value, { nodes, edges }, options);
  
  // Restore view state
  const savedState = localStorage.getItem('impression_view_state');
  if (savedState) {
      const { x, y, scale } = JSON.parse(savedState);
      network.value.moveTo({ position: { x, y }, scale, animation: false });
  }

  network.value.on('click', (params) => {
    isLinkingMode.value = false;
    isEditingNode.value = false;
    isEditingNoteContent.value = false;
    if (params.nodes.length > 0) {
      handleNodeClick(params.nodes[0]);
    } else {
      selectedNodeDetails.value = null;
    }
  });

  network.value.on('dragEnd', saveViewState);
  network.value.on('zoom', saveViewState);
};

watch(showTempGallery, (newVal) => {
    localStorage.setItem('impression_gallery_show', newVal ? 'true' : 'false');
});

watch(isPhysicsEnabled, (newVal) => {
    if (network.value) network.value.setOptions({ physics: { enabled: newVal } });
});

const handleNodeClick = (nodeId: string) => {
  const node = nodes.get(nodeId);
  if (node) {
      selectedNodeDetails.value = node.raw;
      editForm.value = { ...node.raw };
  }
};

const loadGraph = async (nodeId?: string) => {
  isLoading.value = true;
  try {
    const data = await apiService.getImpressionGraph(nodeId || centerNodeId.value || '');
    
    const visNodes = data.nodes.map((n: any) => {
        let finalUrl = n.imageUrl ? apiService.getAbsoluteUrl(n.imageUrl) : '';
        // 🚨 CRITICAL FIX: Use 'fileId' from the record, NOT the node 'id'
        if (!finalUrl && n.fileId) finalUrl = apiService.getStorehouseFileUrl(n.fileId, n.sourcePlatform);
        
        if (finalUrl && !finalUrl.includes('?t=')) {
           const sep = finalUrl.includes('?') ? '&' : '?';
           finalUrl = `${finalUrl}${sep}t=${Date.now()}`;
        }
        n.imageUrl = finalUrl; // CRITICAL: Update the raw object so the card gets it!
        return {
            id: n.id, label: n.title, shape: n.imageUrl || n.fileId ? 'circularImage' : 'dot',
            image: finalUrl || undefined,
            color: { border: n.id === (nodeId || centerNodeId.value) ? '#22d3ee' : '#4338ca', background: '#1e293b' },
            raw: n
        };
    });

    nodes.clear();
    nodes.add(visNodes);
    edges.clear();
    edges.add(data.edges.map((e: any) => ({ id: e.id, from: e.sourceId, to: e.targetId, label: e.label })));
    
    // SMART CENTERING: 
    // If we specifically requested a nodeId (search/random), JUMP to it.
    // Otherwise, try to restore view state.
    if (nodeId && network.value) {
        setTimeout(() => {
            network.value?.fit({ nodes: [nodeId], animation: true });
            centerNodeId.value = nodeId;
            localStorage.setItem('impression_last_center', nodeId);
        }, 200);
    } else if (localStorage.getItem('impression_view_state') && network.value) {
        const { x, y, scale } = JSON.parse(localStorage.getItem('impression_view_state')!);
        network.value.moveTo({ position: { x, y }, scale, animation: false });
    } else if (data.nodes.length > 0 && network.value) {
        network.value.fit({ animation: true });
    }
  } catch (e) { console.error(e); } finally { isLoading.value = false; }
};

const exportGraphData = async () => {
    try {
        const data = await apiService.exportImpressionGraph();
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `impression_backup_${Date.now()}.json`;
        link.click();
    } catch (e) { alert('Export failed'); }
};

const importGraphData = async (event: any) => {
    const file = event.target.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async (e) => {
        try {
            const data = JSON.parse(e.target?.result as string);
            await apiService.importImpressionGraph(data);
            alert('Universe Restored.');
            loadGraph();
        } catch (err) { alert('Import failed: Check file format'); }
    };
    reader.readAsText(file);
};

const goToRandomNode = async () => {
    try {
        const res = await apiService.getImpressionRandom();
        if (res.id) loadGraph(res.id);
    } catch (e) { console.error(e); }
};

const syncToSnippet = async (id: string) => {
    try {
        const res = await apiService.syncImpressionToSnippet(id);
        if (res.status === 'linked') alert('Linked Successfully! Check your Personal board.');
        else if (res.status === 'existing') {
             // Already linked, now fetch the content to edit
             const snippet = await apiService.getLinkedSnippet(res.snippetId);
             linkedNoteData.value = { id: snippet.id, name: snippet.name, content: snippet.content || '' };
             isEditingNoteContent.value = true;
             isEditingNode.value = false;
             isLinkingMode.value = false;
        }
        loadGraph(id); 
    } catch (e) { console.error(e); }
};

const openNoteEditor = async (snippetId: string) => {
    try {
        const snippet = await apiService.getLinkedSnippet(snippetId);
        linkedNoteData.value = { id: snippet.id, name: snippet.name, content: snippet.content || '' };
        isEditingNoteContent.value = true;
        isEditingNode.value = false;
        isLinkingMode.value = false;
    } catch (e) { console.error(e); }
};

const saveNoteChanges = async () => {
    try {
        await apiService.updateSnippet(linkedNoteData.value.id, { 
            name: linkedNoteData.value.name, 
            content: linkedNoteData.value.content 
        });
        isEditingNoteContent.value = false;
        alert('Note Manifest Refined.');
    } catch (e) { console.error(e); }
};

const saveNodeEdits = async () => {
    if (!selectedNodeDetails.value) return;
    try {
        const updated = await apiService.updateImpressionNode(selectedNodeDetails.value.id, editForm.value);
        selectedNodeDetails.value = updated;
        isEditingNode.value = false;
        loadGraph(updated.id);
    } catch (e) { console.error(e); }
};

const deleteNode = async (id: string) => {
    if (!confirm('Destroy this memory node?')) return;
    try { await apiService.deleteImpressionNode(id); selectedNodeDetails.value = null; await loadGraph(); } catch (e) { console.error(e); }
};


const exportAsImage = () => {
    if (!network.value || !canvas.value) return;
    network.value.fit({ animation: false });
    
    setTimeout(async () => {
        const originalCanvas = canvas.value!.getElementsByTagName('canvas')[0];
        if (!originalCanvas) return;
        
        const exportCanvas = document.createElement('canvas');
        exportCanvas.width = originalCanvas.width;
        exportCanvas.height = originalCanvas.height;
        const ctx = exportCanvas.getContext('2d');
        if (!ctx) return;

        // 1. Draw Background (Safe Loader)
        if (exportBgImage.value) {
            try {
                const bgImg = new Image();
                bgImg.crossOrigin = "anonymous";
                bgImg.src = exportBgImage.value;
                await new Promise((resolve, reject) => { 
                    bgImg.onload = resolve; 
                    bgImg.onerror = reject;
                    // Timeout fallback
                    setTimeout(resolve, 3000); 
                });
                
                const scale = Math.max(exportCanvas.width / bgImg.width, exportCanvas.height / bgImg.height);
                const x = (exportCanvas.width / 2) - (bgImg.width / 2) * scale;
                const y = (exportCanvas.height / 2) - (bgImg.height / 2) * scale;
                ctx.drawImage(bgImg, x, y, bgImg.width * scale, bgImg.height * scale);
            } catch (err) {
                // Image failed, use fallback color
                ctx.fillStyle = exportBgColor.value;
                ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
            }
        } else {
            ctx.fillStyle = exportBgColor.value;
            ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
        }

        // 2. Overlay Graph
        ctx.drawImage(originalCanvas, 0, 0);

        // 3. Download
        const link = document.createElement('a');
        link.download = `impression_art_${Date.now()}.png`;
        link.href = exportCanvas.toDataURL('image/png');
        link.click();
        showExportPanel.value = false;
    }, 200);
};

const loadTempItems = async () => {
  try {
    const data = await apiService.getImpressionTemp();
    tempItems.value = data.map((item: any) => {
        // Construct standard image URL using BOTH ID and Platform
        item.imageUrl = apiService.getStorehouseFileUrl(item.id, item.platform);
        
        // Cache busting
        const sep = item.imageUrl.includes('?') ? '&' : '?';
        item.imageUrl = `${item.imageUrl}${sep}t=${Date.now()}`;
        return item;
    });
  } catch (e) { console.error(e); }
};

const setAsCenter = (id: string) => { loadGraph(id); selectedNodeDetails.value = null; };
const openAddModal = (item: any) => {
  selectedTempItem.value = item;
  newNodeData.value = { title: item.title || item.caption || 'New Node', content: item.notes || '', nodeType: 'general' };
  showAddNodeModal.value = true;
};
const saveNode = async () => {
    if (!selectedTempItem.value) return;
    try {
        const newNode = await apiService.createImpressionNode({ mediaId: selectedTempItem.value.id, ...newNodeData.value });
        showAddNodeModal.value = false; await loadTempItems(); await loadGraph(newNode.id);
    } catch (e) { console.error(e); }
};
const createLink = async () => {
    if (!selectedNodeDetails.value || !targetNode.value) return;
    try {
        await apiService.createImpressionLink({ sourceId: selectedNodeDetails.value.id, targetId: targetNode.value.id, label: linkLabel.value });
        targetNode.value = null; isLinkingMode.value = false; linkLabel.value = ''; await loadGraph(selectedNodeDetails.value.id);
    } catch (e) { console.error(e); }
};
const performGlobalSearch = async () => {
    if (globalSearchQuery.value.length < 1) { globalSearchResults.value = []; return; }
    const results = await apiService.searchImpressionNodes(globalSearchQuery.value);
    globalSearchResults.value = results.map((r: any) => ({
        ...r,
        imageUrl: r.imageUrl?.startsWith('/') ? apiService.getAbsoluteUrl(r.imageUrl) : r.imageUrl
    }));
};
const performSearch2 = async () => {
    if (searchQ2.value.length < 1) { searchResults2.value = []; return; }
    const results = await apiService.searchImpressionNodes(searchQ2.value);
    searchResults2.value = results.map((r: any) => ({
        ...r,
        imageUrl: r.imageUrl?.startsWith('/') ? apiService.getAbsoluteUrl(r.imageUrl) : r.imageUrl
    }));
};
const selectTarget = (node: any) => { targetNode.value = node; searchQ2.value = node.title; searchResults2.value = []; };

onMounted(() => { initGraph(); loadTempItems(); loadGraph(); });
</script>

<template>
  <div class="impression-container">
    <div ref="canvas" class="graph-canvas"></div>

    <div class="ui-layer">
        <!-- Gallery Dock -->
        <div class="gallery-dock glass" :class="{ collapsed: !showTempGallery }">
            <div class="dock-header">
                <div class="title-meta">
                    <span class="p-icon">🌌</span>
                    <h2>Discovery Queue</h2>
                    <span class="count-badge">{{ tempItems.length }}</span>
                </div>
            </div>
            <div class="dock-content">
                <div v-for="item in tempItems" :key="item.id" class="dock-card" @click="openAddModal(item)">
                    <img :src="item.imageUrl" />
                    <div class="card-overlay"><span class="add-icon">+</span></div>
                </div>
                <div v-if="!tempItems.length" class="empty-state">No discoveries found. Sync from bots.</div>
            </div>
            <div class="dock-pull-tab" :class="{ active: showTempGallery }" @click="showTempGallery = !showTempGallery">
                <span class="pull-text">{{ showTempGallery ? '收起隊列 ▲' : '✨ 展開隊列 (' + tempItems.length + ') ▼' }}</span>
            </div>
        </div>

        <!-- Studio Toolbox -->
        <div class="studio-toolbox glass neon-border">
            <div class="tool-btn" :class="{ on: isPhysicsEnabled }" @click="isPhysicsEnabled = !isPhysicsEnabled">
                <div class="t-icon">⚛️</div>
                <div class="t-label">Flow</div>
            </div>
            <div class="tool-btn" @click="goToRandomNode">
                <div class="t-icon">🎲</div>
                <div class="t-label">Random</div>
            </div>
            <div class="t-sep"></div>
            <div class="tool-btn" @click="showExportPanel = !showExportPanel" :class="{ on: showExportPanel }">
                <div class="t-icon">📸</div>
                <div class="t-label">Photo</div>
            </div>
            <div class="t-sep"></div>
            <div class="tool-btn" @click="exportGraphData"><div class="t-icon">📥</div><div class="t-label">Export</div></div>
            <div class="tool-btn" @click="importFileRef?.click()"><div class="t-icon">📤</div><div class="t-label">Load</div></div>
        </div>

        <!-- Node Explorer Bottom Card -->
        <div v-if="selectedNodeDetails" class="node-explorer-card glass neon-border" :class="{ 'link-expanded': isLinkingMode || isEditingNode || isEditingNoteContent }">
            <button class="card-close" @click="selectedNodeDetails = null">×</button>
            
            <div class="card-flex">
                <!-- Identity Column -->
                <div class="card-identity">
                    <div class="image-area">
                        <img v-if="selectedNodeDetails.imageUrl" :src="selectedNodeDetails.imageUrl" />
                        <div class="tag">{{ selectedNodeDetails.nodeType }}</div>
                    </div>
                    <div class="info-area">
                        <h2 class="node-title">{{ selectedNodeDetails.title }}</h2>
                        <div class="scroll-area">
                            <p>{{ selectedNodeDetails.content || 'Standing by for synthesis.' }}</p>
                            <div class="timestamp">Memory Created: {{ new Date(selectedNodeDetails.createdAt).toLocaleString() }}</div>
                        </div>
                        <div class="btn-group">
                            <button class="g-btn focus-b" @click="setAsCenter(selectedNodeDetails.id)">Focus</button>
                            <button class="g-btn edit-b" @click="isEditingNode = !isEditingNode" :class="{ active: isEditingNode }">✏️ Edit</button>
                            <button class="g-btn sync-b" 
                                @click="() => selectedNodeDetails.linkedSnippetId ? openNoteEditor(selectedNodeDetails.linkedSnippetId) : syncToSnippet(selectedNodeDetails.id)" 
                                :class="{ linked: selectedNodeDetails.linkedSnippetId, active: isEditingNoteContent }">
                                {{ selectedNodeDetails.linkedSnippetId ? '📝 Edit Note' : '➕ Note' }}
                            </button>
                            <button class="g-btn link-b" @click="isLinkingMode = !isLinkingMode" :class="{ active: isLinkingMode }">🔗 Link</button>
                            <button class="g-btn del-b" @click="deleteNode(selectedNodeDetails.id)">🗑️</button>
                        </div>
                    </div>
                </div>

                <!-- Link Column -->
                <div v-if="isLinkingMode" class="card-link-engine">
                    <h3>Weave Connection</h3>
                    <div class="in-field"><label>Label</label><input v-model="linkLabel" placeholder="Name the bond..." /></div>
                    <div class="in-field">
                        <label>Target</label>
                        <input v-model="searchQ2" placeholder="Search knowledge..." @input="performSearch2" />
                        <div v-if="searchResults2.length" class="inline-results glass">
                            <div v-for="r in searchResults2" :key="r.id" class="res-item" @click="selectTarget(r)">{{ r.title }}</div>
                        </div>
                    </div>
                    <div v-if="targetNode" class="target-indicator">Connecting to: <span>{{ targetNode.title }}</span></div>
                    <button class="confirm-link-btn" @click="createLink" :disabled="!targetNode">Confirm Weaving</button>
                </div>

                <!-- Edit Column -->
                <div v-if="isEditingNode" class="card-link-engine edit-mode">
                    <h3>Refine Memory</h3>
                    <div class="in-field"><label>Title</label><input v-model="editForm.title" /></div>
                    <div class="in-field">
                        <label>Category</label>
                        <select v-model="editForm.nodeType" class="premium-select">
                            <option value="general">Concept</option>
                            <option value="person">Bio</option>
                            <option value="place">Location</option>
                        </select>
                    </div>
                    <div class="in-field"><label>Manifest Content</label><textarea v-model="editForm.content" class="premium-textarea"></textarea></div>
                    <button class="confirm-link-btn" @click="saveNodeEdits">Save Refinement</button>
                </div>

                <!-- Note Editing Column -->
                <div v-if="isEditingNoteContent" class="card-link-engine note-edit-mode">
                    <h3>Edit Linked Note</h3>
                    <div class="in-field">
                        <label>Note Name</label>
                        <input v-model="linkedNoteData.name" />
                    </div>
                    <div class="in-field">
                        <label>Knowledge Body</label>
                        <textarea v-model="linkedNoteData.content" class="premium-textarea note-area"></textarea>
                    </div>
                    <button class="confirm-link-btn sync-save" @click="saveNoteChanges">Update Personal Board</button>
                </div>
            </div>
        </div>

        <!-- Export Panel -->
        <div v-if="showExportPanel" class="snapshot-panel glass neon-border">
            <div class="panel-head"><h4>Snapshot Studio</h4><button class="close-p" @click="showExportPanel = false">×</button></div>
            <div class="panel-body">
                <p class="p-hint">Select Backdrop Style</p>
                <div class="color-picker-grid">
                    <div v-for="c in ['#0f172a', '#1e293b', '#000000', '#0a0a0a']" :key="c" :style="{ background: c }"
                        @click="exportBgColor = c; exportBgImage = null" :class="{ active: !exportBgImage && exportBgColor === c }" class="c-dot"></div>
                </div>
                <div class="bg-picker-grid">
                    <div v-for="bg in defaultBackgrounds" :key="bg.name" class="bg-item" 
                         :class="{ active: exportBgImage === bg.url }" @click="exportBgImage = bg.url">
                        <img :src="bg.url" />
                        <span>{{ bg.name }}</span>
                    </div>
                </div>
                <button class="action-submit-btn" @click="exportAsImage">Download Art</button>
            </div>
        </div>

        <div class="corner-search glass">
            <span class="s-icon">🔍</span>
            <input v-model="globalSearchQuery" placeholder="Search universe..." @input="performGlobalSearch" />
            <div v-if="globalSearchResults.length" class="search-drop glass">
                <div v-for="r in globalSearchResults" :key="r.id" class="drop-item" @click="loadGraph(r.id)">
                    <img v-if="r.imageUrl" :src="r.imageUrl" /><span>{{ r.title }}</span>
                </div>
            </div>
        </div>
    </div>

    <!-- Modals -->
    <div v-if="showAddNodeModal" class="modal-layer" @click.self="showAddNodeModal = false">
        <div class="modal-card glass neon-border">
            <header><h3>Integrate Impression</h3></header>
            <div class="modal-split">
                <div class="m-preview"><img :src="selectedTempItem.imageUrl" /></div>
                <div class="m-form">
                    <div class="m-f"><label>Title</label><input v-model="newNodeData.title" /></div>
                    <div class="m-f">
                        <label>Identity Type</label>
                        <select v-model="newNodeData.nodeType">
                            <option value="general">Concept</option>
                            <option value="person">Bio</option>
                            <option value="place">Location</option>
                        </select>
                    </div>
                </div>
            </div>
            <button class="modal-submit" @click="saveNode">Authorize Integration</button>
        </div>
    </div>
    <input type="file" ref="importFileRef" class="hidden" accept=".json" @change="importGraphData" />
    <div v-if="isLoading" class="loading-overlay"><div class="spin"></div></div>
  </div>
</template>

<style scoped>
.impression-container {
  height: calc(100vh - 64px); width: 100vw; background: #0b0f1a; position: relative; overflow: hidden; font-family: 'Inter', system-ui, sans-serif;
}
.graph-canvas { width: 100%; height: 100%; }

.ui-layer { position: absolute; inset: 0; pointer-events: none; z-index: 100; padding: 25px; }
.ui-layer > * { pointer-events: auto; }

.glass { background: rgba(15, 23, 42, 0.88); backdrop-filter: blur(25px); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 24px; box-shadow: 0 15px 45px rgba(0,0,0,0.6); }
.neon-border { border: 1px solid rgba(34, 211, 238, 0.4); box-shadow: 0 0 25px rgba(34, 211, 238, 0.1), 0 15px 45px rgba(0,0,0,0.6); }

/* Gallery Dock */
.gallery-dock {
  position: absolute; top: -235px; left: 50%; transform: translateX(-50%); width: 90%; max-width: 960px; transition: all 0.6s cubic-bezier(0.18, 0.89, 0.32, 1.28); display: flex; flex-direction: column;
}
.snapshot-panel { position: absolute; right: 25px; top: 50%; transform: translateY(-50%); width: 340px; padding: 25px; z-index: 2000; }
.color-picker-grid { display: flex; gap: 12px; margin-bottom: 20px; flex-wrap: wrap; }
.c-dot { width: 32px; height: 32px; border-radius: 50%; border: 2px solid transparent; cursor: pointer; }
.c-dot.active { border-color: #22d3ee; transform: scale(1.1); box-shadow: 0 0 15px rgba(34, 211, 238, 0.4); }

.bg-picker-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; margin-bottom: 25px; }
.bg-item { height: 80px; border-radius: 12px; overflow: hidden; position: relative; cursor: pointer; border: 2px solid transparent; transition: 0.3s; }
.bg-item img { width: 100%; height: 100%; object-fit: cover; opacity: 0.7; }
.bg-item span { position: absolute; bottom: 5px; left: 8px; font-size: 0.65rem; font-weight: 700; color: white; text-shadow: 0 1px 4px rgba(0,0,0,0.8); }
.bg-item:hover img { opacity: 1; }
.bg-item.active { border-color: #22d3ee; transform: scale(1.05); }
.bg-item.active img { opacity: 1; }

.p-hint { font-size: 0.7rem; color: #94a3b8; margin-bottom: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 1px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.panel-head h4 { color: #22d3ee; margin: 0; font-size: 1.1rem; }
.close-p { background: none; border: none; color: #94a3b8; font-size: 24px; cursor: pointer; }
.action-submit-btn { width: 100%; padding: 14px; background: linear-gradient(135deg, #22d3ee, #0ea5e9); border: none; border-radius: 12px; color: #0f172a; font-weight: 800; cursor: pointer; transition: 0.3s; }
.action-submit-btn:hover { transform: translateY(-3px); box-shadow: 0 8px 25px rgba(34, 211, 238, 0.4); }

.gallery-dock:not(.collapsed) { top: 15px; }
.dock-header { display: flex; justify-content: space-between; align-items: center; padding: 18px 28px; border-bottom: 1px solid rgba(255,255,255,0.05); }
.title-meta { display: flex; align-items: center; gap: 14px; }
.count-badge { background: linear-gradient(135deg, #4f46e5, #4338ca); color: white; padding: 2px 10px; border-radius: 20px; font-size: 0.75rem; font-weight: 800; }

.dock-content { display: flex; gap: 18px; padding: 25px 28px; overflow-x: auto; height: 165px; }
.dock-card { min-width: 115px; height: 115px; border-radius: 16px; overflow: hidden; position: relative; cursor: pointer; border: 2px solid transparent; transition: 0.3s; }
.dock-card:hover { transform: translateY(-8px) scale(1.05); border-color: #22d3ee; }
.dock-card img { width: 100%; height: 100%; object-fit: cover; }
.card-overlay { position: absolute; inset: 0; background: rgba(34, 211, 238, 0.2); opacity: 0; display: flex; align-items: center; justify-content: center; }
.dock-card:hover .card-overlay { opacity: 1; }
.dock-pull-tab {
  position: absolute; bottom: -45px; left: 50%; transform: translateX(-50%); width: 220px; height: 45px; background: rgba(15, 23, 42, 0.95); border: 1px solid rgba(34, 211, 238, 0.4); border-top: none; border-radius: 0 0 20px 20px; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: 0.3s;
}
.pull-text { color: #22d3ee; font-weight: 800; font-size: 0.8rem; }
.dock-pull-tab:hover { background: #1e293b; bottom: -50px; height: 50px; }
.dock-pull-tab.active { background: #22d3ee; color: #0f172a; }
.dock-pull-tab.active .pull-text { color: #0f172a; }

/* Studio Toolbox */
.studio-toolbox { position: absolute; left: 25px; top: 50%; transform: translateY(-50%); width: 85px; padding: 20px 0; display: flex; flex-direction: column; align-items: center; gap: 15px; pointer-events: auto; z-index: 1000; }
.tool-btn { 
    display: flex; flex-direction: column; align-items: center; gap: 6px; cursor: pointer; transition: 0.4s; padding: 12px 0; width: 70px; border-radius: 16px; position: relative;
}
.tool-btn .t-icon { font-size: 1.5rem; opacity: 0.5; filter: grayscale(1); transition: 0.3s; }
.tool-btn .t-label { font-size: 0.6rem; font-weight: 800; text-transform: uppercase; color: #64748b; }
.tool-btn:hover { background: rgba(255,255,255,0.05); }
.tool-btn.on .t-icon { opacity: 1; filter: grayscale(0); color: #22d3ee; text-shadow: 0 0 15px rgba(34, 211, 238, 0.6); }
.tool-btn.on .t-label { color: #22d3ee; }
.t-sep { width: 30%; height: 1px; background: rgba(255,255,255,0.1); margin: 5px 0; }

/* Node Explorer */
.node-explorer-card { position: absolute; bottom: 35px; left: 130px; width: 450px; transition: width 0.5s; overflow: hidden; z-index: 500; }
.node-explorer-card.link-expanded { width: 900px; }
.card-flex { display: flex; width: 100%; height: 100%; }
.card-identity { flex: 450px; min-width: 450px; display: flex; flex-direction: column; }
.image-area { height: 220px; position: relative; }
.image-area img { width: 100%; height: 100%; object-fit: cover; }
.tag { position: absolute; top: 15px; right: 20px; background: #4f46e5; color: white; padding: 5px 12px; border-radius: 8px; font-size: 0.75rem; font-weight: 900; }

.info-area { padding: 30px; flex: 1; display: flex; flex-direction: column; }
.node-title { margin: 0 0 10px; font-size: 1.8rem; color: #f1f5f9; font-weight: 800; }
.scroll-area { max-height: 140px; overflow-y: auto; margin-bottom: 25px; }
.timestamp { margin-top: 15px; font-size: 0.7rem; color: #475569; font-weight: 700; text-transform: uppercase; }

.btn-group { display: flex; gap: 10px; flex-wrap: wrap; margin-top: auto; }
.g-btn { flex: 1; height: 42px; min-width: 70px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.1); background: rgba(255,255,255,0.03); color: white; font-weight: 700; cursor: pointer; font-size: 0.8rem; transition: 0.3s; }
.g-btn:hover { background: rgba(255,255,255,0.08); }
.g-btn.active { background: #22d3ee; color: #0f172a; border-color: #22d3ee; }
.focus-b { border-color: #22d3ee; color: #22d3ee; }
.sync-b { background: rgba(34, 211, 238, 0.05); color: #22d3ee; border-color: rgba(34, 211, 238, 0.2); }
.sync-b.linked { border-color: #22d3ee; }
.del-b { flex: 0 0 50px; color: #f43f5e; border-color: rgba(244, 63, 94, 0.2); }

.card-link-engine { flex: 450px; padding: 35px; background: rgba(0,0,0,0.3); border-left: 1px solid rgba(255,255,255,0.08); display: flex; flex-direction: column; animation: slideIn 0.4s ease-out; }
@keyframes slideIn { from { opacity: 0; transform: translateX(20px); } to { opacity: 1; transform: translateX(0); } }

.card-link-engine h3 { margin: 0 0 25px; color: #22d3ee; font-size: 1.1rem; }
.premium-select, .premium-textarea { width: 100%; background: #0f172a; border: 1px solid rgba(255,255,255,0.1); color: white; padding: 12px; border-radius: 12px; font-family: inherit; }
.premium-textarea { height: 100px; resize: none; margin-bottom: 20px; }
.note-area { height: 260px; }

.action-submit-btn, .confirm-link-btn, .modal-submit { width: 100%; height: 48px; background: #22d3ee; color: #080c14; border: none; border-radius: 12px; font-weight: 800; cursor: pointer; }
.sync-save { background: linear-gradient(135deg, #22d3ee, #0ea5e9); margin-top: auto; }

/* Utils */
.corner-search { position: absolute; top: 25px; right: 25px; width: 320px; padding: 10px 20px; display: flex; align-items: center; gap: 12px; }
.corner-search input { background: none; border: none; outline: none; color: white; flex: 1; }
.search-drop { position: absolute; top: 60px; left: 0; width: 100%; max-height: 300px; overflow-y: auto; }
.drop-item { padding: 12px 20px; display: flex; align-items: center; gap: 12px; cursor: pointer; }
.drop-item img { width: 32px; height: 32px; border-radius: 6px; object-fit: cover; }

.modal-layer { position: fixed; inset: 0; background: rgba(2, 6, 23, 0.85); display: flex; align-items: center; justify-content: center; z-index: 3000; }
.modal-card { width: 550px; padding: 35px; }
.modal-split { display: flex; gap: 20px; margin: 25px 0; }
.m-preview { width: 160px; height: 160px; border-radius: 12px; overflow: hidden; }
.m-preview img { width: 100%; height: 100%; object-fit: cover; }
.m-form { flex: 1; display: flex; flex-direction: column; gap: 15px; }

.hidden { display: none; }
.loading-overlay { position: fixed; inset: 0; background: rgba(11, 15, 26, 0.8); display: flex; align-items: center; justify-content: center; z-index: 5000; }
.spin { width: 40px; height: 40px; border: 3px solid rgba(34, 211, 238, 0.1); border-top-color: #22d3ee; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
