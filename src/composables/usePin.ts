import { ref } from 'vue';
import { apiService } from '../services/api';

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
            await apiService.addDeskItem({ type, refId, shelfId });
            // We don't alert here to keep it abstract, allow caller to handle feedback
            return true;
        } catch (err) {
            console.error(`[PinService] Failed to pin ${type}:${refId}`, err);
            throw err;
        } finally {
            isPinning.value = false;
        }
    };

    /**
     * Unpins (deletes) an item from the Desk
     * @param deskItemId The ID of the entry in desk_items table
     */
    const unpinFromDesk = async (deskItemId: string) => {
        try {
            await apiService.deleteDeskItem(deskItemId);
            return true;
        } catch (err) {
            console.error(`[PinService] Failed to unpin item ${deskItemId}`, err);
            throw err;
        }
    };

    /**
     * Special helper for pinning a Knowledge Universe (Graph) to Desk.
     * It handles the bookmark creation automatically.
     */
    const pinUniverseToDesk = async (kgName: string) => {
        isPinning.value = true;
        try {
            // 1. Create Internal Bookmark
            const bookmark = await apiService.addBookmark({
                title: `Universe: ${kgName}`,
                url: `/impression?kg=${kgName}`,
                category: 'Impression'
            });

            // 2. Add as Desk Item
            await apiService.addDeskItem({
                type: 'bookmark',
                refId: bookmark.id,
                shelfId: null
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
            await apiService.updateRemark(remarkId, { isPinned: !currentStatus });
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
