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

export interface LocalShelf {
    id: string;
    name: string;
    color: string;
    sortOrder: number;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalDeskItem {
    id: string;
    type: string;
    refId: string;
    shelfId: string | null;
    sortOrder: number;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalBook {
    id: string;
    storeId: string;
    title: string;
    category: string;
    folder: string;
    notes?: string;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalBookNote {
    id: string;
    bookId: string;
    title: string;
    content: string;
    noteType: string;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalRemark {
    id: string;
    name: string;
    content: string;
    isPinned: boolean;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalRemarkItem {
    id: string;
    containerId: string;
    logId: string;
    sortOrder: number;
    updatedAt: string;
    syncStatus: 'synced' | 'pending' | 'error';
}

export interface LocalAICache {
    query: string;
    content: string;
    expiresAt: number;
}

export interface SyncAction {
    id?: number;
    action: 'CREATE' | 'UPDATE' | 'DELETE' | 'MOVE';
    entityType: 'snippet' | 'bookmark' | 'remark' | 'remarkItem' | 'shelf' | 'deskItem' | 'book' | 'bookNote';
    entityId: string;
    data: any;
    timestamp: number;
}

export class EverSyncDatabase extends Dexie {
    snippets!: Table<LocalSnippet>;
    bookmarks!: Table<LocalBookmark>;
    shelves!: Table<LocalShelf>;
    deskItems!: Table<LocalDeskItem>;
    bookcase!: Table<LocalBook>;
    bookNotes!: Table<LocalBookNote>;
    remarks!: Table<LocalRemark>;
    remarkItems!: Table<LocalRemarkItem>;
    ai_cache!: Table<LocalAICache>;
    sync_queue!: Table<SyncAction>;

    constructor() {
        super('EverSyncDB');
        this.version(4).stores({
            snippets: 'id, parentId, name, syncStatus',
            bookmarks: 'id, parentId, title, url, syncStatus',
            shelves: 'id, name, sortOrder, syncStatus',
            deskItems: 'id, refId, shelfId, sortOrder, syncStatus',
            bookcase: 'id, storeId, title, folder, syncStatus',
            bookNotes: 'id, bookId, title, syncStatus',
            remarks: 'id, name, isPinned, syncStatus',
            remarkItems: 'id, containerId, logId, syncStatus',
            ai_cache: 'query, expiresAt',
            sync_queue: '++id, action, entityId, timestamp'
        });
    }
}

export const db = new EverSyncDatabase();
