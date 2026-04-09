import { ref } from 'vue';
import { syncService } from '../services/syncService';

export function usePin() {
    const isPinning = ref(false);

    /**
     * Pins any resource to the "Desk" (Dashboard)
     * @param type 'remark' | 'snippet' | 'media' | 'bookmark'
     * @param refId The original ID of the resource
     * @param shelfId Optional target shelf, null for Main Desktop
     */
    const pinToDesk = async (type: string, refId: string, shelfId: string | null = null) => {
        isPinning.value = true;
        try {
            await syncService.addDeskItem({ type, refId, shelfId });
            return true;
        } catch (err) {
            console.error(`[PinService] Failed to pin ${type}:${refId}`, err);
            throw err;
        } finally {
            isPinning.value = false;
        }
    };

    const unpinFromDesk = async (deskItemId: string) => {
        try {
            await syncService.deleteDeskItem(deskItemId);
            return true;
        } catch (err) {
            console.error(`[PinService] Failed to unpin item ${deskItemId}`, err);
            throw err;
        }
    };

    const pinUniverseToDesk = async (kgName: string) => {
        isPinning.value = true;
        try {
            // 1. Create Internal Bookmark via EverSync
            const res = await syncService.addBookmark({
                title: `Universe: ${kgName}`,
                url: `/impression?kg=${kgName}`,
                category: 'Impression',
                iconUrl: '',
                parentId: 'root',
                isFolder: false,
                sortOrder: 0
            });
            const bId = res.id;

            // 2. Add as Desk Item via EverSync
            await syncService.addDeskItem({
                type: 'bookmark',
                refId: bId,
                shelfId: null,
                sortOrder: 0
            });
            return true;
        } catch (err) {
            console.error(`[PinService] Failed to pin Universe ${kgName}`, err);
            throw err;
        } finally {
            isPinning.value = false;
        }
    };

    /**
     * Toggles the "pinned" status of a Remark (the sidebar pin, not Desk)
     */
    const toggleRemarkSidebarPin = async (remarkId: string, currentStatus: boolean) => {
        try {
            await syncService.updateRemark(remarkId, { isPinned: !currentStatus });
            return !currentStatus;
        } catch (err) {
            console.error(`[PinService] Failed to toggle remark pin ${remarkId}`, err);
            throw err;
        }
    };

    return {
        isPinning,
        pinToDesk,
        unpinFromDesk,
        pinUniverseToDesk,
        toggleRemarkSidebarPin
    };
}
