<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { apiService, socket } from '../services/api';

const props = defineProps<{
  currentDeviceId?: string;
}>();

const pendingDevices = ref<any[]>([]);
const approvedDevices = ref<any[]>([]);
const users = ref<any[]>([]);
const loading = ref(true);
const newUserName = ref('');

const fetchData = async () => {
  try {
    const [devices, userList] = await Promise.all([
      apiService.getDevices(),
      apiService.getUsers()
    ]);
    pendingDevices.value = devices.filter((d: any) => d.status === 'pending');
    approvedDevices.value = devices.filter((d: any) => d.status === 'approved');
    users.value = userList;
  } catch (err) {
    console.error("Failed to fetch data:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchData();

  socket.on('newDevice', fetchData);
  socket.on('deviceStatusUpdate', fetchData);
  socket.on('usersUpdate', fetchData);
});

// Grouping logic for approved devices
const groupedDevices = computed(() => {
  const groups: Record<string, any[]> = {};
  
  // Initialize groups with user names
  users.value.forEach(user => {
    groups[user.name] = [];
  });
  groups['Unassigned'] = [];

  approvedDevices.value.forEach(device => {
    const user = users.value.find(u => u.id === device.user_id);
    const groupName = user ? user.name : 'Unassigned';
    if (!groups[groupName]) groups[groupName] = [];
    groups[groupName].push(device);
  });

  // Filter out empty groups except for Unassigned if it has items
  return Object.fromEntries(
    Object.entries(groups).filter(([name, items]) => items.length > 0 || (name !== 'Unassigned' && users.value.some(u => u.name === name)))
  );
});

const formatTime = (dateStr: string) => {
  if (!dateStr) return 'Never';
  const date = new Date(dateStr);
  return date.toLocaleString('zh-TW', { 
    month: 'short', 
    day: 'numeric', 
    hour: '2-digit', 
    minute: '2-digit' 
  });
};

const approveDevice = async (deviceId: string) => {
  const deviceName = prompt('Enter a name for this device (e.g., Dad\'s iPad):');
  if (deviceName) {
    try {
      await apiService.updateDeviceStatus(deviceId, 'approved', deviceName);
      await fetchData();
    } catch (err) {
      alert("Approval failed.");
    }
  }
};

const assignUser = async (deviceId: string, userId: string, deviceName: string) => {
  try {
    await apiService.updateDeviceStatus(deviceId, 'approved', deviceName, userId);
    await fetchData();
  } catch (err) {
    alert("Assignment failed.");
  }
};

const revokeDevice = async (deviceId: string) => {
  if (confirm('Are you sure you want to revoke access?')) {
    try {
      await apiService.updateDeviceStatus(deviceId, 'revoked');
      await fetchData();
    } catch (err) {
      alert("Revoke failed.");
    }
  }
};

const removeDevice = async (deviceId: string) => {
  if (confirm('Permanently remove this device?')) {
    try {
      await apiService.deleteDevice(deviceId);
      await fetchData();
    } catch (err) {
      alert("Removal failed.");
    }
  }
};

const addUser = async () => {
  if (!newUserName.value.trim()) return;
  try {
    await apiService.createUser(newUserName.value, false);
    newUserName.value = '';
    await fetchData();
  } catch (err) {
    alert("Failed to add user.");
  }
};

const isCollapsed = ref(localStorage.getItem('admin_dashboard_collapsed') === 'true');
const toggleCollapse = () => {
  isCollapsed.value = !isCollapsed.value;
  localStorage.setItem('admin_dashboard_collapsed', isCollapsed.value.toString());
};
</script>

<template>
  <div class="admin-dashboard card" :class="{ collapsed: isCollapsed }">
    <div class="dashboard-header" @click="toggleCollapse">
      <h2>🛡️ Admin Dashboard</h2>
      <button class="collapse-toggle">
        {{ isCollapsed ? '▼ 展開 (Expand)' : '▲ 縮起 (Collapse)' }}
      </button>
    </div>
    
    <div v-if="!isCollapsed">
      <div v-if="loading" class="loader">Loading data...</div>
      
      <div v-else class="dashboard-content">
      <!-- User Management Section -->
      <section class="admin-section user-mgmt">
        <h3>👥 User Roles</h3>
        <div class="user-grid">
          <div v-for="user in users" :key="user.id" class="user-tag" :class="{ admin: user.is_admin }">
            {{ user.name }} {{ user.is_admin ? '(Admin)' : '' }}
          </div>
        </div>
        <div class="add-user">
          <input v-model="newUserName" placeholder="New User Name..." />
          <button @click="addUser" class="text-btn">+ Add User</button>
        </div>
      </section>

      <!-- Pending Devices Section -->
      <section v-if="pendingDevices.length > 0" class="admin-section">
        <h3>⏳ Pending Approval ({{ pendingDevices.length }})</h3>
        <ul class="device-list">
          <li v-for="device in pendingDevices" :key="device.id" class="device-item" :class="{ 'is-current': device.id === props.currentDeviceId }">
            <div class="device-info">
              <span class="device-id">
                ID: {{ device.id.substring(0, 8) }}...
                <span v-if="device.id === props.currentDeviceId" class="me-badge">YOU</span>
              </span>
              <span class="device-ua">{{ device.user_agent?.split(')')[0] + ')' }}</span>
              <span class="device-time">Last Active: {{ formatTime(device.last_active) }}</span>
            </div>
            <div class="device-actions">
              <button @click="approveDevice(device.id)" class="approve-btn">Approve</button>
              <button @click="removeDevice(device.id)" class="remove-btn">Remove</button>
            </div>
          </li>
        </ul>
      </section>

      <!-- Approved Devices Section (Grouped) -->
      <section class="admin-section">
        <h3>📱 Approved Devices ({{ approvedDevices.length }})</h3>
        <div v-if="approvedDevices.length > 0">
          <div v-for="(groupItems, groupName) in groupedDevices" :key="groupName" class="user-group">
            <h4 class="group-title">{{ groupName }}</h4>
            <ul class="device-list">
              <li v-for="device in groupItems" :key="device.id" class="device-item approved" :class="{ 'is-current': device.id === props.currentDeviceId }">
                <div class="device-info">
                  <div class="device-header">
                    <span class="device-name">{{ device.device_name }}</span>
                    <span v-if="device.id === props.currentDeviceId" class="me-badge">YOU</span>
                  </div>
                  <span class="device-id">ID: {{ device.id.substring(0, 8) }}...</span>
                  <span class="device-time">Last Active: {{ formatTime(device.last_active) }}</span>
                  
                  <!-- User Assignment Dropdown -->
                  <div class="user-assignment">
                    <label>Assign to:</label>
                    <select 
                      :value="device.user_id || ''" 
                      @change="(e: any) => assignUser(device.id, e.target.value, device.device_name)"
                    >
                      <option value="">Unassigned</option>
                      <option v-for="user in users" :key="user.id" :value="user.id">
                        {{ user.name }}
                      </option>
                    </select>
                  </div>
                </div>
                <div class="device-actions">
                  <button @click="revokeDevice(device.id)" class="revoke-btn">Revoke</button>
                  <button @click="removeDevice(device.id)" class="remove-btn">Remove</button>
                </div>
              </li>
            </ul>
          </div>
        </div>
        <p v-else class="empty-text">No approved devices yet.</p>
      </section>
    </div>
  </div>
</div>
</template>

<style scoped>
.admin-dashboard {
  text-align: left;
  max-width: 900px;
  margin: 2rem auto;
  padding: 2rem;
  transition: all 0.3s ease;
}

.admin-dashboard.collapsed {
  padding: 1rem 2rem;
  margin-bottom: 1rem;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  user-select: none;
}

.collapse-toggle {
  background: rgba(255,255,255,0.1);
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.3rem 0.8rem;
  border-radius: 6px;
  font-size: 0.85rem;
  cursor: pointer;
}

.collapse-toggle:hover {
  background: rgba(255,255,255,0.2);
}

.admin-section {
  margin-bottom: 2.5rem;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 1.5rem;
}

.user-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin: 1rem 0;
}

.user-tag {
  padding: 0.3rem 0.8rem;
  background: var(--border-color);
  border-radius: 99px;
  font-size: 0.9rem;
  color: var(--text-color);
}

.user-tag.admin {
  background: var(--primary-color);
  color: white;
}

.add-user {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.add-user input {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  padding: 0.3rem 0.6rem;
  border-radius: 4px;
  color: var(--text-color);
}

.device-list {
  list-style: none;
  padding: 0;
}

.device-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.2rem;
  background: rgba(255, 255, 255, 0.05);
  margin-bottom: 0.8rem;
  border-radius: 8px;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.device-item.is-current {
  background: rgba(var(--primary-rgb, 100, 108, 255), 0.15);
  border-color: var(--primary-color);
  box-shadow: 0 0 15px rgba(var(--primary-rgb, 100, 108, 255), 0.2);
}

.device-item.approved {
  border-left: 4px solid var(--primary-color);
}

.device-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}

.me-badge {
  background: var(--primary-color);
  color: white;
  font-size: 0.65rem;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  font-weight: bold;
  letter-spacing: 0.5px;
}

.device-info {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  flex: 1;
}

.user-group {
  margin-top: 1.5rem;
  margin-bottom: 2rem;
}

.group-title {
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: var(--secondary-color);
  margin-bottom: 1rem;
  padding-left: 0.5rem;
  border-left: 2px solid var(--secondary-color);
}

.device-time {
  font-size: 0.75rem;
  opacity: 0.6;
}

.user-assignment {
  margin-top: 0.5rem;
  font-size: 0.85rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.user-assignment select {
  background: var(--card-bg);
  color: var(--text-color);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 0.1rem 0.4rem;
}

.device-id {
  font-size: 0.75rem;
  font-family: monospace;
  opacity: 0.5;
}

.device-name {
  font-weight: bold;
  color: var(--primary-color);
  font-size: 1.1rem;
}

.device-actions {
  display: flex;
  gap: 0.5rem;
}

button {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: bold;
}

.approve-btn {
  background-color: var(--accent-color);
  color: white;
}

.revoke-btn {
  background-color: #ef4444;
  color: white;
}

.remove-btn {
  background-color: transparent;
  border: 1px solid #ef4444 !important;
  color: #ef4444;
}

.text-btn {
  color: var(--secondary-color);
  text-decoration: underline;
  background: none;
  font-size: 0.9rem;
}

.empty-text {
  text-align: center;
  opacity: 0.5;
  padding: 1rem;
}
</style>
