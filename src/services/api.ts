import axios from 'axios';
// @ts-ignore
import io from 'socket.io-client';

const BASE_URL = import.meta.env.VITE_API_URL || window.location.origin;
const API_BASE = BASE_URL + '/api';
const API_URL = BASE_URL;

// For production (e.g., Firebase), if API_URL and current origin differ, 
// socket must use the explicit API_URL
export const socket = io(API_URL, {
  path: '/socket.io',
  transports: ['websocket', 'polling'],
  transportOptions: {
    polling: {
      extraHeaders: {
        'cf-skip-browser-warning': 'true'
      }
    }
  }
});

// Skip Cloudflare/Ngrok browser warnings globally for Axios
axios.defaults.headers.common['cf-skip-browser-warning'] = 'true';
axios.defaults.headers.common['ngrok-skip-browser-warning'] = 'true';

// Set Global Axios Auth Header
export const setAuthToken = (token: string | null) => {
  if (token) {
    axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
    localStorage.setItem('kitty_token', token);
  } else {
    delete axios.defaults.headers.common['Authorization'];
    localStorage.removeItem('kitty_token');
  }
};

// Initialize Token from LocalStorage
const savedToken = localStorage.getItem('kitty_token');
if (savedToken) {
  setAuthToken(savedToken);
}

// Add generic response interceptor for debugging and Token Refresh
axios.interceptors.response.use(
  response => {
    // 🔄 Sliding Session: If backend sent a refreshed token, update it!
    const refreshToken = response.headers['x-refresh-token'];
    if (refreshToken) {
      console.log('🔄 [Auth] Token refreshed automatically via Sliding Session');
      setAuthToken(refreshToken);
    }
    return response;
  },
  error => {
    console.error('API Error:', {
      url: error.config?.url,
      status: error.response?.status,
      data: error.response?.data
    });
    // Optional: Log out if status is 401
    if (error.response?.status === 401) {
      // localStorage.removeItem('kitty_token');
      // window.location.reload(); 
    }
    return Promise.reject(error);
  }
);

export const apiService = {
  // Device Management
  async registerDevice(id: string, userAgent: string) {
    const response = await axios.post(`${API_BASE}/devices/register`, { id, userAgent });
    return response.data;
  },
  async getDevices() {
    const response = await axios.get(`${API_BASE}/devices`);
    return response.data;
  },
  async updateDeviceStatus(id: string, status: string, deviceName: string = 'Unknown Device', userId: string = '') {
    const response = await axios.put(`${API_BASE}/devices/status`, { id, status, deviceName, userId });
    return response.data;
  },
  async deleteDevice(id: string) {
    await axios.delete(`${API_BASE}/devices/${id}`);
  },

  // User Management
  async getUsers() {
    const response = await axios.get(`${API_BASE}/users`);
    return response.data;
  },
  async createUser(name: string, isAdmin: boolean = false) {
    const response = await axios.post(`${API_BASE}/users`, { name, role: isAdmin ? 'admin' : 'user' });
    return response.data;
  },
  async updateUserRole(userId: string, role: string, _adminEmail?: string) {
    const res = await axios.post(`${API_BASE}/users/role`, { userId, role });
    return res.data;
  },
  async deleteUser(userId: string) {
    await axios.delete(`${API_BASE}/users/${userId}`);
  },

  // Password Vault
  async getPasswords(_userId?: string) {
    const response = await axios.get(`${API_BASE}/passwords`);
    return response.data;
  },
  async addPassword(data: { siteName: string; url?: string; username?: string; passwordRaw: string; category?: string; notes?: string }) {
    const response = await axios.post(`${API_BASE}/passwords`, data);
    return response.data;
  },
  async updatePassword(id: string, data: { siteName: string; url?: string; username?: string; passwordRaw: string; category?: string; notes?: string }) {
    const response = await axios.put(`${API_BASE}/passwords/${id}`, data);
    return response.data;
  },
  async deletePassword(id: string) {
    await axios.delete(`${API_BASE}/passwords/${id}`);
  },

  // Snippets
  async getSnippets(parentId: string | null = null, all: boolean = false) {
    const response = await axios.get(`${API_BASE}/snippets`, { params: { parentId: parentId || 'root', all } });
    return response.data;
  },
  async createSnippet(data: { parentId: string | null; name: string; content?: string; isFolder: boolean; sortOrder?: number }) {
    const response = await axios.post(`${API_BASE}/snippets`, data);
    return response.data;
  },
  async updateSnippet(id: string, data: { name: string; content?: string; parentId?: string | null; sortOrder?: number }) {
    const response = await axios.put(`${API_BASE}/snippets/${id}`, data);
    return response.data;
  },
  async deleteSnippet(id: string) {
    await axios.delete(`${API_BASE}/snippets/${id}`);
  },

  // Storehouse & Media
  async getArchives(platform?: string, type?: string, query?: string) {
    const response = await axios.get(`${API_BASE}/storehouse`, {
      params: { platform, type, q: query }
    });
    return response.data;
  },
  async getStorehouseItems(params?: any) {
    // Forward params if provided
    const response = await axios.get(`${API_BASE}/storehouse`, { params });
    return response.data;
  },
  async updateStorehouseItem(id: string, data: { title: string; caption?: string; notes?: string }) {
    const response = await axios.put(`${API_BASE}/storehouse/${id}`, {
      ...data,
      caption: data.caption || ''
    });
    return response.data;
  },
  async indexStorehouseItem(id: string) {
    const response = await axios.post(`${API_BASE}/storehouse/${id}/index`);
    return response.data;
  },
  async uploadFile(file: File) {
    const formData = new FormData();
    formData.append('file', file);
    const response = await axios.post(`${API_BASE}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    return response.data;
  },
  getStorehouseFileUrl(fileID: string, platform?: string, download: boolean = false) {
    let url = `${API_BASE}/storehouse/file/${fileID}`;
    const params = new URLSearchParams();
    if (platform) {
      params.append('platform', platform);
    }
    if (download) {
      params.append('download', '1');
    }
    const query = params.toString();
    if (query) {
      url += `?${query}`;
    }
    return url;
  },
  getAbsoluteUrl(path: string) {
    if (!path) return '';
    if (path.startsWith('http')) return path;
    const cleanPath = path.startsWith('/') ? path : `/${path}`;
    return `${BASE_URL}${cleanPath}`;
  },

  // Calendar
  async getCalendarEvents() {
    const res = await axios.get(`${API_BASE}/calendar`);
    return res.data;
  },
  async updateCalendarEvent(date: string, content: string) {
    const res = await axios.post(`${API_BASE}/calendar`, { date, content });
    return res.data;
  },

  // Bulletin
  async getBulletin() {
    const response = await axios.get(`${API_BASE}/bulletin`);
    return response.data;
  },
  async updateBulletin(message: string, adminEmail?: string, deviceId?: string) {
    const response = await axios.post(`${API_BASE}/bulletin`, { message, adminEmail, deviceId });
    return response.data;
  },

  // Bookmarks
  async getBookmarks(parentId?: string | 'root', all: boolean = false) {
    const params = parentId ? { parentId, all } : { all };
    const response = await axios.get(`${API_BASE}/bookmarks`, { params });
    return response.data;
  },
  async addBookmark(data: { 
    title: string; 
    url?: string | null; 
    category?: string; 
    iconUrl?: string; 
    passwordId?: string | null;
    isFolder?: boolean;
    parentId?: string | null;
    sortOrder?: number;
  }) {
    const res = await axios.post(`${API_BASE}/bookmarks`, data);
    return res.data;
  },
  async updateBookmark(id: string, data: {
    title?: string;
    url?: string | null;
    category?: string;
    parentId?: string | null;
    sortOrder?: number;
  }) {
    const res = await axios.put(`${API_BASE}/bookmarks/${id}`, data);
    return res.data;
  },
  async deleteBookmark(id: string) {
    return axios.delete(`${API_BASE}/bookmarks/${id}`);
  },

  // Impression (Graph Knowledge Canvas)
  async getImpressionTemp() {
    const response = await axios.get(`${API_BASE}/impression/temp`);
    return response.data;
  },
  async getImpressionGraph(centerId?: string) {
    const response = await axios.get(`${API_BASE}/impression/graph`, { params: { centerId: centerId || '' } });
    return response.data;
  },
  async searchImpressionNodes(query: string) {
    const response = await axios.get(`${API_BASE}/impression/search`, { params: { q: query } });
    return response.data;
  },
  async createImpressionNode(data: { mediaId?: string; title: string; content: string; nodeType: string }) {
    const response = await axios.post(`${API_BASE}/impression/nodes`, data);
    return response.data;
  },
  async createImpressionLink(data: { sourceId: string; targetId: string; label: string }) {
    const response = await axios.post(`${API_BASE}/impression/links`, data);
    return response.data;
  },
  async updateImpressionLink(id: string, data: { label: string }) {
    const response = await axios.put(`${API_BASE}/impression/links/${id}`, data);
    return response.data;
  },
  async deleteImpressionLink(id: string) {
    await axios.delete(`${API_BASE}/impression/links/${id}`);
  },
  async deleteImpressionNode(id: string) {
    const response = await axios.delete(`${API_BASE}/impression/nodes/${id}`);
    return response.data;
  },
  async updateImpressionNode(id: string, data: { title: string; content: string; nodeType: string }) {
    const response = await axios.put(`${API_BASE}/impression/nodes/${id}`, data);
    return response.data;
  },
  async exportImpressionGraph() {
    const response = await axios.get(`${API_BASE}/impression/export`);
    return response.data;
  },
  async getImpressionRandom() {
    const response = await axios.get(`${API_BASE}/impression/random`);
    return response.data;
  },
  async syncImpressionToSnippet(id: string) {
    const response = await axios.post(`${API_BASE}/impression/nodes/${id}/sync`);
    return response.data;
  },
  async getLinkedSnippet(id: string) {
    const response = await axios.get(`${API_BASE}/impression/snippets/${id}`);
    return response.data;
  },
  async importImpressionGraph(data: any) {
    const response = await axios.post(`${API_BASE}/impression/import`, data);
    return response.data;
  },

  // Security 2FA
  async requestSecurityChallenge(_id: string, _deviceId?: string) {
    const res = await axios.post(`${API_BASE}/security/challenge`, { id: _id, deviceId: _deviceId });
    return res.data;
  },
  async getSecurityStatus(_id: string, _deviceId?: string, token?: string) {
    const res = await axios.get(`${API_BASE}/security/status`, {
      params: { id: _id, deviceId: _deviceId, token }
    });
    return res.data;
  },

  // Auth
  async verifyToken(idToken: string) {
    const response = await axios.post(`${API_BASE}/auth/verify`, { idToken });
    if (response.data.token) {
      setAuthToken(response.data.token);
    }
    return response.data;
  },

  // Bot Verification & Linking
  async linkBotAccount(token: string, deviceId: string) {
    const response = await axios.post(`${API_BASE}/bot/link`, { token, deviceId });
    return response.data;
  },

  // Document Chicken & Web Reader
  async runOpenCLI(args: string) {
    const response = await axios.post(`${API_BASE}/opencli`, { args });
    return response.data;
  },
  async readUrl(url: string) {
    const response = await axios.post(`${API_BASE}/web/reader`, { url });
    return response.data;
  },
  async getMyBotStatus() {
    const response = await axios.get(`${API_BASE}/bot/my-status`);
    return response.data;
  },

  // 🖥️ Desk & Shelves
  async getShelves() {
    const res = await axios.get(`${API_BASE}/desk/shelves`);
    return res.data;
  },
  async createShelf(data: { name: string, color?: string, sortOrder?: number }) {
    const res = await axios.post(`${API_BASE}/desk/shelves`, data);
    return res.data;
  },
  async updateShelf(id: string, data: { name?: string, color?: string, sortOrder?: number }) {
    const res = await axios.put(`${API_BASE}/desk/shelves/${id}`, data);
    return res.data;
  },
  async deleteShelf(id: string) {
    const res = await axios.delete(`${API_BASE}/desk/shelves/${id}`);
    return res.data;
  },
  async duplicateShelf(id: string) {
    const res = await axios.post(`${API_BASE}/desk/shelves/${id}/duplicate`);
    return res.data;
  },
  async getDeskItems(shelfId?: string | 'null') {
    const res = await axios.get(`${API_BASE}/desk/items`, { params: { shelfId: shelfId || 'null' } });
    return res.data;
  },
  async addDeskItem(data: { type: string, refId: string, shelfId?: string | null, sortOrder?: number }) {
    const res = await axios.post(`${API_BASE}/desk/items`, data);
    return res.data;
  },
  async updateDeskItem(id: string, data: { shelfId?: string | null, sortOrder?: number }) {
    const res = await axios.put(`${API_BASE}/desk/items/${id}`, data);
    return res.data;
  },
  async deleteDeskItem(id: string) {
    const res = await axios.delete(`${API_BASE}/desk/items/${id}`);
    return res.data;
  },

  async getChatLogs(platform: string, query: string = '', startDate: string = '', endDate: string = '') {
    const params = new URLSearchParams({ platform, q: query, startDate, endDate });
    const response = await axios.get(`${API_BASE}/chat/logs?${params.toString()}`);
    return response.data;
  },

  // 📝 Integrated Remarks
  async getRemarks() {
    const res = await axios.get(`${API_BASE}/chat/remarks`);
    return res.data;
  },
  async createRemark(data: { name: string; content?: string }) {
    const res = await axios.post(`${API_BASE}/chat/remarks`, data);
    return res.data;
  },
  async updateRemark(id: string, data: { name: string; content?: string }) {
    const res = await axios.put(`${API_BASE}/chat/remarks/${id}`, data);
    return res.data;
  },
  async deleteRemark(id: string) {
    const res = await axios.delete(`${API_BASE}/chat/remarks/${id}`);
    return res.data;
  },
  async toggleIntegration(logId: number) {
    const res = await axios.post(`${API_BASE}/chat/remarks/toggle`, { logId: Number(logId) });
    return res.data;
  },
  async moveRemarkItem(itemId: string, containerId: string | null, sortOrder: number = 0) {
    const res = await axios.post(`${API_BASE}/chat/remarks/move`, { itemId, containerId, sortOrder });
    return res.data;
  },
  async removeRemarkItem(id: string) {
    const res = await axios.delete(`${API_BASE}/chat/remarks/items/${id}`);
    return res.data;
  }
};

