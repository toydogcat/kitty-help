import { ref, onUnmounted } from 'vue';
import { auth } from '../firebaseConfig';
import { onAuthStateChanged } from 'firebase/auth';
import type { User } from 'firebase/auth';

export function useAuth() {
  const user = ref<User | null>(auth.currentUser);
  const loading = ref(true);

  const unsubscribe = onAuthStateChanged(auth, (firebaseUser) => {
    user.value = firebaseUser;
    loading.value = false;
  });

  onUnmounted(() => {
    if (unsubscribe) unsubscribe();
  });

  return {
    user,
    loading,
    isAuthenticated: () => !!user.value
  };
}
