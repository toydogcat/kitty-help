import { createRouter, createWebHistory } from 'vue-router';
const HomeView = () => import('../views/HomeView.vue');
const PersonalView = () => import('../views/PersonalView.vue');
const AdminView = () => import('../views/AdminView.vue');
const StorehouseView = () => import('../views/StorehouseView.vue');
const ChatView = () => import('../views/ChatView.vue');
const ImpressionView = () => import('../views/ImpressionView.vue');
const DeskView = () => import('../views/DeskView.vue');
const ObsidianView = () => import('../views/ObsidianView.vue');

const routes = [
  { 
    path: '/', name: 'home', component: HomeView, 
    meta: { requiredRoles: ['user', 'vip', 'admin', 'superadmin'] } 
  },
  { 
    path: '/chat', name: 'chat', component: ChatView, 
    meta: { requiredRoles: ['user', 'vip', 'admin', 'superadmin'] } 
  },
  { 
    path: '/personal', name: 'personal', component: PersonalView, 
    meta: { requiredRoles: ['vip', 'admin', 'superadmin'] } 
  },
  { 
    path: '/desk', name: 'desk', component: DeskView, 
    meta: { requiredRoles: ['vip', 'admin', 'superadmin'] } 
  },
  { 
    path: '/impression', name: 'impression', component: ImpressionView, 
    meta: { requiredRoles: ['vip', 'admin', 'superadmin'] } 
  },
  { 
    path: '/storehouse', name: 'storehouse', component: StorehouseView, 
    meta: { requiredRoles: ['admin', 'superadmin'] } 
  },
  { 
    path: '/admin', name: 'admin', component: AdminView, 
    meta: { requiredRoles: ['admin', 'superadmin'] } 
  },
  { 
    path: '/obsidian', name: 'obsidian', component: ObsidianView, 
    meta: { requiredRoles: ['admin', 'superadmin'] } 
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Navigation Guard (Police Checkpoint)
router.beforeEach((to, _, next) => {
  const userRole = localStorage.getItem('kitty_user_role') || 'visitor';
  const requiredRoles = to.meta.requiredRoles as string[];

  if (!requiredRoles) return next();

  if (requiredRoles.includes(userRole)) {
    next();
  } else {
    if (to.path !== '/') {
      next('/');
    } else {
      next(); 
    }
  }
});

export default router;
