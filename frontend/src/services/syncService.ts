import { apiService } from './api';
import { db, type LocalSnippet } from './localDb';
import { reactive } from 'vue';

export const syncService = reactive({
    mode: (localStorage.getItem('sync_mode') as 'local' | 'cloud') || 'local',

    setMode(newMode: 'local' | 'cloud') {
        this.mode = newMode;
        localStorage.setItem('sync_mode', newMode);
        console.log(`🌐 Sync Mode switched to: ${newMode}`);
        // 可以選擇在這裡觸發全域重新整理
    },

    // Snippets Methods (EverSync Enhanced)
    async getSnippets(parentId: string | null = null) {
        if (this.mode === 'cloud') {
            console.log('📡 [Cloud Mode] Fetching snippets directly from API...');
            return await apiService.getSnippets(parentId);
        }
        const pid = parentId || 'root';
        return await db.snippets
            .where('parentId')
            .equals(pid)
            .sortBy('sortOrder');
    },

    async refreshSnippets(parentId: string | null = null, all: boolean = false) {
        try {
            const remoteItems = await apiService.getSnippets(parentId, all);
            const pendingDeletes = await db.sync_queue.where({ entityType: 'snippet', action: 'DELETE' }).toArray();
            const deletedIds = new Set(pendingDeletes.map(a => a.entityId));

            await db.transaction('rw', db.snippets, async () => {
                for (const item of remoteItems) {
                    if (deletedIds.has(item.id)) continue;
                    await db.snippets.put({
                        ...item,
                        parentId: item.parentId || 'root',
                        syncStatus: 'synced',
                        updatedAt: new Date().toISOString()
                    });
                }
            });
            return remoteItems;
        } catch (err) {
            console.error('Refresh snippets failed:', err);
            throw err;
        }
    },

    async createSnippet(data: { parentId: string | null; name: string; content?: string; isFolder: boolean; sortOrder?: number }) {
        if (this.mode === 'cloud') {
            const res = await apiService.createSnippet(data);
            const newSnippet = {
                ...res,
                parentId: res.parentId || 'root',
                syncStatus: 'synced' as const,
                updatedAt: new Date().toISOString()
            };
            await db.snippets.put(newSnippet);
            return newSnippet;
        }

        const id = crypto.randomUUID();
        const newSnippet: LocalSnippet = {
            id,
            parentId: data.parentId || 'root',
            name: data.name,
            content: data.content || '',
            isFolder: data.isFolder,
            sortOrder: data.sortOrder || 0,
            updatedAt: new Date().toISOString(),
            syncStatus: 'pending'
        };
        await db.snippets.add(newSnippet);
        await db.sync_queue.add({ action: 'CREATE', entityType: 'snippet', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return newSnippet;
    },

    async updateSnippet(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateSnippet(id, data);
            await db.snippets.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.snippets.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'snippet', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async moveSnippet(id: string, sortOrder: number) {
        if (this.mode === 'cloud') {
            await apiService.updateSnippet(id, { sortOrder });
            await db.snippets.update(id, { sortOrder, syncStatus: 'synced' });
            return;
        }
        // Defensive Merge: ensure we don't overwrite server data with empty fields during a move
        const existing = await db.snippets.get(id);
        const name = existing?.name || '';
        const content = existing?.content || '';
        
        await db.snippets.update(id, { sortOrder, syncStatus: 'pending' });
        await db.sync_queue.add({ 
            action: 'UPDATE', 
            entityType: 'snippet', 
            entityId: id, 
            data: { name, content, sortOrder }, // Send core fields to repair potential corruption
            timestamp: Date.now() 
        });
        this.requestSync();
    },

    async deleteSnippet(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteSnippet(id);
            await db.snippets.delete(id);
            return;
        }
        const item = await db.snippets.get(id);
        const isGhost = item && (!item.name || item.name.trim() === '');
        
        await db.snippets.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'snippet', entityId: id, data: null, timestamp: Date.now() });
        
        if (isGhost) {
            this.syncNow();
        } else {
            this.requestSync();
        }
    },

    // --- Bookmarks Methods ---
    async getBookmarks(parentId?: string | 'root') {
        if (this.mode === 'cloud') {
            console.log('📡 [Cloud Mode] Fetching bookmarks directly from API...');
            return await apiService.getBookmarks(parentId);
        }
        const pid = parentId || 'root';
        return await db.bookmarks.where('parentId').equals(pid).sortBy('sortOrder');
    },

    async refreshBookmarks(parentId?: string | 'root', all: boolean = false) {
        const remoteItems = await apiService.getBookmarks(parentId, all);
        const pendingDeletes = await db.sync_queue.where({ entityType: 'bookmark', action: 'DELETE' }).toArray();
        const deletedIds = new Set(pendingDeletes.map(a => a.entityId));

        await db.transaction('rw', db.bookmarks, async () => {
            for (const item of remoteItems) {
                if (deletedIds.has(item.id)) continue;
                await db.bookmarks.put({ 
                    ...item, 
                    parentId: item.parentId || 'root',
                    syncStatus: 'synced', 
                    updatedAt: new Date().toISOString() 
                });
            }
        });
        return remoteItems;
    },

    async addBookmark(data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.addBookmark(data);
            const newBookmark = {
                ...res,
                parentId: res.parentId || 'root',
                syncStatus: 'synced' as const,
                updatedAt: new Date().toISOString()
            };
            await db.bookmarks.put(newBookmark);
            return newBookmark;
        }

        const id = crypto.randomUUID();
        const newBookmark = { ...data, id, syncStatus: 'pending', updatedAt: new Date().toISOString() };
        await db.bookmarks.add(newBookmark);
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookmark', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return newBookmark;
    },

    async updateBookmark(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateBookmark(id, data);
            await db.bookmarks.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.bookmarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookmark', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async moveBookmark(id: string, sortOrder: number) {
        if (this.mode === 'cloud') {
            await apiService.updateBookmark(id, { sortOrder });
            await db.bookmarks.update(id, { sortOrder, syncStatus: 'synced' });
            return;
        }
        // Defensive Merge: ensure we don't overwrite server data with empty fields during a move
        const existing = await db.bookmarks.get(id);
        const title = existing?.title || '';
        const url = existing?.url || null;
        const category = existing?.category || 'General';
        
        await db.bookmarks.update(id, { sortOrder, syncStatus: 'pending' });
        await db.sync_queue.add({ 
            action: 'UPDATE', 
            entityType: 'bookmark', 
            entityId: id, 
            data: { title, url, category, sortOrder }, // Send core fields to repair potential corruption
            timestamp: Date.now() 
        });
        this.requestSync();
    },

    async deleteBookmark(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteBookmark(id);
            await db.bookmarks.delete(id);
            return;
        }
        const item = await db.bookmarks.get(id);
        const isGhost = item && (!item.title || item.title.trim() === '');

        await db.bookmarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookmark', entityId: id, data: null, timestamp: Date.now() });

        if (isGhost) {
            this.syncNow();
        } else {
            this.requestSync();
        }
    },

    // --- Desk Methods ---
    async getShelves() {
        if (this.mode === 'cloud') {
            console.log('📡 [Cloud Mode] Fetching shelves directly from API...');
            return await apiService.getShelves();
        }
        return await db.shelves.orderBy('sortOrder').toArray();
    },
    async refreshShelves() {
        const remote = await apiService.getShelves();
        const pendingDeletes = await db.sync_queue.where({ entityType: 'shelf', action: 'DELETE' }).toArray();
        const deletedIds = new Set(pendingDeletes.map(a => a.entityId));

        await db.transaction('rw', db.shelves, async () => {
            for (const s of remote) {
                if (deletedIds.has(s.id)) continue;
                await db.shelves.put({ ...s, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async createShelf(data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.createShelf(data);
            await db.shelves.put({ ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return { id: res.id };
        }
        const id = crypto.randomUUID();
        await db.shelves.add({ ...data, id, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'shelf', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateShelf(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateShelf(id, data);
            await db.shelves.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.shelves.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'shelf', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteShelf(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteShelf(id);
            await db.shelves.delete(id);
            return;
        }
        await db.shelves.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'shelf', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async getDeskItems(shelfId?: string) {
        if (this.mode === 'cloud') {
            const sid = shelfId || 'null';
            console.log(`📡 [Cloud Mode] Fetching desk items for shelf: ${sid} from API...`);
            return await apiService.getDeskItems(sid === 'null' ? undefined : sid);
        }
        const sid = shelfId || 'null';
        return await db.deskItems.where('shelfId').equals(sid).sortBy('sortOrder');
    },
    async refreshDeskItems(shelfId: string = 'null') {
        const remote = await apiService.getDeskItems(shelfId === 'null' ? undefined : shelfId);
        const pendingDeletes = await db.sync_queue.where({ entityType: 'deskItem', action: 'DELETE' }).toArray();
        const deletedIds = new Set(pendingDeletes.map(a => a.entityId));

        await db.transaction('rw', db.deskItems, async () => {
            for (const item of remote) {
                if (deletedIds.has(item.id)) continue;
                await db.deskItems.put({ ...item, shelfId: item.shelfId || 'null', syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async addDeskItem(data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.addDeskItem(data);
            await db.deskItems.put({ ...res, shelfId: res.shelfId || 'null', syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return { id: res.id };
        }
        const id = crypto.randomUUID();
        await db.deskItems.add({ ...data, id, shelfId: data.shelfId || 'null', syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'deskItem', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateDeskItem(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateDeskItem(id, data);
            await db.deskItems.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.deskItems.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'deskItem', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteDeskItem(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteDeskItem(id);
            await db.deskItems.delete(id);
            return;
        }
        await db.deskItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'deskItem', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async moveDeskItem(id: string, sortOrder: number) {
        if (this.mode === 'cloud') {
            await apiService.updateDeskItem(id, { sortOrder });
            await db.deskItems.update(id, { sortOrder, syncStatus: 'synced' });
            return;
        }
        const item = await db.deskItems.get(id);
        const shelfId = item?.shelfId || 'null';
        await db.deskItems.update(id, { sortOrder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'deskItem', entityId: id, data: { shelfId: shelfId === 'null' ? null : shelfId, sortOrder }, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Bookcase Methods ---
    async getBookcase() {
        if (this.mode === 'cloud') {
            console.log('📡 [Cloud Mode] Fetching bookcase directly from API...');
            return await apiService.getBookcase();
        }
        return await db.bookcase.toArray();
    },
    async refreshBookcase() {
        const remote = await apiService.getBookcase();
        const pendingDeletes = await db.sync_queue.where({ entityType: 'book', action: 'DELETE' }).toArray();
        const deletedIds = new Set(pendingDeletes.map(a => a.entityId));

        await db.transaction('rw', db.bookcase, async () => {
            for (const b of remote) {
                if (deletedIds.has(b.id)) continue;
                await db.bookcase.put({ ...b, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async addBookToBookcase(data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.addBookToBookcase(data);
            await db.bookcase.put({ ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        const id = crypto.randomUUID();
        await db.bookcase.add({ 
            folder: '', 
            sortOrder: 0, 
            ...data, 
            id, 
            syncStatus: 'pending', 
            updatedAt: new Date().toISOString() 
        });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'book', entityId: id, data: { folder: '', sortOrder: 0, ...data }, timestamp: Date.now() });
        this.requestSync();
    },
    async updateBookFolder(id: string, folder: string) {
        if (this.mode === 'cloud') {
            await apiService.updateBookFolder(id, folder);
            await db.bookcase.update(id, { folder, syncStatus: 'synced' });
            return;
        }
        await db.bookcase.update(id, { folder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'book', entityId: id, data: { folder }, timestamp: Date.now() });
        this.requestSync();
    },
    async moveBook(id: string, sortOrder: number) {
        if (this.mode === 'cloud') {
            await apiService.updateBookSortOrder(id, sortOrder);
            await db.bookcase.update(id, { sortOrder, syncStatus: 'synced' });
            return;
        }
        await db.bookcase.update(id, { sortOrder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'book', entityId: id, data: { sortOrder }, timestamp: Date.now() });
        this.requestSync();
    },
    async removeBook(id: string) {
        if (this.mode === 'cloud') {
            await apiService.removeBook(id);
            await db.bookcase.delete(id);
            return;
        }
        await db.bookcase.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'book', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async getBookNotes(bookId: string) {
        if (this.mode === 'cloud') {
            console.log(`📡 [Cloud Mode] Fetching notes for book: ${bookId} from API...`);
            return await apiService.getBookNotes(bookId);
        }
        return await db.bookNotes.where('bookId').equals(bookId).toArray();
    },
    async refreshBookNotes(bookId: string) {
        const remote = await apiService.getBookNotes(bookId);
        await db.transaction('rw', db.bookNotes, async () => {
            for (const n of remote) {
                await db.bookNotes.put({ ...n, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async addBookNote(bookId: string, data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.addBookNote(bookId, data);
            await db.bookNotes.put({ ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return { id: res.id };
        }
        const id = crypto.randomUUID();
        await db.bookNotes.add({ ...data, id, bookId, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookNote', entityId: id, data: { ...data, id, bookId }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateBookNote(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateBookNote(id, data);
            await db.bookNotes.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.bookNotes.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookNote', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async removeBookNote(id: string) {
        if (this.mode === 'cloud') {
            await apiService.removeBookNote(id);
            await db.bookNotes.delete(id);
            return;
        }
        await db.bookNotes.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookNote', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Remarks (Obs) Methods ---
    async getRemarks() {
        if (this.mode === 'cloud') {
            console.log('📡 [Cloud Mode] Fetching remarks directly from API...');
            const remote = await apiService.getRemarks();
            // 雲端模式下 API 回傳格式可能不同，這裡通常回傳原始資料
            return remote;
        }
        return await db.remarks.toArray();
    },
    async refreshRemarks() {
        const remote = await apiService.getRemarks();
        await db.transaction('rw', [db.remarks, db.remarkItems], async () => {
             for (const c of remote.containers || []) {
                 await db.remarks.put({ ...c, syncStatus: 'synced', updatedAt: new Date().toISOString() });
                 
                 // Also ensure nested items are in the remarkItems table for easier offline access
                 if (c.items && Array.isArray(c.items)) {
                     for (const it of c.items) {
                         await db.remarkItems.put({
                             ...it,
                             containerId: c.id,
                             syncStatus: 'synced',
                             updatedAt: new Date().toISOString()
                         });
                     }
                 }
             }
             // Handle items that might be sent outside containers (if API does that)
             for (const it of remote.items || []) {
                 await db.remarkItems.put({ ...it, syncStatus: 'synced', updatedAt: new Date().toISOString() });
             }
        });
        return remote;
    },
    async createRemark(data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.createRemark(data);
            await db.remarks.put({ ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return { id: res.id };
        }
        const id = crypto.randomUUID();
        await db.remarks.add({ ...data, id, isPinned: false, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'remark', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateRemark(id: string, data: any) {
        if (this.mode === 'cloud') {
            await apiService.updateRemark(id, data);
            await db.remarks.update(id, { ...data, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }
        await db.remarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'remark', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteRemark(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteRemark(id);
            await db.remarks.delete(id);
            return;
        }
        await db.remarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remark', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async addRemarkItem(data: { containerId: string, logId: string, log?: any }) {
        if (this.mode === 'cloud') {
            const res = await apiService.addRemarkItem(data);
            await db.remarkItems.put({ ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return { id: res.id };
        }
        const id = crypto.randomUUID();
        // data.log is provided from the UI (drag payload)
        await db.remarkItems.add({ 
            ...data, 
            id, 
            sortOrder: 0, 
            syncStatus: 'pending', 
            updatedAt: new Date().toISOString() 
        });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'remarkItem', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async removeRemarkItem(id: string) {
        if (this.mode === 'cloud') {
            await apiService.removeRemarkItem(id);
            await db.remarkItems.delete(id);
            return;
        }
        await db.remarkItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remarkItem', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Impression (KG) Methods ---
    async getImpressionGraph(centerId: string = '', kgName: string = 'default') {
        if (this.mode === 'cloud') {
            console.log(`📡 [Cloud Mode] Fetching graph '${kgName}' from API...`);
            return await apiService.getImpressionGraph(centerId, kgName);
        }
        const nodes = await db.impressionNodes.where('kgName').equals(kgName).toArray();
        const links = await db.impressionLinks.where('kgName').equals(kgName).toArray();
        return {
            nodes,
            edges: links.map(l => ({ ...l, sourceId: l.sourceId, targetId: l.targetId }))
        };
    },
    async createImpressionNode(data: { title: string; content: string; nodeType: string; kgName: string; mediaId?: string }) {
        if (this.mode === 'cloud') {
            const res = await apiService.createImpressionNode(data);
            await db.impressionNodes.put({ ...res, kgName: data.kgName, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        const id = crypto.randomUUID();
        const newNode = { ...data, id, syncStatus: 'pending' as const, updatedAt: new Date().toISOString() };
        await db.impressionNodes.add(newNode);
        await db.sync_queue.add({ action: 'CREATE', entityType: 'impressionNode', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return newNode;
    },
    async updateImpressionNode(id: string, data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.updateImpressionNode(id, data);
            await db.impressionNodes.update(id, { ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        await db.impressionNodes.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'impressionNode', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteImpressionNode(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteImpressionNode(id);
            await db.impressionNodes.delete(id);
            return;
        }
        await db.impressionNodes.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'impressionNode', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async createImpressionLink(data: { sourceId: string; targetId: string; label: string; kgName: string }) {
        if (this.mode === 'cloud') {
            const res = await apiService.createImpressionLink(data);
            await db.impressionLinks.put({ ...res, kgName: data.kgName, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        const id = crypto.randomUUID();
        const newLink = { ...data, id, syncStatus: 'pending' as const, updatedAt: new Date().toISOString() };
        await db.impressionLinks.add(newLink);
        await db.sync_queue.add({ action: 'CREATE', entityType: 'impressionLink', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return newLink;
    },
    async deleteImpressionLink(id: string) {
        if (this.mode === 'cloud') {
            await apiService.deleteImpressionLink(id);
            await db.impressionLinks.delete(id);
            return;
        }
        await db.impressionLinks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'impressionLink', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async updateImpressionLink(id: string, data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.updateImpressionLink(id, data);
            await db.impressionLinks.update(id, { ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        await db.impressionLinks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'impressionLink', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Storehouse (Clip) Methods ---
    async getStorehouseItems(params: any = {}) {
        if (this.mode === 'cloud') {
            return await apiService.getStorehouseItems(params);
        }
        let query = db.storehouseItems.toCollection();
        if (params.platform) query = db.storehouseItems.where('source').equals(params.platform);
        if (params.q) {
             const words = params.q.toLowerCase().split(' ');
             query = query.filter(it => words.every((w: string) => (it.title || '').toLowerCase().includes(w) || (it.caption || '').toLowerCase().includes(w)));
        }
        return await query.reverse().limit(params.limit || 20).toArray();
    },
    async updateStorehouseItem(id: string, data: any) {
        if (this.mode === 'cloud') {
            const res = await apiService.updateStorehouseItem(id, data);
            await db.storehouseItems.update(id, { ...res, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        await db.storehouseItems.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'storehouseItem', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Bulletin (News) Methods ---
    async getBulletin() {
        if (this.mode === 'cloud') {
            return await apiService.getBulletin();
        }
        const locally = await db.bulletin.toArray();
        return locally.length > 0 ? locally[0] : { message: 'Offline mode: No bulletin cached.' };
    },
    async updateBulletin(message: string, adminEmail?: string, deviceId?: string) {
        if (this.mode === 'cloud') {
            const res = await apiService.updateBulletin(message, adminEmail, deviceId);
            await db.bulletin.clear();
            await db.bulletin.add({ id: 'main', message, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return res;
        }
        await db.bulletin.put({ id: 'main', message, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bulletin', entityId: 'main', data: { message, adminEmail, deviceId }, timestamp: Date.now() });
        this.requestSync();
    },

    // --- AI Cache Methods ---
    async getAICache(query: string) {
        const entry = await db.ai_cache.get(query);
        if (entry && entry.expiresAt > Date.now()) {
            return entry.content;
        }
        return null;
    },
    async setAICache(query: string, content: string, ttlHours: number = 3) {
        await db.ai_cache.put({
            query,
            content,
            expiresAt: Date.now() + (ttlHours * 60 * 60 * 1000)
        });
    },

    async handleIdTransition(table: any, oldId: string, res: any) {
        console.log(`[Sync Debug] Transitioning ID for ${table.name}. Response:`, res);
        
        // 嘗試從各種可能的欄位中提取 ID (支援大小寫與不同格式)
        const newIdRaw = res.id || res.ID || res._id || (res.data && (res.data.id || res.data._id));
        if (!newIdRaw) {
            console.warn(`[Sync Warning] No new ID found in response for ${table.name}.`);
            await table.update(oldId, { syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }

        const newId = String(newIdRaw).toLowerCase();
        const oldIdNormalized = String(oldId).toLowerCase();
        
        if (newId === oldIdNormalized) {
            await table.update(oldId, { syncStatus: 'synced', updatedAt: new Date().toISOString() });
            return;
        }

        const oldRecord = await table.get(oldId);
        
        // --- 核心修復：使用原子事務 (Atomic Transaction) ---
        // 確保「刪除舊 ID」與「寫入新 ID」在資料庫中是同一個瞬間完成
        // 防止 LiveQuery 在中間狀態時回傳「記錄已消失」的結果
        await db.transaction('rw', [table, db.sync_queue], async () => {
            // 1. 更新同步隊列中的待處理動作 (防止後續動作指向已失效的舊 ID)
            const affectedActions = await db.sync_queue.where('entityId').equals(oldId).modify({ entityId: newId });
            if (affectedActions > 0) {
                console.log(`[Sync Debug] Updated ${affectedActions} pending actions in queue from ${oldId} to ${newId}`);
            }

            // 2. 轉換資料庫記錄
            if (oldRecord) {
                await table.delete(oldId);
                await table.put({
                    ...oldRecord,
                    ...res,
                    id: newId,
                    syncStatus: 'synced',
                    updatedAt: new Date().toISOString()
                });
                console.log(`[Sync Debug] ${table.name} ID transitioned: ${oldId} -> ${newId}`);
            } else {
                console.log(`[Sync Info] Old record ${oldId} not found locally, transition skipped but queue checked.`);
            }
        });
    },

    syncTimer: null as any,
    isProcessing: false,

    async processQueue() {
        if (this.isProcessing) return;
        
        const actions = await db.sync_queue.toArray();
        if (actions.length === 0) return;

        this.isProcessing = true;
        console.log(`🔄 EverSync: Processing ${actions.length} pending actions...`);

        try {
            for (const action of actions) {
                try {
                    if (action.entityType === 'snippet') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.createSnippet(action.data);
                            await this.handleIdTransition(db.snippets, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateSnippet(action.entityId, action.data);
                            await db.snippets.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteSnippet(action.entityId);
                        }
                    } else if (action.entityType === 'bookmark') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.addBookmark(action.data);
                            await this.handleIdTransition(db.bookmarks, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateBookmark(action.entityId, action.data);
                            await db.bookmarks.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteBookmark(action.entityId);
                        }
                    } else if (action.entityType === 'shelf') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.createShelf(action.data);
                            await this.handleIdTransition(db.shelves, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateShelf(action.entityId, action.data);
                            await db.shelves.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteShelf(action.entityId);
                        }
                    } else if (action.entityType === 'deskItem') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.addDeskItem(action.data);
                            await this.handleIdTransition(db.deskItems, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateDeskItem(action.entityId, action.data);
                            await db.deskItems.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteDeskItem(action.entityId);
                        }
                    } else if (action.entityType === 'book') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.addBookToBookcase(action.data);
                            await this.handleIdTransition(db.bookcase, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            if (action.data.folder !== undefined) {
                                await apiService.updateBookFolder(action.entityId, action.data.folder);
                            }
                            if (action.data.sortOrder !== undefined) {
                                await apiService.updateBookSortOrder(action.entityId, action.data.sortOrder);
                            }
                            await db.bookcase.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.removeBook(action.entityId);
                        }
                    } else if (action.entityType === 'bookNote') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.addBookNote(action.data.bookId, action.data);
                            await this.handleIdTransition(db.bookNotes, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateBookNote(action.entityId, action.data);
                            await db.bookNotes.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.removeBookNote(action.entityId);
                        }
                    } else if (action.entityType === 'remark') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.createRemark(action.data);
                            await this.handleIdTransition(db.remarks, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateRemark(action.entityId, action.data);
                            await db.remarks.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteRemark(action.entityId);
                        }
                    } else if (action.entityType === 'remarkItem') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.addRemarkItem(action.data);
                            await this.handleIdTransition(db.remarkItems, action.entityId, res);
                        } else if (action.action === 'DELETE') {
                            await apiService.removeRemarkItem(action.entityId);
                        }
                    } else if (action.entityType === 'impressionNode') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.createImpressionNode(action.data);
                            await this.handleIdTransition(db.impressionNodes, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateImpressionNode(action.entityId, action.data);
                            await db.impressionNodes.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteImpressionNode(action.entityId);
                        }
                    } else if (action.entityType === 'impressionLink') {
                        if (action.action === 'CREATE') {
                            const res = await apiService.createImpressionLink(action.data);
                            await this.handleIdTransition(db.impressionLinks, action.entityId, res);
                        } else if (action.action === 'UPDATE') {
                            await apiService.updateImpressionLink(action.entityId, action.data);
                            await db.impressionLinks.update(action.entityId, { syncStatus: 'synced' });
                        } else if (action.action === 'DELETE') {
                            await apiService.deleteImpressionLink(action.entityId);
                        }
                    } else if (action.entityType === 'storehouseItem') {
                        if (action.action === 'UPDATE') {
                            await apiService.updateStorehouseItem(action.entityId, action.data);
                            await db.storehouseItems.update(action.entityId, { syncStatus: 'synced' });
                        }
                    } else if (action.entityType === 'bulletin') {
                        if (action.action === 'UPDATE') {
                            await apiService.updateBulletin(action.data.message, action.data.adminEmail, action.data.deviceId);
                            await db.bulletin.update(action.entityId, { syncStatus: 'synced' });
                        }
                    }
                    
                    if (action.id) await db.sync_queue.delete(action.id);
                    console.log(`🗑️ Sync queue cleared for action: ${action.entityType} ${action.action}`);
                } catch (err: any) {
                    console.error(`❌ Sync failed for ${action.entityType}:`, err.response?.data || err.message || err);
                    break; 
                }
            }
        } finally {
            this.isProcessing = false;
        }
    },

    requestSync() {
        console.log('💳 Credit Card Sync: Change recorded, will sync in 10s...');
        if (this.syncTimer) clearTimeout(this.syncTimer);
        this.syncTimer = setTimeout(() => {
            this.processQueue();
        }, 10000); 
    },

    async syncNow() {
        console.log('🚀 Settle Balance Now!');
        if (this.syncTimer) clearTimeout(this.syncTimer);
        await this.processQueue();
    },

    async purgeDatabase() {
        if (!confirm('🚨 警告：這將清除所有本地快取並重新與伺服器同步。尚未同步的更改將會遺失。確定要執行嗎？')) return;
        try {
            await db.delete();
            localStorage.clear();
            window.location.reload();
        } catch (err) {
            console.error('Purge failed:', err);
            alert('清除失敗，請嘗試手動清理瀏覽器快取。');
        }
    }
});

// Auto-sync when network comes online
window.addEventListener('online', () => {
    console.log('🌐 Network online! Triggering EverSync...');
    syncService.processQueue();
});
