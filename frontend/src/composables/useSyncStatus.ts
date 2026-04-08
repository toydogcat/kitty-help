import { ref, onMounted, onUnmounted } from 'vue';
import { db } from '../services/localDb';

export function useSyncStatus() {
    const pendingCount = ref(0);
    let interval: any = null;

    const updateCount = async () => {
        pendingCount.value = await db.sync_queue.count();
    };

    onMounted(() => {
        updateCount();
        interval = setInterval(updateCount, 3000); // Check every 3 seconds
    });

    onUnmounted(() => {
        if (interval) clearInterval(interval);
    });

    return {
        pendingCount
    };
}
