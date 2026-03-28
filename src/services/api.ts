import axios from 'axios';
import { io } from 'socket.io-client';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';
const API_BASE = `${API_URL}/api`;

// Add ngrok skip warning header to all axios requests
axios.defaults.headers.common['ngrok-skip-browser-warning'] = 'true';

export const socket = io(API_URL, {
  transports: ['websocket']
});

export const apiService = {
  // Devices
  async registerDevice(id: string, userAgent: string) {
    const response = await axios.post(`${API_BASE}/devices/register`, { id, userAgent });
    return response.data;
  },
  async getDevices() {
    const response = await axios.get(`${API_BASE}/devices`);
    return response.data;
  },
  async updateDeviceStatus(id: string, status: string, deviceName?: string, userId?: string) {
    const response = await axios.post(`${API_BASE}/devices/status`, { id, status, deviceName, userId });
    return response.data;
  },
  async deleteDevice(id: string) {
    const response = await axios.delete(`${API_BASE}/devices/${id}`);
    return response.data;
  },

  // Users
  async getUsers() {
    const response = await axios.get(`${API_BASE}/users`);
    return response.data;
  },
  async createUser(name: string, isAdmin: boolean) {
    const response = await axios.post(`${API_BASE}/users`, { name, isAdmin });
    return response.data;
  },

  // Common State
  async getCommonState() {
    const response = await axios.get(`${API_BASE}/common`);
    return response.data;
  },
  async updateCommonState(key: 'text' | 'image', data: { content?: string; fileUrl?: string; fileName?: string; userId?: string }) {
    const response = await axios.post(`${API_BASE}/common/update`, { key, ...data });
    return response.data;
  },

  // Snippets
  async getSnippets(userId: string, parentId: string | null = null, all: boolean = false) {
    const response = await axios.get(`${API_BASE}/snippets`, { params: { userId, parentId: parentId || 'root', all } });
    return response.data;
  },
  async createSnippet(data: { userId: string; parentId: string | null; name: string; content?: string; isFolder: boolean }) {
    const response = await axios.post(`${API_BASE}/snippets`, data);
    return response.data;
  },
  async deleteSnippet(id: string) {
    const response = await axios.delete(`${API_BASE}/snippets/${id}`);
    return response.data;
  },
  async updateSnippet(id: string, data: { name: string; content?: string }) {
    const response = await axios.put(`${API_BASE}/snippets/${id}`, data);
    return response.data;
  },

  // Files
  async uploadFile(file: File) {
    const formData = new FormData();
    formData.append('file', file);
    const response = await axios.post(`${API_BASE}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
    return response.data;
  }
};
