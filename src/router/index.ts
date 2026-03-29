import { createRouter, createWebHistory } from 'vue-router';
import HomeView from '../views/HomeView.vue';
import PersonalView from '../views/PersonalView.vue';
import AdminView from '../views/AdminView.vue';

const routes = [
  { path: '/', name: 'home', component: HomeView },
  { path: '/personal', name: 'personal', component: PersonalView },
  { path: '/admin', name: 'admin', component: AdminView },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
