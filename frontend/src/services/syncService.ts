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
                await db.bookmarks.put({ ...item, syncStatus: 'synced', updatedAt: new Date().toISOString() });
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
