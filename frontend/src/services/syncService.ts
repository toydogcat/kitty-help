import { apiService } from './api';
import { db, type LocalSnippet } from './localDb';
import { reactive } from 'vue';

export const syncService = reactive({
    // Snippets Methods (EverSync Enhanced)
    async getSnippets(parentId: string | null = null) {
        const pid = parentId || 'root';
        return await db.snippets
            .where('parentId')
            .equals(pid)
            .sortBy('sortOrder');
    },

    async refreshSnippets(parentId: string | null = null, all: boolean = false) {
        try {
            const remoteItems = await apiService.getSnippets(parentId, all);
            await db.transaction('rw', db.snippets, async () => {
                for (const item of remoteItems) {
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
        await db.snippets.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'snippet', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },

    async deleteSnippet(id: string) {
        await db.snippets.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'snippet', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Bookmarks Methods ---
    async getBookmarks(parentId?: string | 'root') {
        const pid = parentId || 'root';
        return await db.bookmarks.where('parentId').equals(pid).sortBy('sortOrder');
    },

    async refreshBookmarks(parentId?: string | 'root', all: boolean = false) {
        const remoteItems = await apiService.getBookmarks(parentId, all);
        await db.transaction('rw', db.bookmarks, async () => {
            for (const item of remoteItems) {
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
        const id = crypto.randomUUID();
        const newBookmark = { ...data, id, syncStatus: 'pending', updatedAt: new Date().toISOString() };
        await db.bookmarks.add(newBookmark);
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookmark', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return newBookmark;
    },

    async updateBookmark(id: string, data: any) {
        await db.bookmarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookmark', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },

    async deleteBookmark(id: string) {
        await db.bookmarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookmark', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Desk Methods ---
    async getShelves() {
        return await db.shelves.orderBy('sortOrder').toArray();
    },
    async refreshShelves() {
        const remote = await apiService.getShelves();
        await db.transaction('rw', db.shelves, async () => {
            for (const s of remote) {
                await db.shelves.put({ ...s, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async createShelf(data: any) {
        const id = crypto.randomUUID();
        await db.shelves.add({ ...data, id, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'shelf', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateShelf(id: string, data: any) {
        await db.shelves.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'shelf', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteShelf(id: string) {
        await db.shelves.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'shelf', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async getDeskItems(shelfId?: string) {
        const sid = shelfId || 'null';
        return await db.deskItems.where('shelfId').equals(sid).sortBy('sortOrder');
    },
    async refreshDeskItems(shelfId: string = 'null') {
        const remote = await apiService.getDeskItems(shelfId === 'null' ? undefined : shelfId);
        await db.transaction('rw', db.deskItems, async () => {
            for (const item of remote) {
                await db.deskItems.put({ ...item, shelfId: item.shelfId || 'null', syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async addDeskItem(data: any) {
        const id = crypto.randomUUID();
        await db.deskItems.add({ ...data, id, shelfId: data.shelfId || 'null', syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'deskItem', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateDeskItem(id: string, data: any) {
        await db.deskItems.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'deskItem', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteDeskItem(id: string) {
        await db.deskItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'deskItem', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Bookcase Methods ---
    async getBookcase() {
        return await db.bookcase.toArray();
    },
    async refreshBookcase() {
        const remote = await apiService.getBookcase();
        await db.transaction('rw', db.bookcase, async () => {
            for (const b of remote) {
                await db.bookcase.put({ ...b, syncStatus: 'synced', updatedAt: new Date().toISOString() });
            }
        });
        return remote;
    },
    async addBookToBookcase(data: any) {
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
        await db.bookcase.update(id, { folder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'book', entityId: id, data: { folder }, timestamp: Date.now() });
        this.requestSync();
    },
    async moveBook(id: string, sortOrder: number) {
        await db.bookcase.update(id, { sortOrder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'book', entityId: id, data: { sortOrder }, timestamp: Date.now() });
        this.requestSync();
    },
    async removeBook(id: string) {
        await db.bookcase.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'book', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async getBookNotes(bookId: string) {
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
        const id = crypto.randomUUID();
        await db.bookNotes.add({ ...data, id, bookId, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookNote', entityId: id, data: { ...data, id, bookId }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateBookNote(id: string, data: any) {
        await db.bookNotes.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookNote', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async removeBookNote(id: string) {
        await db.bookNotes.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookNote', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },

    // --- Remarks (Obs) Methods ---
    async getRemarks() {
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
        const id = crypto.randomUUID();
        await db.remarks.add({ ...data, id, isPinned: false, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'remark', entityId: id, data: { ...data, id }, timestamp: Date.now() });
        this.requestSync();
        return { id };
    },
    async updateRemark(id: string, data: any) {
        await db.remarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'remark', entityId: id, data, timestamp: Date.now() });
        this.requestSync();
    },
    async deleteRemark(id: string) {
        await db.remarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remark', entityId: id, data: null, timestamp: Date.now() });
        this.requestSync();
    },
    async addRemarkItem(data: { containerId: string, logId: string, log?: any }) {
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
        await db.remarkItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remarkItem', entityId: id, data: null, timestamp: Date.now() });
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
