import { initializeApp } from "firebase/app";
import { getAuth, signInAnonymously, GoogleAuthProvider, signInWithPopup, signOut, signInWithRedirect, getRedirectResult } from "firebase/auth";
import { getFirestore } from "firebase/firestore";
import { getStorage } from "firebase/storage";
import { getAnalytics } from "firebase/analytics";

const firebaseConfig = {
  apiKey: "AIzaSyB0lzNusHGHN3q5FBqM6M2PFcDdHJjmJt4",
  authDomain: "ai-factory-tarot.firebaseapp.com",
  projectId: "ai-factory-tarot",
  storageBucket: "ai-factory-tarot.firebasestorage.app",
  messagingSenderId: "869997180055",
  appId: "1:869997180055:web:5b2827a85e0ff4c80725d2",
  measurementId: "G-S3M30CXRLV"
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
