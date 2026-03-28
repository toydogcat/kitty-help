<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { apiService, socket } from '../services/api';

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
          <li v-for="device in pendingDevices" :key="device.id" class="device-item">
            <div class="device-info">
              <span class="device-id">ID: {{ device.id.substring(0, 8) }}...</span>
              <span class="device-ua">{{ device.user_agent?.split(')')[0] + ')' }}</span>
            </div>
            <div class="device-actions">
              <button @click="approveDevice(device.id)" class="approve-btn">Approve</button>
              <button @click="removeDevice(device.id)" class="remove-btn">Remove</button>
            </div>
          </li>
        </ul>
      </section>

      <!-- Approved Devices Section -->
      <section class="admin-section">
        <h3>📱 Approved Devices ({{ approvedDevices.length }})</h3>
        <ul v-if="approvedDevices.length > 0" class="device-list">
          <li v-for="device in approvedDevices" :key="device.id" class="device-item approved">
            <div class="device-info">
              <span class="device-name">{{ device.device_name }}</span>
              <span class="device-id">ID: {{ device.id.substring(0, 8) }}...</span>
              
              <!-- User Assignment Dropdown -->
              <div class="user-assignment">
                <label>Assigned to:</label>
                <select 
                  :value="device.user_id" 
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
}

.device-item.approved {
  border-left: 4px solid var(--primary-color);
}

.device-info {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  flex: 1;
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
