<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { apiService, socket } from '../services/api';

const props = defineProps<{
  mode: 'home' | 'personal';
  userId?: string;
}>();

const currentDate = ref(new Date());
const events = ref<any[]>([]);
const showModal = ref(false);
const selectedDate = ref('');
const editContent = ref('');
const isSaving = ref(false);
const users = ref<any[]>([]);

// Calendar Logic
const monthNames = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
const daysOfWeek = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const year = computed(() => currentDate.value.getFullYear());
const month = computed(() => currentDate.value.getMonth());

const firstDayOfMonth = computed(() => new Date(year.value, month.value, 1).getDay());
const daysInMonth = computed(() => new Date(year.value, month.value + 1, 0).getDate());

const calendarDays = computed(() => {
  const days = [];
  // Fill empty slots for previous month
  for (let i = 0; i < firstDayOfMonth.value; i++) {
    days.push({ day: null });
  }
  // Fill current month days
  for (let i = 1; i <= daysInMonth.value; i++) {
    const dateStr = `${year.value}-${String(month.value + 1).padStart(2, '0')}-${String(i).padStart(2, '0')}`;
    const dayEvents = events.value.filter(e => e.eventDate === dateStr);
    days.push({ day: i, dateStr, events: dayEvents });
  }
  return days;
});

const prevMonth = () => {
  currentDate.value = new Date(year.value, month.value - 1, 1);
};

const nextMonth = () => {
  currentDate.value = new Date(year.value, month.value + 1, 1);
};

// Data Fetching
const fetchEvents = async () => {
  try {
    events.value = await apiService.getCalendarEvents();
  } catch (err) {
    console.error("Failed to fetch events:", err);
  }
};

const fetchUsers = async () => {
  try {
    users.value = await apiService.getUsers();
  } catch (err) {
    console.error("Failed to fetch users:", err);
  }
};

onMounted(() => {
  fetchEvents();
  fetchUsers();
  socket.on('calendarUpdate', fetchEvents);
});

onUnmounted(() => {
  socket.off('calendarUpdate', fetchEvents);
});

// Event Handling
const openDay = (day: any) => {
  if (!day.day) return;
  selectedDate.value = day.dateStr;
  
  if (props.mode === 'personal' && props.userId) {
    const myEvent = day.events.find((e: any) => e.userId === props.userId);
    editContent.value = myEvent ? myEvent.content : '';
    showModal.value = true;
  } else if (props.mode === 'home') {
    if (day.events.length > 0) {
      showModal.value = true;
    }
  }
};

const saveEvent = async () => {
  if (!props.userId || !selectedDate.value) return;
  isSaving.value = true;
  try {
    await apiService.updateCalendarEvent(selectedDate.value, editContent.value);
    showModal.value = false;
  } catch (err) {
    alert("Save failed");
  } finally {
    isSaving.value = false;
  }
};

const getUserColor = (userName: string) => {
  const colors: any = {
    'Toby': '#00f2ff', // Cyan
    '爸爸': '#ff9f43', // Orange
    '媽媽': '#ff6b6b', // Pink/Red
    '如如': '#a29bfe'  // Purple
  };
  return colors[userName] || 'var(--primary-color)';
};

const selectedDateEvents = computed(() => {
  return events.value.filter(e => e.eventDate === selectedDate.value);
});
</script>

<template>
  <div class="family-calendar card" :class="mode">
    <div class="calendar-header">
      <div class="month-selector">
        <button @click="prevMonth" class="nav-btn">❮</button>
        <h2>{{ monthNames[month] }} {{ year }}</h2>
        <button @click="nextMonth" class="nav-btn">❯</button>
      </div>
      <div v-if="mode === 'home'" class="legend">
        <div v-for="u in users" :key="u.id" class="legend-item">
          <span class="dot" :style="{ background: getUserColor(u.name) }"></span>
          {{ u.name }}
        </div>
      </div>
    </div>

    <div class="calendar-grid">
      <div v-for="day in daysOfWeek" :key="day" class="weekday">{{ day }}</div>
      <div 
        v-for="(d, idx) in calendarDays" 
        :key="idx" 
        class="day-cell"
        :class="{ 'has-events': d.events?.length, 'empty': !d.day, 'today': d.dateStr === new Date().toISOString().split('T')[0] }"
        @click="openDay(d)"
      >
        <span class="day-num" v-if="d.day">{{ d.day }}</span>
        <div class="event-dots" v-if="d.events?.length">
          <span 
            v-for="e in d.events" 
            :key="e.id" 
            class="event-dot" 
            :style="{ background: getUserColor(e.userName) }"
            :title="e.userName"
          ></span>
        </div>
      </div>
    </div>

    <!-- Modal -->
    <Transition name="fade">
      <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
        <div class="modal-content card">
          <div class="modal-header">
            <h3>📅 {{ selectedDate }}</h3>
            <button @click="showModal = false" class="close-btn">&times;</button>
          </div>
          
          <div v-if="mode === 'personal'" class="edit-mode">
            <textarea 
              v-model="editContent" 
              placeholder="What's happening today?..."
              rows="5"
            ></textarea>
            <div class="modal-actions">
              <button @click="saveEvent" :disabled="isSaving" class="btn primary">
                {{ isSaving ? 'Saving...' : 'Save Note' }}
              </button>
            </div>
          </div>
          
          <div v-else class="view-mode">
            <div v-if="selectedDateEvents.length" class="event-list">
              <div v-for="e in selectedDateEvents" :key="e.id" class="event-item">
                <span class="user-tag" :style="{ background: getUserColor(e.userName) + '22', color: getUserColor(e.userName), border: '1px solid ' + getUserColor(e.userName) }">
                  {{ e.userName }}
                </span>
                <p>{{ e.content }}</p>
              </div>
            </div>
            <div v-else class="no-events">No entries for this day.</div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.family-calendar {
  padding: 1.5rem;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 20px;
}

.calendar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
  gap: 1rem;
}

.month-selector {
  display: flex;
  align-items: center;
  gap: 1.5rem;
}

.month-selector h2 {
  min-width: 180px;
  margin: 0;
  font-size: 1.4rem;
  color: var(--primary-color);
}

.nav-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color);
  color: white;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.nav-btn:hover {
  background: var(--primary-color);
  border-color: var(--primary-color);
}

.legend {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  opacity: 0.8;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 1px;
  background: var(--border-color);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  overflow: hidden;
}

.weekday {
  background: rgba(0, 0, 0, 0.2);
  padding: 0.8rem 0;
  font-weight: 600;
  font-size: 0.8rem;
  color: var(--secondary-color);
  border-bottom: 1px solid var(--border-color);
}

.day-cell {
  background: var(--card-bg);
  height: 100px;
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
}

.day-cell:not(.empty):hover {
  background: rgba(255, 255, 255, 0.05);
}

.day-cell.today {
  background: rgba(var(--primary-rgb), 0.05);
}

.day-cell.today .day-num {
  color: var(--primary-color);
  font-weight: bold;
}

.day-num {
  font-size: 0.9rem;
  opacity: 0.6;
}

.event-dots {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  justify-content: center;
  margin-top: auto;
  padding-bottom: 0.4rem;
}

.event-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  box-shadow: 0 0 6px rgba(0,0,0,0.5);
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  backdrop-filter: blur(8px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
}

.modal-content {
  width: 100%;
  max-width: 500px;
  padding: 2rem;
  border: 1px solid var(--border-color);
  box-shadow: 0 20px 50px rgba(0,0,0,0.5);
  animation: modalEnter 0.3s ease-out;
}

@keyframes modalEnter {
  from { opacity: 0; transform: scale(0.9); }
  to { opacity: 1; transform: scale(1); }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.close-btn {
  background: none;
  border: none;
  font-size: 2rem;
  color: var(--text-color);
  cursor: pointer;
  opacity: 0.5;
}

textarea {
  width: 100%;
  padding: 1rem;
  background: rgba(0,0,0,0.3);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  color: var(--text-color);
  font-size: 1rem;
  resize: none;
  font-family: inherit;
}

.modal-actions {
  margin-top: 1.5rem;
  display: flex;
  justify-content: flex-end;
}

.btn.primary {
  background: var(--primary-color);
  color: white;
  padding: 0.8rem 2rem;
  border-radius: 8px;
  font-weight: 600;
  border: none;
  cursor: pointer;
}

.event-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.event-item {
  text-align: left;
}

.user-tag {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: bold;
  margin-bottom: 0.5rem;
}

.no-events {
  opacity: 0.5;
  font-style: italic;
  padding: 2rem 0;
}

@media (max-width: 768px) {
  .day-cell { height: 70px; }
  .month-selector h2 { font-size: 1.1rem; min-width: 140px; }
  .legend { display: none; }
}

/* Transitions */
.fade-enter-active, .fade-leave-active { transition: opacity 0.3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
