import { ref, onMounted, onUnmounted } from 'vue';
import { liveQuery } from 'dexie';
import { db } from '../services/localDb';

export function useSyncStatus() {
    const pendingCount = ref(0);
    let subscription: any = null;

    onMounted(() => {
        subscription = liveQuery(() => db.sync_queue.count()).subscribe(count => {
            pendingCount.value = count;
        });
    });

    onUnmounted(() => {
        if (subscription) subscription.unsubscribe();
    });

    return {
        pendingCount
    };
}
