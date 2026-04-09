<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue';
import { apiService } from '../services/api';
import { useAuth } from '../composables/useAuth';
import { useRoute } from 'vue-router';
import { usePin } from '../composables/usePin';
import { syncService } from '../services/syncService';
import { db } from '../services/localDb';

const route = useRoute();
const { } = useAuth();
const canvas = ref<HTMLElement | null>(null);
const network = ref<any>(null);
const nodes = ref<any>(null);
const edges = ref<any>(null);
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
const showMediaStore = ref(false);
const mediaSearchQuery = ref('');
const mediaSearchResults = ref<any[]>([]);
const isMediaSearching = ref(false);

// Edge State
const selectedEdgeDetails = ref<any>(null);
const isEditingEdge = ref(false);
const edgeEditForm = ref({ label: '' });

const isLinkingMode = ref(false);
const interactionMode = ref<'view' | 'edit'>('view');
const selectedSourceNodeId = ref<string | null>(null);

// Multi-KG State - READ FROM ROUTE FIRST
const kgName = ref((route.query.kg as string) || localStorage.getItem('impression_kg_name') || 'default');
const commandInput = ref('');
const commandResults = ref<any[]>([]);
const availableKGsList = ref<string[]>([]);
const showCommandHelp = ref(false);

// Event Timers
let clickTimer: any = null;
let isWaitingForSecondClick = false;

// Advanced Interactive Buffers
const candidateNodeId = ref<string | null>(null);
const highlightedNodes = ref(new Set<string>());
const highlightedEdges = ref(new Set<string>());

watch(() => route.query.kg, (newVal) => {
    if (newVal && newVal !== kgName.value) {
        kgName.value = newVal as string;
        loadGraph();
    }
});

const { pinUniverseToDesk } = usePin();

// Desk Linkage
const isEditingDeskLink = ref(false);
const shelves = ref<any[]>([]);
const selectedShelfItems = ref<any[]>([]);

// Physics and Export
const isPhysicsEnabled = ref(true);
const exportBgColor = ref('#0f172a'); 
const exportBgImage = ref<string | null>(null);
const showExportPanel = ref(false);

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
    forceAtlas2Based: { gravitationalConstant: -200, springLength: 200, springConstant: 0.04 },
    solver: 'forceAtlas2Based',
    stabilization: { iterations: 1000, updateInterval: 25, fit: true }
  },
  interaction: { hover: true, navigationButtons: false, keyboard: true, zoomView: true }
};

const saveViewState = () => {
    if (!network.value) return;
    const pos = network.value.getViewPosition();
    const scale = network.value.getScale();
    localStorage.setItem('impression_view_state', JSON.stringify({ x: pos.x, y: pos.y, scale }));
};

const clearKGCache = async () => {
    // Clear all cache entries starting with impression_graph_[kgName]
    const prefix = `impression_graph_${kgName.value}`;
    const keys = await db.ai_cache.where('query').startsWith(prefix).primaryKeys();
    await db.ai_cache.bulkDelete(keys);
};

const initGraph = async () => {
  if (!canvas.value) return;

  const [visNetwork, visData] = await Promise.all([
      import('vis-network'),
      import('vis-data')
  ]);

  if (!nodes.value) nodes.value = new visData.DataSet<any>([]);
  if (!edges.value) edges.value = new visData.DataSet<any>([]);

  network.value = new visNetwork.Network(canvas.value, { nodes: nodes.value, edges: edges.value }, options);
  
  const savedState = localStorage.getItem('impression_view_state');
  if (savedState) {
      const { x, y, scale } = JSON.parse(savedState);
      network.value.moveTo({ position: { x, y }, scale, animation: false });
  }

  network.value.on('click', (params: any) => {
    // 🧠 Multi-click differentiation logic
    isWaitingForSecondClick = true;
    clearTimeout(clickTimer);

    clickTimer = setTimeout(() => {
        // If 350ms have passed and NO doubleClick event killed this timer, 
        // it is officially a SINGLE click.
        if (!isWaitingForSecondClick) return;
        isWaitingForSecondClick = false;

        if (interactionMode.value === 'view') {
            isLinkingMode.value = false;
            
            if (params.nodes.length > 0) {
                const nid = params.nodes[0];
                const isH = highlightedNodes.value.has(nid);
                
                if (isH) {
                    highlightedNodes.value.delete(nid);
                    nodes.value.update({ id: nid, size: 30, color: { border: undefined }, borderWidth: 3 });
                } else {
                    highlightedNodes.value.add(nid);
                    nodes.value.update({ id: nid, size: 55, color: { border: '#fbbf24' }, borderWidth: 8 });
                }
                handleNodeClick(nid);
            } else if (params.edges.length > 0) {
                const eid = params.edges[0];
                const isH = highlightedEdges.value.has(eid);
                
                if (isH) {
                    highlightedEdges.value.delete(eid);
                    edges.value.update({ id: eid, width: 2, color: { color: 'rgba(255,255,255,0.15)' } });
                } else {
                    highlightedEdges.value.add(eid);
                    edges.value.update({ id: eid, width: 8, color: { color: '#fbbf24' } });
                }
            }
        } else {
            // --- EDIT MODE CLICK ---
            if (params.nodes.length > 0) {
                const nid = params.nodes[0];
                resetSelection();
                selectedSourceNodeId.value = nid;
                nodes.value.update({ id: nid, borderWidth: 6, color: { border: '#22d3ee' } });
                handleNodeClick(nid);
                isEditingNode.value = true;
            } else if (params.edges.length > 0) {
                const eid = params.edges[0];
                const edgeData = edges.value.get(eid) as any;
                if (edgeData) {
                    selectedEdgeDetails.value = { id: eid, label: edgeData.label || '' };
                    edgeEditForm.value.label = edgeData.label || '';
                    isEditingEdge.value = true;
                    selectedNodeDetails.value = null;
                }
            } else {
                resetSelection();
                selectedNodeDetails.value = null;
                selectedEdgeDetails.value = null;
            }
        }
    }, 350); // Increased sensitivity threshold to 350ms
  });

  network.value.on('doubleClick', async (params: any) => {
    // 🛑 KILL THE SINGLE CLICK IMMEDIATELY
    isWaitingForSecondClick = false;
    clearTimeout(clickTimer);
    
    if (interactionMode.value === 'view') {
        if (params.nodes.length > 0) {
            const nid = params.nodes[0];
            network.value.moveTo({ position: network.value.getPosition(nid), animation: true });
            // Local re-layout logic could go here
        }
    } else {
        // --- EDIT MODE DOUBLE CLICK ---
        if (params.nodes.length > 0) {
            const nid = params.nodes[0];
            
            // Check if double clicking the already SELETED node -> Clone
            if (nid === selectedSourceNodeId.value && confirm("Clone this memory node?")) {
                const res = await apiService.cloneImpressionNode(nid);
                await clearKGCache();
                await loadGraph(res.id);
                return;
            }

            if (!candidateNodeId.value) {
                // Step 1: Set Candidate (Orange)
                candidateNodeId.value = nid;
                nodes.value.update({ id: nid, color: { border: '#f97316' }, borderWidth: 10 });
            } else {
                if (candidateNodeId.value === nid) {
                    // Double Click Self -> Cancel candidate
                    nodes.value.update({ id: nid, color: { border: undefined }, borderWidth: 3 });
                    candidateNodeId.value = null;
                } else {
                    // Step 2: Establish Edge
                    await performQuickLink(candidateNodeId.value, nid);
                    candidateNodeId.value = null; // Reset
                }
            }
        }
    }
  });

  network.value.on('hold', async (params: any) => {
    if (params.nodes.length === 0 && params.edges.length === 0) {
        // Global Flow toggle on space hold
        isPhysicsEnabled.value = !isPhysicsEnabled.value;
        return;
    }

    if (interactionMode.value === 'edit') {
        if (params.nodes.length > 0) {
            const nodeId = params.nodes[0] as string;
            if (confirm(`⚠️ DELETE Node?`)) {
                await syncService.deleteImpressionNode(nodeId);
                await clearKGCache();
                await loadGraph();
            }
        } else if (params.edges.length > 0) {
            if (confirm(`⚠️ SEVER Edge?`)) {
                await syncService.deleteImpressionLink(params.edges[0]);
                await clearKGCache();
                await loadGraph();
            }
        } else {
            const title = prompt("New Node Title:");
            if (title) {
                const res = await syncService.createImpressionNode({ title, content: '', nodeType: 'general', kgName: kgName.value });
                await clearKGCache();
                await loadGraph(res.id);
            }
        }
    }
  });

  network.value.on('dragEnd', (params: any) => {
    saveViewState();
    // After dragging, we let it settle for a short bit then freeze it
    if (isPhysicsEnabled.value && params.nodes.length > 0) {
        setTimeout(() => {
            const allIds = nodes.value.getIds();
            const updates = allIds.map((id: string) => ({ id, physics: false }));
            nodes.value.update(updates);
        }, 1000);
    }
  });

  network.value.on('dragStart', (params: any) => {
    if (!isPhysicsEnabled.value || params.nodes.length === 0) return;
    
    // Use the recursive collector to find the string/cluster
    const rootId = params.nodes[0];
    const clusterIds = getConnectedCluster(rootId);
    
    const updates = nodes.value.getIds().map((id: string) => ({
        id,
        physics: clusterIds.has(id)
    }));
    nodes.value.update(updates);
  });

  network.value.on('zoom', saveViewState);
  loadShelves();
};

const getConnectedCluster = (startId: string): Set<string> => {
    const visited = new Set<string>();
    const stack = [startId];
    while (stack.length > 0) {
        const id = stack.pop()!;
        if (!visited.has(id)) {
            visited.add(id);
            const neighbors = network.value.getConnectedNodes(id) as string[];
            neighbors.forEach(n => { if (!visited.has(n)) stack.push(n); });
        }
    }
    return visited;
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
        await syncService.updateImpressionNode(selectedNodeDetails.value.id, updated);
        selectedNodeDetails.value.deskShelfId = shelfId;
        const existingNode = nodes.value.get(selectedNodeDetails.value.id) as any;
        if (existingNode && existingNode.raw) {
            existingNode.raw.deskShelfId = shelfId;
            nodes.value.update(existingNode);
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

const resetSelection = () => {
    if (selectedSourceNodeId.value) {
        const node = nodes.value.get(selectedSourceNodeId.value);
        if (node) nodes.value.update({ id: selectedSourceNodeId.value, borderWidth: 3, color: { border: undefined } });
        selectedSourceNodeId.value = null;
    }
};

const performQuickLink = async (src: string, tgt: string) => {
    try {
        await syncService.createImpressionLink({ sourceId: src, targetId: tgt, label: '', kgName: kgName.value });
        await clearKGCache();
        resetSelection();
        await loadGraph(src);
    } catch (e) { console.error(e); resetSelection(); }
};

watch(isPhysicsEnabled, () => {
    if (network.value) {
        // We always keep physics enabled on the network level to allow selective physics
        network.value.setOptions({ physics: { enabled: true } }); 
        
        // But we pin/unpin individual nodes based on the mode
        const allIds = nodes.value.getIds();
        const updates = allIds.map((id: string) => ({
            id,
            physics: false // Default to fixed even if Flow is on, wait for drag
        }));
        nodes.value.update(updates);
    }
});

const handleNodeClick = (nodeId: string) => {
  const node = nodes.value.get(nodeId);
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
    const nid = nodeId || centerNodeId.value || '';
    const cacheKey = `impression_graph_${kgName.value}_${nid}`;
    
    let data;
    const cached = await syncService.getAICache(cacheKey);
    if (cached) {
        data = JSON.parse(cached);
    } else {
        data = await syncService.getImpressionGraph(nid, kgName.value);
        if (data && data.nodes && data.nodes.length === 0 && (nodeId || centerNodeId.value)) {
            console.warn("Requested specific graph center but got 0 nodes. Potential kg_name mismatch or legacy data issue.");
        }
        // Cache for 3 hours
        await syncService.setAICache(cacheKey, JSON.stringify(data), 3);
    }
    
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
            color: { border: n.id === nid ? '#22d3ee' : '#4338ca', background: '#1e293b' },
            raw: n
        };
    }));

    nodes.value.clear();
    nodes.value.add(visNodes);
    edges.value.clear();
    edges.value.add(data.edges.map((e: any) => ({ 
        id: e.id, from: e.sourceId, to: e.targetId, label: e.label,
        font: { color: kgName.value === 'default' ? '#94a3b8' : '#22d3ee' }
    })));
    
    // After load, respect the physics flag but allow a small settle window
    if (!isPhysicsEnabled.value) {
        const allIds = nodes.value.getIds();
        nodes.value.update(allIds.map((id: string) => ({ id, physics: false })));
    }
    
    if (nid && network.value && nodes.value.length > 0) {
        setTimeout(() => {
            if (network.value && nodes.value.get(nid)) {
                network.value.fit({ nodes: [nid], animation: true });
                // NEW: Sync selectedNodeDetails with the newly loaded data
                const fresh = nodes.value.get(nid) as any;
                if (fresh) selectedNodeDetails.value = fresh.raw;
            } else if (network.value && nodes.value.length > 0) {
                network.value.fit({ animation: true });
            }
            centerNodeId.value = nid;
            localStorage.setItem('impression_last_center', nid);
        }, 300);
    } else if (localStorage.getItem('impression_view_state') && network.value) {
        const { x, y, scale } = JSON.parse(localStorage.getItem('impression_view_state')!);
        network.value.moveTo({ position: { x, y }, scale, animation: false });
    } else if (network.value && nodes.value.length > 0) {
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
        await pinUniverseToDesk(kgName.value);
        alert(`Successfully pinned '${kgName.value}' to Main Desktop!`);
    } catch (e) { 
        alert('Pinning failed.'); 
    }
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
            const newNode = await syncService.createImpressionNode({ title, content: '', nodeType: 'general', kgName: kgName.value });
            await loadGraph(newNode.id);
            commandInput.value = '';
            fetchKGs();
        } else if (cmd === '/edit') {
            const title = args.join(' ');
            if (!title) throw new Error('Usage: /edit [Node Title]');
            const node = await resolveNodeByTitle(title);
            if (!node) throw new Error(`Node "${title}" not found.`);
            handleNodeClick(node.id);
            isEditingNode.value = true;
            commandInput.value = '';
        } else if (cmd === '/model') {
            const mode = args[0]?.toLowerCase();
            if (mode === 'edit') {
                interactionMode.value = 'edit';
                commandResults.value = [{ id: 'm-edit', title: 'Edit Mode Active: [Hold] to Delete, [Click 2 nodes] to link.', resultType: 'info' }];
            } else {
                interactionMode.value = 'view';
                resetSelection();
                commandResults.value = [{ id: 'm-view', title: 'View Mode Active: [Click] to focus, [Hold] to refine.', resultType: 'info' }];
            }
            commandInput.value = '';
        } else if (cmd === '/add' && args[0] === 'edge') {
            const argString = args.slice(1).join(' ');
            const matches = argString.match(/(?:[^\s"]+|"[^"]*")+/g) || [];
            const cleanArgs = matches.map(a => a.replace(/"/g, ''));

            if (cleanArgs.length < 2) throw new Error('Source and Target titles required. Usage: /add edge "Source" "Target" [Label]');
            const src = await resolveNodeByTitle(cleanArgs[0]);
            const tgt = await resolveNodeByTitle(cleanArgs[1]);
            const label = cleanArgs[2] || '';

            if (!src || !tgt) throw new Error(`Node not found: ${!src ? cleanArgs[0] : cleanArgs[1]}`);
            await syncService.createImpressionLink({ sourceId: src.id, targetId: tgt.id, label, kgName: kgName.value });
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
            if (!args[0]) throw new Error('KG name required. Usage: /kg [name] OR /kg copy [src] [tgt]');
            
            if (args[0] === 'copy') {
                if (args.length < 3) throw new Error('Source and Target names required. Usage: /kg copy [source] [target]');
                await apiService.duplicateKG(args[1], args[2]);
                commandResults.value = [{ id: 'kg-c', title: `Universe "${args[1]}" cloned to "${args[2]}".`, resultType: 'info' }];
            } else {
                switchKG(args[0]);
            }
            commandInput.value = '';
        } else if (cmd === '/layout') {
            const mode = args[0]?.toLowerCase();
            if (mode === 'tree' || mode === 'hierarchy') {
                network.value.setOptions({ layout: { hierarchical: { enabled: true, direction: 'UD', sortMethod: 'directed' } } });
                commandResults.value = [{ id: 'l-t', title: 'Switched to Hierarchical Layout.', resultType: 'info' }];
            } else {
                // Global Shuffle: Disable hierarchy and unfreeze everyone temporarily
                network.value.setOptions({ layout: { hierarchical: { enabled: false } } });
                const allIds = nodes.value.getIds() as string[];
                nodes.value.update(allIds.map((id: string) => ({ id, physics: true })));
                
                commandResults.value = [{ id: 'l-f', title: 'Global Force Shuffle: Unfreezing for 5s...', resultType: 'info' }];
                
                // Freeze again after stabilization
                setTimeout(() => {
                    const latestIds = nodes.value.getIds() as string[];
                    nodes.value.update(latestIds.map((id: string) => ({ id, physics: false })));
                    commandResults.value = [{ id: 'l-f2', title: 'Layout Stabilized & Frozen.', resultType: 'info' }];
                }, 5000);
            }
            commandInput.value = '';
        } else if (cmd === '/rank') {
            const algo = args[0]?.toLowerCase() || 'centrality';
            const direction = args[1]?.toLowerCase() || 'total';
            performGraphAnalysis(algo, direction);
            commandResults.value = [{ id: 'r-a', title: `Graph Scaled by ${algo} (${direction}).`, resultType: 'info' }];
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

const COMMAND_LIST = ['/add', '/edit', '/model', '/search', '/list', '/kg', '/layout', '/rank', '/pin', '/help'];

const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Tab') {
        e.preventDefault(); // Prevent focus change
        const input = commandInput.value.trim().toLowerCase();
        if (!input) return;

        const match = COMMAND_LIST.find(c => c.startsWith(input));
        if (match) {
            commandInput.value = match + ' ';
        }
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
            alert('Universe Restored Successfully.');
            if (importFileRef.value) importFileRef.value.value = ''; // Reset input
            await loadGraph(); // Full fresh reload
        } catch (err: any) { 
            console.error(err);
            alert(`Restore Failed: ${err.response?.data?.error || err.message}`); 
            if (importFileRef.value) importFileRef.value.value = '';
        }
    };
    reader.readAsText(file);
};

const goToRandomNode = async () => {
    try {
        const all = nodes.value.getIds();
        if (all.length > 0) {
             const rid = all[Math.floor(Math.random() * all.length)] as string;
             if (network.value) network.value.focus(rid, { animation: true, scale: 1.0 });
             const fresh = nodes.value.get(rid) as any;
             if (fresh) selectedNodeDetails.value = fresh.raw;
        } else {
             const res = await apiService.getImpressionRandom();
             if (res.id) loadGraph(res.id);
        }
    } catch (e) { console.error(e); }
};

const saveNodeEdits = async () => {
    if (!selectedNodeDetails.value) return;
    try {
        const updated = await syncService.updateImpressionNode(selectedNodeDetails.value.id, editForm.value);
        await clearKGCache();
        selectedNodeDetails.value = updated;
        isEditingNode.value = false;
        loadGraph(updated.id);
    } catch (e) { console.error(e); }
};

const deleteNode = async (id: string) => {
    if (!confirm('Destroy this memory node?')) return;
    try { 
        await syncService.deleteImpressionNode(id); 
        await clearKGCache();
        selectedNodeDetails.value = null; 
        await loadGraph(); 
    } catch (e) { console.error(e); }
};

const saveEdgeEdits = async () => {
    if (!selectedEdgeDetails.value) return;
    try {
        await syncService.updateImpressionLink(selectedEdgeDetails.value.id, { label: edgeEditForm.value.label });
        await clearKGCache();
        selectedEdgeDetails.value = null;
        isEditingEdge.value = false;
        await loadGraph();
    } catch (e) { console.error(e); }
};

const deleteEdge = async (id: string) => {
    if (!confirm('Sever this relationship bond?')) return;
    try {
        await syncService.deleteImpressionLink(id);
        await clearKGCache();
        selectedEdgeDetails.value = null;
        isEditingEdge.value = false;
        await loadGraph();
    } catch (e) { console.error(e); }
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
        await syncService.createImpressionLink({ sourceId: selectedNodeDetails.value.id, targetId: targetNode.value.id, label: linkLabel.value, kgName: kgName.value });
        await clearKGCache();
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

// --- Advanced Graph Analysis Algorithms ---
const performGraphAnalysis = (algo: string, dir: string) => {
    const allNodes = nodes.value.get();
    const allEdges = edges.value.get();
    const scores: Record<string, number> = {};
    
    // Initialize
    allNodes.forEach((n: any) => scores[n.id] = 1);

    if (algo === 'centrality') {
        allEdges.forEach((e: any) => {
            if (dir === 'in' || dir === 'total') scores[e.to] = (scores[e.to] || 0) + 1;
            if (dir === 'out' || dir === 'total') scores[e.from] = (scores[e.from] || 0) + 1;
        });
    } else if (algo === 'pagerank') {
        const d = 0.85; // Damping factor
        const iterations = 10;
        for (let i = 0; i < iterations; i++) {
            const newScores: Record<string, number> = {};
            allNodes.forEach((n: any) => {
                let rankSum = 0;
                const incoming = allEdges.filter((e: any) => e.to === n.id);
                incoming.forEach((e: any) => {
                    const outboundCount = allEdges.filter((oe: any) => oe.from === e.from).length || 1;
                    rankSum += (scores[e.from] || 0) / outboundCount;
                });
                newScores[n.id] = (1 - d) + d * rankSum;
            });
            Object.assign(scores, newScores);
        }
    }

    // Scale nodes based on scores
    const minS = Math.min(...Object.values(scores));
    const maxS = Math.max(...Object.values(scores));
    const range = maxS - minS || 1;

    const updates = allNodes.map((n: any) => {
        const normalized = (scores[n.id] - minS) / range;
        const size = 20 + (normalized * 50); // Min size 20, Max 70
        return { id: n.id, size, font: { size: 12 + (normalized * 10) } };
    });
    nodes.value.update(updates);
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
    if (!nodes.value) return [];
    return nodes.value.get().filter((n: any) => n.id !== selectedNodeDetails.value?.id).map((n: any) => ({
        id: n.id,
        title: n.label,
        raw: n.raw
    }));
});

const onNodeSelectChange = (e: Event) => {
    const val = (e.target as HTMLSelectElement).value;
    const node = nodes.value.get(val) as any;
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
                <div class="mode-badge" :class="interactionMode" @click="interactionMode = interactionMode === 'view' ? 'edit' : 'view'">{{ interactionMode.toUpperCase() }}</div>
                <span class="prompt">λ</span>
                <input 
                    type="text" 
                    v-model="commandInput" 
                    placeholder="Enter command... (/add, /search, /list, /kg, /help)" 
                    @keyup.enter="executeCommand"
                    @keydown="handleKeyDown"
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
                    <li><code>🔭 [VIEW 模式]</code> - 單擊標記/取消標記並開啟詳情；拖拽觸發「智慧局部物理」(拉動一串鈴鐺)</li>
                    <li><code>🛠️ [EDIT 模式]</code> - 點擊選中(藍圈)並編輯；雙擊設為候選點(橘光)後雙擊第二點連線；長按刪除</li>
                    <li><code>/layout [tree/force]</code> - 切換結構化層級佈局或自由力學模式</li>
                    <li><code>/rank [centrality/pagerank]</code> - 執行節點權重分析，自動縮放重要節點</li>
                    <li><code>/edit [標題]</code> - 快速跳轉至節點並開啟編輯面板</li>
                    <li><code>/kg [名稱]</code> - 切換知識宇宙；<code>/kg copy [源] [目]</code> - 複製整個宇宙</li>
                    <li><code>[全局操作]</code> - 空白處長按開關【全局物理】；滾輪縮放與平移位置會自動記憶</li>
                    <li><code>λ 指令系統</code> - 支援 /add, /search, /list, /pin 等所有進階管理功能</li>
                </ul>
            </div>
        </div>

        <!-- Studio Toolbox -->
        <div class="studio-toolbox glass neon-border">
            <div class="tool-btn" :class="interactionMode" @click="interactionMode = interactionMode === 'view' ? 'edit' : 'view'">
                <div class="t-icon">{{ interactionMode === 'view' ? '🔭' : '🛠️' }}</div>
                <div class="t-label">{{ interactionMode.toUpperCase() }}</div>
            </div>
            <div class="t-sep"></div>
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

        <!-- Edge Explorer Bottom Card -->
        <div v-if="selectedEdgeDetails" class="node-explorer-card glass neon-border" style="max-width: 600px; bottom: 80px;">
            <button class="card-close" @click="selectedEdgeDetails = null">×</button>
            <div class="card-flex" style="flex-direction: column; padding: 30px;">
                <div class="edge-edit-head" style="margin-bottom: 20px;">
                    <h2 style="color: #22d3ee; margin: 0; font-size: 1.2rem;">Relational Bond</h2>
                    <p style="opacity: 0.5; font-size: 0.8rem;">Modifying the link connection.</p>
                </div>
                
                <div class="in-field">
                    <label>Relationship Label</label>
                    <input v-model="edgeEditForm.label" placeholder="e.g. 'friend of', 'belongs to'..." class="m-input" />
                </div>

                <div class="btn-group">
                    <button class="confirm-link-btn" style="flex: 2; margin-top: 0;" @click="saveEdgeEdits">Commit Changes</button>
                    <button class="g-btn del-b" style="flex: 0.5; height: 48px; font-size: 1.2rem; display: flex; align-items: center; justify-content: center;" @click="deleteEdge(selectedEdgeDetails.id)">🗑️</button>
                </div>
            </div>
        </div>

        <!-- Snapshot Panel -->
        <div v-if="showExportPanel" class="snapshot-panel glass neon-border">
            <div class="panel-head"><h4>Snapshot Studio</h4><button class="close-p" @click="showExportPanel = false">×</button></div>
            <div class="panel-body">
                <div class="color-picker-grid">
                    <div v-for="c in [
                        '#0f172a', '#1e293b', '#000000', '#ffffff', 
                        '#164e63', '#4c1d95', '#7c2d12', '#064e3b',
                        '#be123c', '#ca8a04'
                    ]" :key="c" :style="{ background: c }"
                        @click="exportBgColor = c; exportBgImage = null" class="c-dot"
                        :class="{ active: exportBgColor === c && !exportBgImage }"></div>
                </div>

                <div class="texture-label">Cosmic Textures</div>
                <div class="texture-grid">
                    <div v-for="(img, name) in {
                        'Deep Space': 'https://images.unsplash.com/photo-1464802686167-b939a6910659?q=80&w=1000',
                        'Blueprint': 'https://images.unsplash.com/photo-1581092160562-40aa08e78837?q=80&w=1000',
                        'Cyberpunk': 'https://images.unsplash.com/photo-1605810230434-7631ac76ec81?q=80&w=1000',
                        'Silk': 'https://images.unsplash.com/photo-1557683316-973673baf926?q=80&w=1000'
                    }" :key="name" class="t-item" @click="exportBgImage = img; exportBgColor = '#ffffff'" :class="{ active: exportBgImage === img }">
                        <img :src="img" />
                        <span class="t-name">{{ name }}</span>
                    </div>
                </div>

                <button class="confirm-link-btn" @click="exportAsImage">Render & Download</button>
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

.snapshot-panel { position: absolute; top: 95px; right: 25px; width: 280px; padding: 25px; z-index: 2200; box-shadow: 0 15px 50px rgba(0,0,0,0.8); }
.panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.panel-head h4 { margin: 0; font-size: 0.9rem; letter-spacing: 2px; color: #f1f5f9; text-transform: uppercase; font-weight: 900; }
.close-p { background: none; border: none; color: #475569; font-size: 1.5rem; cursor: pointer; transition: 0.2s; }
.close-p:hover { color: #f1f5f9; }

.texture-label { font-size: 0.65rem; color: #64748b; font-weight: 800; text-transform: uppercase; margin: 15px 0 10px; border-top: 1px solid rgba(255,255,255,0.05); padding-top: 15px; }
.texture-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 25px; }
.t-item { height: 60px; border-radius: 12px; overflow: hidden; position: relative; cursor: pointer; border: 2px solid transparent; transition: 0.2s; }
.t-item img { width: 100%; height: 100%; object-fit: cover; opacity: 0.5; transition: 0.3s; }
.t-item:hover img { opacity: 0.8; }
.t-item.active { border-color: #22d3ee; }
.t-item.active img { opacity: 1; }
.t-name { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; font-size: 0.65rem; font-weight: 900; color: white; text-shadow: 0 2px 4px rgba(0,0,0,0.8); }

.color-picker-grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; margin-bottom: 15px; }
.c-dot { aspect-ratio: 1; border-radius: 50%; cursor: pointer; border: 2px solid rgba(255,255,255,0.1); transition: 0.2s; position: relative; }
.c-dot:hover { transform: scale(1.15); border-color: rgba(34, 211, 238, 0.4); }
.c-dot.active { border-color: #22d3ee; transform: scale(1.15); box-shadow: 0 0 15px rgba(34, 211, 238, 0.4); }

.in-field { margin-bottom: 25px; }
.in-field label { display: block; font-size: 0.7rem; color: #64748b; font-weight: 800; text-transform: uppercase; margin-bottom: 8px; }
.premium-select, .premium-textarea, .m-input { width: 100%; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.1); color: white; padding: 14px; border-radius: 12px; font-family: inherit; transition: 0.3s; }
.premium-select:focus, .premium-textarea:focus, .m-input:focus { border-color: #22d3ee; background: rgba(0,0,0,0.5); outline: none; }
.premium-textarea { height: 180px; resize: none; }
.confirm-link-btn { width: 100%; height: 48px; background: #22d3ee; color: #080c14; border: none; border-radius: 12px; font-weight: 800; cursor: pointer; margin-top: auto; transition: 0.3s; }
.confirm-link-btn:hover { background: #67e8f9; transform: translateY(-2px); box-shadow: 0 10px 20px rgba(34, 211, 238, 0.3); }
.confirm-link-btn:disabled { opacity: 0.3; cursor: not-allowed; }

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
.mode-badge { position: absolute; left: 10px; top: -25px; font-size: 0.7rem; font-weight: 800; padding: 2px 8px; border-radius: 4px; letter-spacing: 1px; color: #fff; background: #64748b; opacity: 0.8; }
.mode-badge.edit { background: #f43f5e; box-shadow: 0 0 10px rgba(244, 63, 94, 0.4); }
.mode-badge.view { background: #22d3ee; }

</style>
