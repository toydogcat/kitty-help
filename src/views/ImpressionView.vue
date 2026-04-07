<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue';
import { Network } from 'vis-network';
import { DataSet } from 'vis-data';
import { apiService } from '../services/api';
import { useAuth } from '../composables/useAuth';

const { } = useAuth();
const canvas = ref<HTMLElement | null>(null);
const network = ref<Network | null>(null);
const importFileRef = ref<HTMLInputElement | null>(null);

// Data
const centerNodeId = ref<string | null>(localStorage.getItem('impression_last_center') || null);

// UI State
const isLoading = ref(false);

const searchQ2 = ref('');
const searchResults2 = ref<any[]>([]);
const targetNode = ref<any>(null);
const linkLabel = ref('');

const selectedNodeDetails = ref<any>(null);
const isEditingNode = ref(false);
const editForm = ref({ title: '', content: '', nodeType: 'general', mediaId: null as string | null });
const mediaSearchQuery = ref('');
const mediaSearchResults = ref<any[]>([]);
const isMediaSearching = ref(false);
const showMediaStore = ref(false);

const isLinkingMode = ref(false);

// Multi-KG State
const kgName = ref(localStorage.getItem('impression_kg_name') || 'default');
const commandInput = ref('');
const commandResults = ref<any[]>([]);
const availableKGsList = ref<string[]>([]);
const showCommandHelp = ref(false);

// Desk Linkage
const isEditingDeskLink = ref(false);
const shelves = ref<any[]>([]);
const selectedShelfItems = ref<any[]>([]);

// Physics and Export
const isPhysicsEnabled = ref(true);
const exportBgColor = ref('#0f172a'); 
const exportBgImage = ref<string | null>(null);
const showExportPanel = ref(false);

const nodes = new DataSet<any>([]);
const edges = new DataSet<any>([]);

const options = {
  nodes: {
    shape: 'dot', size: 30,
    font: { size: 14, color: '#ffffff', strokeWidth: 0, face: 'Inter, system-ui' },
    borderWidth: 3,
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
  
  const savedState = localStorage.getItem('impression_view_state');
  if (savedState) {
      const { x, y, scale } = JSON.parse(savedState);
      network.value.moveTo({ position: { x, y }, scale, animation: false });
  }

  network.value.on('click', (params) => {
    isLinkingMode.value = false;
    isEditingNode.value = false;
    if (params.nodes.length > 0) {
      handleNodeClick(params.nodes[0]);
    } else {
      selectedNodeDetails.value = null;
    }
  });

  network.value.on('dragEnd', saveViewState);
  network.value.on('zoom', saveViewState);
  loadShelves();
};

const loadShelves = async () => {
    try {
        shelves.value = await apiService.getShelves();
    } catch (e) { console.error(e); }
};

const loadShelfPreview = async (shelfId: string) => {
    try {
        selectedShelfItems.value = await apiService.getDeskItems(shelfId);
    } catch (e) { console.error(e); }
};

const linkShelf = async (shelfId: string | null) => {
    if (!selectedNodeDetails.value) return;
    try {
        const updated = { ...selectedNodeDetails.value, deskShelfId: shelfId };
        await apiService.updateImpressionNode(selectedNodeDetails.value.id, updated);
        selectedNodeDetails.value.deskShelfId = shelfId;
        const existingNode = nodes.get(selectedNodeDetails.value.id) as any;
        if (existingNode && existingNode.raw) {
            existingNode.raw.deskShelfId = shelfId;
            nodes.update(existingNode);
        }
        if (shelfId) loadShelfPreview(shelfId);
        else selectedShelfItems.value = [];
    } catch (e) { console.error(e); }
};

import { useRouter } from 'vue-router';
const router = useRouter();
const jumpToDesk = (shelfId: string) => {
    router.push({ name: 'desk', query: { shelfId } });
};

watch(isPhysicsEnabled, (newVal) => {
    if (network.value) network.value.setOptions({ physics: { enabled: newVal } });
});

const handleNodeClick = (nodeId: string) => {
  const node = nodes.get(nodeId);
  if (node) {
      selectedNodeDetails.value = node.raw;
      editForm.value = { ...node.raw };
      isEditingDeskLink.value = false;
      isEditingNode.value = false;
      showMediaStore.value = false;
      mediaSearchResults.value = [];
      showMediaStore.value = false;
      mediaSearchResults.value = [];
      if (node.raw.deskShelfId) {
          loadShelfPreview(node.raw.deskShelfId);
      } else {
          selectedShelfItems.value = [];
      }
  }
};

const getBase64Image = async (url: string) => {
    try {
        const response = await fetch(url);
        const blob = await response.blob();
        return new Promise((resolve) => {
            const reader = new FileReader();
            reader.onloadend = () => resolve(reader.result);
            reader.readAsDataURL(blob);
        });
    } catch (e) {
        return url;
    }
}

const loadGraph = async (nodeId?: string) => {
  isLoading.value = true;
  try {
    const data = await apiService.getImpressionGraph(nodeId || centerNodeId.value || '', kgName.value);
    
    const visNodes = await Promise.all(data.nodes.map(async (n: any) => {
        let finalUrl = n.imageUrl ? apiService.getAbsoluteUrl(n.imageUrl) : '';
        if (!finalUrl && n.fileId) finalUrl = apiService.getStorehouseFileUrl(n.fileId, n.sourcePlatform);
        
        const thumbUrl = finalUrl ? `${finalUrl}${finalUrl.includes('?') ? '&' : '?'}w=256` : '';

        if (finalUrl && !finalUrl.includes('?t=')) {
           const sep = finalUrl.includes('?') ? '&' : '?';
           finalUrl = `${finalUrl}${sep}t=${Date.now()}`;
        }
        
        let safeImage: any = undefined;
        if (thumbUrl) {
            safeImage = await getBase64Image(thumbUrl);
        }

        n.imageUrl = finalUrl; 
        return {
            id: n.id, label: n.title, shape: safeImage ? 'circularImage' : 'dot',
            image: safeImage,
            color: { border: n.id === (nodeId || centerNodeId.value) ? '#22d3ee' : '#4338ca', background: '#1e293b' },
            raw: n
        };
    }));

    nodes.clear();
    nodes.add(visNodes);
    edges.clear();
    edges.add(data.edges.map((e: any) => ({ 
        id: e.id, from: e.sourceId, to: e.targetId, label: e.label,
        font: { color: kgName.value === 'default' ? '#94a3b8' : '#22d3ee' }
    })));
    
    if (nodeId && network.value && nodes.length > 0) {
        setTimeout(() => {
            if (network.value && nodes.get(nodeId)) {
                network.value.fit({ nodes: [nodeId], animation: true });
                // NEW: Sync selectedNodeDetails with the newly loaded data
                const fresh = nodes.get(nodeId) as any;
                if (fresh) selectedNodeDetails.value = fresh.raw;
            } else if (network.value && nodes.length > 0) {
                network.value.fit({ animation: true });
            }
            centerNodeId.value = nodeId;
            localStorage.setItem('impression_last_center', nodeId);
        }, 300);
    } else if (localStorage.getItem('impression_view_state') && network.value) {
        const { x, y, scale } = JSON.parse(localStorage.getItem('impression_view_state')!);
        network.value.moveTo({ position: { x, y }, scale, animation: false });
    } else if (network.value && nodes.length > 0) {
        setTimeout(() => network.value?.fit({ animation: true }), 300);
    }
  } catch (e) { console.error(e); } finally { isLoading.value = false; }
};

const fetchKGs = async () => {
    try {
        availableKGsList.value = await apiService.getKnowledgeGraphs();
    } catch (e) { console.error(e); }
};

const switchKG = (name: string) => {
    kgName.value = name;
    localStorage.setItem('impression_kg_name', name);
    loadGraph();
};

const resolveNodeByTitle = async (title: string) => {
    const res = await apiService.searchImpression(title, kgName.value);
    const exact = res.find((r: any) => r.resultType === 'node' && r.title.toLowerCase() === title.toLowerCase());
    return exact || null;
};

const pinToDesk = async () => {
    try {
        const currentShelves = await apiService.getShelves();
        let targetShelf = currentShelves.find((s: any) => s.name === 'Knowledge Universe');
        if (!targetShelf) {
            targetShelf = await apiService.createShelf({ name: 'Knowledge Universe' });
        }
        
        // 1. Create a bookmark first
        const bookmark = await apiService.addBookmark({
            title: `Universe: ${kgName.value}`,
            url: `/impression?kg=${kgName.value}`,
            category: 'Impression'
        });

        // 2. Add that bookmark to the shelf
        await apiService.addDeskItem({
            shelfId: targetShelf.id,
            refId: bookmark.id,
            type: 'bookmark'
        });

        alert(`Successfully pinned '${kgName.value}' to Desk!`);
    } catch (e) { alert('Pinning failed'); }
};

const executeCommand = async () => {
    const input = commandInput.value.trim();
    if (!input) return;

    const parts = input.split(' ');
    const cmd = parts[0].toLowerCase();
    const args = parts.slice(1);

    commandResults.value = [];
    
    try {
        if (cmd === '/add' && args[0] === 'point') {
            const title = args.slice(1).join(' ');
            if (!title) throw new Error('Title required. Usage: /add point [title]');
            const newNode = await apiService.createImpressionNode({ title, content: '', nodeType: 'general', kgName: kgName.value });
            await loadGraph(newNode.id);
            commandInput.value = '';
            fetchKGs();
        } else if (cmd === '/add' && args[0] === 'edge') {
            const argString = args.slice(1).join(' ');
            const matches = argString.match(/(?:[^\s"]+|"[^"]*")+/g) || [];
            const cleanArgs = matches.map(a => a.replace(/"/g, ''));

            if (cleanArgs.length < 2) throw new Error('Source and Target titles required. Usage: /add edge "Source" "Target" [Label]');
            const src = await resolveNodeByTitle(cleanArgs[0]);
            const tgt = await resolveNodeByTitle(cleanArgs[1]);
            const label = cleanArgs[2] || '';

            if (!src || !tgt) throw new Error(`Node not found: ${!src ? cleanArgs[0] : cleanArgs[1]}`);
            await apiService.createImpressionLink({ sourceId: src.id, targetId: tgt.id, label, kgName: kgName.value });
            await loadGraph(src.id);
            commandInput.value = '';
        } else if (cmd === '/search') {
            const q = args.join(' ');
            if (!q) throw new Error('Query required. Usage: /search [term]');
            commandResults.value = await apiService.searchImpression(q, kgName.value);
        } else if (cmd === '/list') {
            const typeFilter = args[0]?.toLowerCase();
            if (typeFilter === 'kg' || typeFilter === 'kgs') {
                await fetchKGs();
                let combined = [...availableKGsList.value];
                if (!combined.includes('default')) combined.push('default');
                if (!combined.includes(kgName.value)) combined.push(kgName.value);
                
                const q = args.slice(1).join(' ').toLowerCase();
                let filtered = combined;
                if (q) {
                    filtered = combined.filter(n => n.toLowerCase().includes(q));
                }
                commandResults.value = filtered.map(name => ({
                    id: name, title: name, resultType: 'kg', kgName: name
                }));
                return;
            }
            const all = await apiService.searchImpression('', kgName.value);
            if (typeFilter === 'point' || typeFilter === 'node') {
                commandResults.value = all.filter((r: any) => r.resultType === 'node');
            } else if (typeFilter === 'edge' || typeFilter === 'link') {
                commandResults.value = all.filter((r: any) => r.resultType === 'edge');
            } else {
                commandResults.value = all;
            }
        } else if (cmd === '/kg') {
            if (!args[0]) throw new Error('KG name required. Usage: /kg [name]');
            switchKG(args[0]);
            commandInput.value = '';
        } else if (cmd === '/pin') {
            await pinToDesk();
            commandInput.value = '';
        } else if (cmd === '/help') {
            showCommandHelp.value = true;
        } else {
            commandResults.value = await apiService.searchImpression(input, kgName.value);
        }
    } catch (e: any) {
        alert(`${e.message}`);
    }
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
    const savedPos = network.value.getViewPosition();
    const savedScale = network.value.getScale();

    network.value.fit({ animation: false });
    const currentScale = network.value.getScale();
    network.value.moveTo({ scale: currentScale * 1.2, animation: false });
    
    setTimeout(async () => {
        const originalCanvas = canvas.value!.getElementsByTagName('canvas')[0];
        if (!originalCanvas) return;
        
        const exportCanvas = document.createElement('canvas');
        exportCanvas.width = originalCanvas.width;
        exportCanvas.height = originalCanvas.height;
        const ctx = exportCanvas.getContext('2d');
        if (!ctx) return;

        if (exportBgImage.value) {
            try {
                const bgImg = new Image();
                bgImg.crossOrigin = "anonymous";
                bgImg.src = exportBgImage.value;
                await new Promise((resolve, reject) => { 
                    bgImg.onload = resolve; 
                    bgImg.onerror = reject;
                    setTimeout(resolve, 3000); 
                });
                const scale = Math.max(exportCanvas.width / bgImg.width, exportCanvas.height / bgImg.height);
                const x = (exportCanvas.width / 2) - (bgImg.width / 2) * scale;
                const y = (exportCanvas.height / 2) - (bgImg.height / 2) * scale;
                ctx.drawImage(bgImg, x, y, bgImg.width * scale, bgImg.height * scale);
            } catch (err) {
                ctx.fillStyle = exportBgColor.value;
                ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
            }
        } else {
            ctx.fillStyle = exportBgColor.value;
            ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);
        }

        ctx.drawImage(originalCanvas, 0, 0);
        const link = document.createElement('a');
        link.download = `impression_art_${Date.now()}.png`;
        link.href = exportCanvas.toDataURL('image/png');
        link.click();
        
        showExportPanel.value = false;
        network.value?.moveTo({ 
            position: savedPos, scale: savedScale, 
            animation: { duration: 500, easingFunction: 'easeInOutQuad' } 
        });
    }, 200);
};

const setAsCenter = (id: string) => { loadGraph(id); selectedNodeDetails.value = null; };

const createLink = async () => {
    if (!selectedNodeDetails.value || !targetNode.value) return;
    try {
        await apiService.createImpressionLink({ sourceId: selectedNodeDetails.value.id, targetId: targetNode.value.id, label: linkLabel.value, kgName: kgName.value });
        targetNode.value = null; isLinkingMode.value = false; linkLabel.value = ''; await loadGraph(selectedNodeDetails.value.id);
    } catch (e) { console.error(e); }
};

const performSearch2 = async () => {
    if (searchQ2.value.length < 1) { searchResults2.value = []; return; }
    const results = await apiService.searchImpression(searchQ2.value, kgName.value);
    searchResults2.value = results.filter((r:any) => r.resultType === 'node').map((r: any) => ({
        ...r, imageUrl: r.imageUrl?.startsWith('/') ? apiService.getAbsoluteUrl(r.imageUrl) : r.imageUrl
    }));
};

let searchTimer2: any = null;
const debouncedSearch2 = () => {
    clearTimeout(searchTimer2);
    searchTimer2 = setTimeout(performSearch2, 300);
};

const searchMediaStore = async () => {
    isMediaSearching.value = true;
    try {
        // Switch to getStorehouseItems for direct media archive access (not just logs)
        const res = await apiService.getStorehouseItems({ q: mediaSearchQuery.value, limit: 30 });
        // res is a direct array from GetStorehouseItems
        mediaSearchResults.value = res.map((m: any) => ({
            ...m,
            // Backend uses snake_case: file_id, source
            fileId: m.file_id || m.fileID,
            sourcePlatform: m.source || m.source_platform,
            thumbUrl: apiService.getStorehouseFileUrl(m.file_id || m.fileID, m.source || m.source_platform) + '&w=200'
        }));
    } catch (e) { console.error(e); } finally { isMediaSearching.value = false; }
};

const selectMediaStoreItem = (item: any) => {
    editForm.value.mediaId = item.id;
    // Immediate preview update
    if (selectedNodeDetails.value) {
        selectedNodeDetails.value.imageUrl = apiService.getAbsoluteUrl(apiService.getStorehouseFileUrl(item.fileId, item.sourcePlatform));
    }
    showMediaStore.value = false;
};

const allNodesInKG = computed(() => {
    return nodes.get().filter((n: any) => n.id !== selectedNodeDetails.value?.id).map((n: any) => ({
        id: n.id,
        title: n.label,
        raw: n.raw
    }));
});

const onNodeSelectChange = (e: Event) => {
    const val = (e.target as HTMLSelectElement).value;
    const node = nodes.get(val) as any;
    if (node) selectTarget(node.raw);
};

const selectTarget = (node: any) => { targetNode.value = node; searchQ2.value = node.title; searchResults2.value = []; };

onMounted(() => { initGraph(); fetchKGs(); loadGraph(); });
</script>

<template>
  <div class="impression-container">
    <div ref="canvas" class="graph-canvas"></div>

    <div class="ui-layer">
        <!-- Command Console -->
        <div class="command-console glass" :class="{ 'has-results': commandResults.length }">
            <div class="console-input-area">
                <span class="prompt">λ</span>
                <input 
                    type="text" 
                    v-model="commandInput" 
                    placeholder="Enter command... (/add, /search, /list, /kg, /help)" 
                    @keyup.enter="executeCommand"
                    @input="commandResults = []"
                    spellcheck="false"
                />
                <button class="exec-btn" @click="executeCommand">RUN</button>
                <div class="kg-indicator" @click="showCommandHelp = !showCommandHelp">
                    <span class="kg-label">KG:</span>
                    <span class="kg-val">{{ kgName }}</span>
                </div>
            </div>

            <!-- Command Results Terminal -->
            <div v-if="commandResults.length" class="console-results">
                <div v-for="r in commandResults" :key="r.id" class="res-row" @click="r.resultType === 'kg' ? switchKG(r.title) : loadGraph(r.id); commandResults = []">
                    <span class="res-type" :class="r.resultType">{{ r.resultType }}</span>
                    <span class="res-title">{{ r.title }}</span>
                    <span v-if="r.resultType === 'edge'" class="res-link">({{ r.sourceTitle }} → {{ r.targetTitle }})</span>
                    <span class="res-kg">#{{ r.kgName }}</span>
                </div>
            </div>

            <!-- Command Help Inline -->
            <div v-if="showCommandHelp" class="console-help glass">
                <button class="close-help" @click="showCommandHelp = false">×</button>
                <h3>Knowledge Terminal 知識圖譜終端機控制台</h3>
                <ul>
                    <li><code>/add point [標題]</code> - 建立一個新的知識點 (Concept Node)</li>
                    <li><code>/add edge "[起點]" "[終點]" [標籤]</code> - 透過標題連結兩個點 (含空格標題請用雙引號)</li>
                    <li><code>/search [關鍵字]</code> - 同時搜尋當前圖譜中的節點與連線</li>
                    <li><code>/list [point/edge/kg]</code> - 分類列出所有節點、邊或是現有的知識宇宙 (KG)</li>
                    <li><code>/kg [名稱]</code> - 切換至指定的知識宇宙，若名稱不存在則會建立新圖譜</li>
                    <li><code>/pin</code> - 將目前的圖譜固定至 Desk 工作台，方便從其他畫面快速進入</li>
                    <li><code>/help</code> - 切換顯示此專業說明手冊</li>
                </ul>
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
            <div class="tool-btn" @click="pinToDesk">
                <div class="t-icon">📌</div>
                <div class="t-label">Pin Desk</div>
            </div>
            <div class="t-sep"></div>
            <div class="tool-btn" @click="exportGraphData"><div class="t-icon">📥</div><div class="t-label">Export</div></div>
            <div class="tool-btn" @click="importFileRef?.click()"><div class="t-icon">📤</div><div class="t-label">Load</div></div>
        </div>

        <!-- Node Explorer Bottom Card -->
        <div v-if="selectedNodeDetails" class="node-explorer-card glass neon-border">
            <button class="card-close" @click="selectedNodeDetails = null">×</button>
            <div class="card-flex">
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
                            <button class="g-btn link-b" @click="isLinkingMode = !isLinkingMode" :class="{ active: isLinkingMode }">🔗 Link</button>
                            <button class="g-btn desk-b" @click="isEditingDeskLink = !isEditingDeskLink" :class="{ active: isEditingDeskLink }">
                                {{ selectedNodeDetails.deskShelfId ? '💾 Linked' : '💾 Link Desk' }}
                            </button>
                            <button class="g-btn del-b" @click="deleteNode(selectedNodeDetails.id)">🗑️</button>
                        </div>
                    </div>
                </div>

                <div v-if="isLinkingMode" class="card-link-engine">
                    <h3>Weave Connection</h3>
                    <div class="in-field"><label>Label</label><input v-model="linkLabel" placeholder="Name the bond..." /></div>
                    <div class="in-field">
                        <label>Target Node</label>
                        <div class="select-search-combo">
                            <select :value="targetNode?.id || ''" @change="onNodeSelectChange" class="premium-select target-sel">
                                <option value="" disabled>-- Quick Jump to Node --</option>
                                <option v-for="n in allNodesInKG" :key="n.id" :value="n.id">{{ n.title }}</option>
                            </select>
                            <input type="text" v-model="searchQ2" placeholder="...or Search title" @input="debouncedSearch2" class="m-input" />
                        </div>
                        <div v-if="searchResults2.length" class="inline-results glass">
                            <div v-for="r in searchResults2" :key="r.id" class="res-item" @click="selectTarget(r)">{{ r.title }}</div>
                        </div>
                    </div>
                    <button class="confirm-link-btn" @click="createLink" :disabled="!targetNode">Confirm Weaving</button>
                </div>

                <div v-if="isEditingNode" class="card-link-engine">
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
                    <div class="in-field">
                        <label>Content</label>
                        <textarea v-model="editForm.content" class="premium-textarea" placeholder="Describe this memory..."></textarea>
                    </div>

                    <div class="in-field">
                        <label>Image Resource</label>
                        <div class="media-picker-trigger glass" @click="showMediaStore = !showMediaStore; if(showMediaStore) searchMediaStore()">
                            <span v-if="editForm.mediaId" class="m-status">✅ Resource Linked</span>
                            <span v-else class="m-status">🔍 Search Storehouse...</span>
                        </div>
                        
                        <div v-if="showMediaStore" class="media-store-explorer glass">
                            <div class="ms-head">
                                <input v-model="mediaSearchQuery" @input="searchMediaStore" placeholder="Filter images..." />
                                <button @click="showMediaStore = false">×</button>
                            </div>
                            <div class="ms-grid" v-if="!isMediaSearching">
                                <div v-for="m in mediaSearchResults" :key="m.id" class="ms-item" @click="selectMediaStoreItem(m)">
                                    <img :src="m.thumbUrl" />
                                </div>
                            </div>
                            <div v-else class="ms-loader">Scanning...</div>
                        </div>
                    </div>

                    <button class="confirm-link-btn" @click="saveNodeEdits">Save Refinement</button>
                </div>

                <div v-if="isEditingDeskLink" class="card-link-engine">
                    <h3>Workspace Link</h3>
                    <div class="in-field">
                        <label>Associate Shelf</label>
                        <select v-model="selectedNodeDetails.deskShelfId" @change="linkShelf(selectedNodeDetails.deskShelfId)" class="premium-select">
                            <option :value="null">-- No Shelf Linked --</option>
                            <option v-for="s in shelves" :key="s.id" :value="s.id">{{ s.name }}</option>
                        </select>
                    </div>
                    <div v-if="selectedNodeDetails.deskShelfId" class="shelf-snapshot">
                        <div v-for="it in selectedShelfItems" :key="it.id" class="mini-shelf-item">
                            <span class="m-icon">{{ it.type === 'bookmark' ? '🔗' : it.type === 'snippet' ? '📝' : '🖼️' }}</span>
                            <span class="m-title">{{ it.title }}</span>
                        </div>
                        <button class="confirm-link-btn" style="background: #8b5cf6; color: white" @click="jumpToDesk(selectedNodeDetails.deskShelfId)">🚀 Teleport</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Snapshot Panel -->
        <div v-if="showExportPanel" class="snapshot-panel glass neon-border">
            <div class="panel-head"><h4>Snapshot Studio</h4><button class="close-p" @click="showExportPanel = false">×</button></div>
            <div class="panel-body">
                <div class="color-picker-grid">
                    <div v-for="c in ['#0f172a', '#1e293b', '#000000', '#ffffff']" :key="c" :style="{ background: c }"
                        @click="exportBgColor = c; exportBgImage = null" class="c-dot"></div>
                </div>
                <button class="confirm-link-btn" @click="exportAsImage">Download Art</button>
            </div>
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

/* Command Console Styles */
.command-console {
    position: absolute; top: 25px; left: 50%; transform: translateX(-50%); width: 90%; max-width: 800px; display: flex; flex-direction: column; z-index: 2000; border: 1px solid rgba(34, 211, 238, 0.3);
}
.console-input-area { display: flex; align-items: center; padding: 12px 20px; gap: 15px; }
.prompt { font-family: monospace; color: #22d3ee; font-weight: 800; font-size: 1.2rem; }
.console-input-area input { flex: 1; background: none; border: none; outline: none; color: #f1f5f9; font-family: monospace; font-size: 1.1rem; }
.exec-btn { background: rgba(34, 211, 238, 0.1); border: 1px solid rgba(34, 211, 238, 0.3); color: #22d3ee; padding: 6px 15px; border-radius: 8px; font-size: 0.7rem; font-weight: 900; cursor: pointer; }
.kg-indicator { display: flex; align-items: center; gap: 8px; background: rgba(0,0,0,0.3); padding: 6px 12px; border-radius: 10px; cursor: pointer; }
.kg-label { font-size: 0.65rem; color: #64748b; font-weight: 800; }
.kg-val { font-size: 0.7rem; color: #22d3ee; font-weight: 900; }

.console-results { border-top: 1px solid rgba(255,255,255,0.05); background: rgba(0,0,0,0.4); max-height: 400px; overflow-y: auto; border-radius: 0 0 24px 24px; }
.res-row { padding: 12px 25px; display: flex; align-items: center; gap: 15px; cursor: pointer; font-family: monospace; font-size: 0.85rem; }
.res-row:hover { background: rgba(34, 211, 238, 0.1); }
.res-type { font-size: 0.65rem; padding: 2px 6px; border-radius: 4px; text-transform: uppercase; font-weight: 900; }
.res-type.node { background: rgba(34, 211, 238, 0.2); color: #22d3ee; }
.res-type.edge { background: rgba(139, 92, 246, 0.2); color: #a78bfa; }
.res-type.kg { background: rgba(34, 197, 94, 0.2); color: #4ade80; }
.res-kg { margin-left: auto; color: #475569; font-size: 0.7rem; }

.console-help { position: absolute; top: 75px; left: 0; width: 100%; padding: 30px; z-index: 100; }
.console-help h3 { color: #22d3ee; margin: 0 0 20px; font-size: 1.1rem; }
.console-help ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 12px; }
.console-help li { color: #94a3b8; font-size: 0.9rem; }
.console-help code { color: #22d3ee; font-family: monospace; background: rgba(34, 211, 238, 0.1); padding: 2px 6px; border-radius: 4px; margin-right: 10px; }
.close-help { position: absolute; top: 15px; right: 20px; background: none; border: none; color: #475569; font-size: 24px; cursor: pointer; }

/* Node Explorer Card */
.node-explorer-card { 
    position: absolute; bottom: 25px; left: 25px; right: 25px; margin: 0 auto; width: auto; max-width: 1400px; 
    max-height: calc(100vh - 120px); z-index: 1500; overflow: hidden; display: flex; flex-direction: column;
    box-shadow: 0 25px 70px rgba(0,0,0,0.8);
}
.card-flex { display: flex; height: 100%; }
.card-identity { flex: 1; min-width: 400px; display: flex; flex-direction: column; border-right: 1px solid rgba(255,255,255,0.05); }
.image-area { height: 300px; position: relative; }
.image-area img { width: 100%; height: 100%; object-fit: cover; }
.tag { position: absolute; top: 15px; right: 20px; background: #4f46e5; color: white; padding: 5px 12px; border-radius: 8px; font-size: 0.75rem; font-weight: 900; }
.info-area { padding: 40px; flex: 1; display: flex; flex-direction: column; }
.node-title { margin: 0 0 15px; font-size: 2.2rem; color: #f1f5f9; font-weight: 800; }
.scroll-area { flex: 1; font-size: 1.1rem; line-height: 1.6; color: #cbd5e1; overflow-y: auto; }
.timestamp { margin-top: 15px; font-size: 0.7rem; color: #475569; font-weight: 700; text-transform: uppercase; }
.btn-group { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 25px; }
.g-btn { flex: 1; height: 42px; min-width: 70px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.1); background: rgba(255,255,255,0.03); color: white; font-weight: 700; cursor: pointer; transition: 0.3s; }
.g-btn.active { background: #22d3ee; color: #0f172a; border-color: #22d3ee; }
.del-b { flex: 0 0 50px; color: #f43f5e; border-color: rgba(244, 63, 94, 0.2); }

.card-link-engine { flex: 1.5; min-width: 400px; padding: 40px; background: rgba(0,0,0,0.2); display: flex; flex-direction: column; overflow-y: auto; }
.card-close { position: absolute; top: 15px; right: 20px; background: none; border: none; color: #475569; font-size: 28px; cursor: pointer; z-index: 10; }

.studio-toolbox { 
    position: absolute; top: 25px; right: 25px; display: flex; padding: 4px 15px; align-items: center; z-index: 2100; 
    background: rgba(15, 23, 42, 0.5); border: 1px solid rgba(255,255,255,0.05); border-radius: 20px;
}
.tool-btn { display: flex; flex-direction: column; align-items: center; gap: 4px; cursor: pointer; padding: 8px 12px; border-radius: 12px; transition: 0.3s; opacity: 0.6; }
.tool-btn:hover { background: rgba(255,255,255,0.1); opacity: 1; transform: translateY(-2px); }
.tool-btn.on { opacity: 1; color: #22d3ee; }
.tool-btn .t-icon { font-size: 1.1rem; }
.t-label { font-size: 0.55rem; font-weight: 800; text-transform: uppercase; }
.t-sep { width: 1px; height: 18px; background: rgba(255,255,255,0.1); margin: 0 8px; }

.snapshot-panel { position: absolute; top: 85px; right: 25px; width: 250px; padding: 20px; z-index: 2200; }
.color-picker-grid { display: flex; gap: 10px; margin-bottom: 15px; }
.c-dot { width: 30px; height: 30px; border-radius: 50%; cursor: pointer; }

.in-field { margin-bottom: 25px; }
.in-field label { display: block; font-size: 0.7rem; color: #64748b; font-weight: 800; text-transform: uppercase; margin-bottom: 8px; }
.premium-select, .premium-textarea, .m-input { width: 100%; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); color: white; padding: 14px; border-radius: 12px; font-family: inherit; }
.premium-textarea { height: 180px; resize: none; }
.confirm-link-btn { width: 100%; height: 48px; background: #22d3ee; color: #080c14; border: none; border-radius: 12px; font-weight: 800; cursor: pointer; margin-top: auto; }

.loading-overlay { position: fixed; inset: 0; background: rgba(11, 15, 26, 0.8); display: flex; align-items: center; justify-content: center; z-index: 5000; }
.spin { width: 40px; height: 40px; border: 3px solid rgba(34, 211, 238, 0.1); border-top-color: #22d3ee; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.hidden { display: none; }

/* Media Picker Styles */
.media-picker-trigger { padding: 12px 20px; cursor: pointer; border-radius: 12px; border: 1px solid rgba(34, 211, 238, 0.2); transition: 0.3s; margin-top: 5px; }
.media-picker-trigger:hover { background: rgba(34, 211, 238, 0.1); }
.m-status { font-size: 0.8rem; font-weight: 700; color: #22d3ee; }

.media-store-explorer { 
    position: absolute; bottom: 85px; right: 20px; width: 380px; height: 450px; 
    z-index: 2000; display: flex; flex-direction: column; overflow: hidden; padding: 20px;
    border: 1px solid rgba(34, 211, 238, 0.3);
}
.ms-head { display: flex; gap: 10px; margin-bottom: 20px; }
.ms-head input { flex: 1; background: rgba(0,0,0,0.5); border: none; padding: 10px; border-radius: 8px; color: white; outline: none; }
.ms-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; overflow-y: auto; flex: 1; }
.ms-item { aspect-ratio: 1; border-radius: 10px; overflow: hidden; cursor: pointer; border: 2px solid transparent; transition: 0.2s; }
.ms-item:hover { transform: scale(1.05); border-color: #22d3ee; }
.ms-item img { width: 100%; height: 100%; object-fit: cover; }
.ms-loader { flex: 1; display: flex; align-items: center; justify-content: center; color: #64748b; font-size: 0.9rem; }

.select-search-combo { display: flex; flex-direction: column; gap: 8px; }
.target-sel { width: 100%; }

.shelf-snapshot { margin-top: 15px; background: rgba(0,0,0,0.25); border-radius: 16px; padding: 15px; flex: 1; overflow-y: auto; }
.mini-shelf-item { display: flex; align-items: center; gap: 10px; padding: 8px; background: rgba(255,255,255,0.03); border-radius: 8px; margin-bottom: 6px; font-size: 0.8rem; }

@media (max-width: 1000px) {
    .node-explorer-card { height: auto; max-height: 85vh; overflow-y: auto; }
    .card-flex { flex-direction: column; }
    .card-identity { border-right: none; border-bottom: 1px solid rgba(255,255,255,0.05); min-width: 0; }
}
</style>
