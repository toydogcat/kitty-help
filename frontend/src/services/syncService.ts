import { apiService } from './api';
import { db, type LocalSnippet } from './localDb';

export const syncService = {
    // Snippets Methods (EverSync Enhanced)
    async getSnippets(parentId: string | null = null, all: boolean = false) {
        // 1. Try to get from local first for instant UI
        const pid = parentId || 'root';
        const localItems = await db.snippets
            .where('parentId')
            .equals(pid)
            .sortBy('sortOrder');

        // 2. Refresh from API in background (Non-blocking)
        this.refreshSnippets(parentId, all).catch(err => {
            console.warn('Network refresh failed (Offline mode active):', err.message);
        });

        return localItems;
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
        await db.sync_queue.add({ action: 'CREATE', entityType: 'snippet', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return newSnippet;
    },

    async updateSnippet(id: string, data: any) {
        await db.snippets.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'snippet', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },

    async deleteSnippet(id: string) {
        await db.snippets.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'snippet', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },

    // --- Bookmarks Methods ---
    async getBookmarks(parentId?: string | 'root', all: boolean = false) {
        const pid = parentId || 'root';
        const localItems = await db.bookmarks.where('parentId').equals(pid).sortBy('sortOrder');
        this.refreshBookmarks(parentId, all).catch(() => {});
        return localItems;
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
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookmark', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return newBookmark;
    },

    async updateBookmark(id: string, data: any) {
        await db.bookmarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookmark', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },

    async deleteBookmark(id: string) {
        await db.bookmarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookmark', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },

    // --- Desk Methods ---
    async getShelves() {
        const local = await db.shelves.orderBy('sortOrder').toArray();
        this.refreshShelves().catch(() => {});
        return local;
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
        await db.sync_queue.add({ action: 'CREATE', entityType: 'shelf', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return { id };
    },
    async updateShelf(id: string, data: any) {
        await db.shelves.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'shelf', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },
    async deleteShelf(id: string) {
        await db.shelves.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'shelf', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },
    async getDeskItems(shelfId?: string) {
        const sid = shelfId || 'null';
        const local = await db.deskItems.where('shelfId').equals(sid).sortBy('sortOrder');
        this.refreshDeskItems(sid).catch(() => {});
        return local;
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
        await db.sync_queue.add({ action: 'CREATE', entityType: 'deskItem', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return { id };
    },
    async updateDeskItem(id: string, data: any) {
        await db.deskItems.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'deskItem', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },
    async deleteDeskItem(id: string) {
        await db.deskItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'deskItem', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },

    // --- Bookcase Methods ---
    async getBookcase() {
        const local = await db.bookcase.toArray();
        this.refreshBookcase().catch(() => {});
        return local;
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
        await db.bookcase.add({ ...data, id, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'book', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },
    async updateBookFolder(id: string, folder: string) {
        await db.bookcase.update(id, { folder, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'book', entityId: id, data: { folder }, timestamp: Date.now() });
        this.processQueue();
    },
    async removeBook(id: string) {
        await db.bookcase.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'book', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },
    async getBookNotes(bookId: string) {
        const local = await db.bookNotes.where('bookId').equals(bookId).toArray();
        this.refreshBookNotes(bookId).catch(() => {});
        return local;
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
        await db.sync_queue.add({ action: 'CREATE', entityType: 'bookNote', entityId: id, data: { ...data, bookId }, timestamp: Date.now() });
        this.processQueue();
        return { id };
    },
    async updateBookNote(id: string, data: any) {
        await db.bookNotes.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'bookNote', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },
    async removeBookNote(id: string) {
        await db.bookNotes.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'bookNote', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },

    // --- Remarks (Obs) Methods ---
    async getRemarks() {
        const local = await db.remarks.toArray();
        this.refreshRemarks().catch(() => {});
        return local;
    },
    async refreshRemarks() {
        const remote = await apiService.getRemarks();
        await db.transaction('rw', [db.remarks, db.remarkItems], async () => {
             for (const c of remote.containers || []) {
                 await db.remarks.put({ ...c, syncStatus: 'synced', updatedAt: new Date().toISOString() });
             }
             for (const it of remote.items || []) {
                 await db.remarkItems.put({ ...it, syncStatus: 'synced', updatedAt: new Date().toISOString() });
             }
        });
        return remote;
    },
    async createRemark(data: any) {
        const id = crypto.randomUUID();
        await db.remarks.add({ ...data, id, isPinned: false, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'remark', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return { id };
    },
    async updateRemark(id: string, data: any) {
        await db.remarks.update(id, { ...data, syncStatus: 'pending' });
        await db.sync_queue.add({ action: 'UPDATE', entityType: 'remark', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
    },
    async deleteRemark(id: string) {
        await db.remarks.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remark', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
    },
    async addRemarkItem(data: any) {
        const id = crypto.randomUUID();
        await db.remarkItems.add({ ...data, id, sortOrder: 0, syncStatus: 'pending', updatedAt: new Date().toISOString() });
        await db.sync_queue.add({ action: 'CREATE', entityType: 'remarkItem', entityId: id, data, timestamp: Date.now() });
        this.processQueue();
        return { id };
    },
    async removeRemarkItem(id: string) {
        await db.remarkItems.delete(id);
        await db.sync_queue.add({ action: 'DELETE', entityType: 'remarkItem', entityId: id, data: null, timestamp: Date.now() });
        this.processQueue();
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

    async processQueue() {
        const actions = await db.sync_queue.toArray();
        if (actions.length === 0) return;

        for (const action of actions) {
            try {
                if (action.entityType === 'snippet') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.createSnippet(action.data);
                        await db.snippets.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateSnippet(action.entityId, action.data);
                        await db.snippets.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.deleteSnippet(action.entityId);
                    }
                } else if (action.entityType === 'bookmark') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.addBookmark(action.data);
                        await db.bookmarks.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateBookmark(action.entityId, action.data);
                        await db.bookmarks.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.deleteBookmark(action.entityId);
                    }
                } else if (action.entityType === 'shelf') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.createShelf(action.data);
                        await db.shelves.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateShelf(action.entityId, action.data);
                        await db.shelves.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.deleteShelf(action.entityId);
                    }
                } else if (action.entityType === 'deskItem') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.addDeskItem(action.data);
                        await db.deskItems.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateDeskItem(action.entityId, action.data);
                        await db.deskItems.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.deleteDeskItem(action.entityId);
                    }
                } else if (action.entityType === 'book') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.addBookToBookcase(action.data);
                        await db.bookcase.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateBookFolder(action.entityId, action.data.folder);
                        await db.bookcase.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.removeBook(action.entityId);
                    }
                } else if (action.entityType === 'bookNote') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.addBookNote(action.data.bookId, action.data);
                        await db.bookNotes.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateBookNote(action.entityId, action.data);
                        await db.bookNotes.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.removeBookNote(action.entityId);
                    }
                } else if (action.entityType === 'remark') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.createRemark(action.data);
                        await db.remarks.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'UPDATE') {
                        await apiService.updateRemark(action.entityId, action.data);
                        await db.remarks.update(action.entityId, { syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.deleteRemark(action.entityId);
                    }
                } else if (action.entityType === 'remarkItem') {
                    if (action.action === 'CREATE') {
                        const res = await apiService.addRemarkItem(action.data);
                        await db.remarkItems.update(action.entityId, { id: res.id, syncStatus: 'synced' });
                    } else if (action.action === 'DELETE') {
                        await apiService.removeRemarkItem(action.entityId);
                    }
                }
                
                if (action.id) await db.sync_queue.delete(action.id);
            } catch (err) {
                console.error(`❌ Sync failed:`, err);
                break; 
            }
        }
    }
};

// Auto-sync when network comes online
window.addEventListener('online', () => {
    console.log('🌐 Network online! Triggering EverSync...');
    syncService.processQueue();
});
