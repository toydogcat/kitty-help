import { initializeApp } from "firebase/app";
import { getAuth, signInAnonymously, GoogleAuthProvider, signInWithPopup, signOut, signInWithRedirect, getRedirectResult } from "firebase/auth";
import { getFirestore } from "firebase/firestore";
import { getStorage } from "firebase/storage";
import { getAnalytics } from "firebase/analytics";

const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY || "AIzaSyB0lzNusHGHN3q5FBqM6M2PFcDdHJjmJt4",
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || "ai-factory-tarot.firebaseapp.com",
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || "ai-factory-tarot",
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET || "ai-factory-tarot.firebasestorage.app",
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID || "869997180055",
  appId: import.meta.env.VITE_FIREBASE_APP_ID || "1:869997180055:web:5b2827a85e0ff4c80725d2",
  measurementId: import.meta.env.VITE_FIREBASE_MEASUREMENT_ID || "G-S3M30CXRLV"
};

// Initialize Firebase
const app = initializeApp(firebaseConfig);
const auth = getAuth(app);
const db = getFirestore(app);
const storage = getStorage(app);
const analytics = typeof window !== 'undefined' ? getAnalytics(app) : null;
const googleProvider = new GoogleAuthProvider();
googleProvider.setCustomParameters({ prompt: 'select_account' });

export { app, auth, db, storage, analytics, signInAnonymously, googleProvider, signInWithPopup, signOut, signInWithRedirect, getRedirectResult };
