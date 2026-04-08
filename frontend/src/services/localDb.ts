import Dexie, { type Table } from 'dexie';

export interface LocalSnippet {
    id: string; // Using same UUID as backend
    parentId: string | null;
    name: string;
    content: string;
    isFolder: boolean;
    sortOrder: number;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalBookmark {
    id: string;
    title: string;
    url: string | null;
    category: string;
    iconUrl: string;
    parentId: string | null;
    isFolder: boolean;
    sortOrder: number;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface SyncAction {
    id?: number;
    action: 'CREATE' | 'UPDATE' | 'DELETE';
    entityType: 'snippet' | 'bookmark' | 'remark';
    entityId: string;
    data: any;
    timestamp: number;
}

export class EverSyncDatabase extends Dexie {
    snippets!: Table<LocalSnippet>;
    bookmarks!: Table<LocalBookmark>;
    sync_queue!: Table<SyncAction>;

    constructor() {
        super('EverSyncDB');
        this.version(1).stores({
            snippets: 'id, parentId, name, syncStatus',
            bookmarks: 'id, parentId, title, url, syncStatus',
            sync_queue: '++id, action, entityId, timestamp'
        });
    }
}

export const db = new EverSyncDatabase();
