import express, { Request, Response } from 'express';
import http from 'http';
import { Server } from 'socket.io';
import cors from 'cors';
import multer from 'multer';
import path from 'path';
import fs from 'fs';
import { pool } from './db';

const app = express();
const ADMIN_EMAIL = 'toydogcat@gmail.com';
const server = http.createServer(app);

// Ensure uploads directory exists (use project root for consistency)
const uploadsDir = path.resolve(process.cwd(), 'uploads');
if (!fs.existsSync(uploadsDir)) {
  fs.mkdirSync(uploadsDir, { recursive: true });
}

// Configure multer
const storage = multer.diskStorage({
  destination: (req, file, cb) => {
    cb(null, uploadsDir);
  },
  filename: (req, file, cb) => {
    const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9);
    cb(null, uniqueSuffix + path.extname(file.originalname));
  }
});
const upload = multer({ storage: storage });

// --- Aggressive CORS Middleware ---
app.use((req, res, next) => {
  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url} - Origin: ${req.headers.origin}`);
  const origin = req.headers.origin;
  if (origin) {
    res.setHeader('Access-Control-Allow-Origin', origin);
  } else {
    res.setHeader('Access-Control-Allow-Origin', '*');
  }
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept, Authorization, ngrok-skip-browser-warning, cf-skip-browser-warning');
  res.setHeader('Access-Control-Allow-Credentials', 'true');
  res.setHeader('Access-Control-Max-Age', '86400'); // 24 hours
  
  // Handle Preflight (OPTIONS)
  if (req.method === 'OPTIONS') {
    return res.status(200).end();
  }
  next();
});

const io = new Server(server, {
  cors: {
    origin: (origin, callback) => {
      // Reflect origin for Socket.io too
      callback(null, true);
    },
    credentials: true
  }
});

app.use(express.json());
app.use('/uploads', express.static(uploadsDir));

// --- Database Initialization ---
const initDb = async () => {
  try {
    // 1. Users table
    await pool.query(`
      CREATE TABLE IF NOT EXISTS users (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        name TEXT UNIQUE NOT NULL,
        role TEXT DEFAULT 'user', -- 'admin', 'subadmin', 'user'
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `);

    // 2. Insert default users if empty
    const usersCount = await pool.query('SELECT count(*) FROM users');
    if (parseInt(usersCount.rows[0].count) === 0) {
      const defaultUsers = [
        { name: 'Toby', role: 'admin' },
        { name: '爸爸', role: 'user' },
        { name: '媽媽', role: 'user' },
        { name: '如如', role: 'user' }
      ];
      for (const u of defaultUsers) {
        await pool.query('INSERT INTO users (name, role) VALUES ($1, $2)', [u.name, u.role]);
      }
      console.log('Default users with roles created');
    }

    // Ensure Toby is always admin (for migration of existing DBs)
    await pool.query("ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user'");
    await pool.query("UPDATE users SET role = 'admin' WHERE name = 'Toby'");

    // 3. Devices table
    await pool.query(`
      CREATE TABLE IF NOT EXISTS devices (
        id TEXT PRIMARY KEY,
        status TEXT DEFAULT 'pending',
        device_name TEXT,
        user_agent TEXT,
        last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `);
    
    // Add missing columns if they don't exist
    await pool.query("ALTER TABLE devices ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)");
    await pool.query("ALTER TABLE devices ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP");

    // Bulletin board table
    await pool.query(`
      CREATE TABLE IF NOT EXISTS bulletin (
        id SERIAL PRIMARY KEY,
        message TEXT,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `);

    // Ensure at least one row exists
    const res = await pool.query('SELECT COUNT(*) FROM bulletin');
    if (res.rows[0].count === '0') {
      await pool.query("INSERT INTO bulletin (message) VALUES ('Welcome to kitty-help! Admin has not set any notice yet.')");
    }

    // 4. Common State table
    await pool.query(`
      CREATE TABLE IF NOT EXISTS common_state (
        key TEXT PRIMARY KEY,
        content TEXT,
        file_url TEXT,
        file_name TEXT,
        updated_by UUID REFERENCES users(id),
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `);

    // Initialize common state entries if not exist
    await pool.query("INSERT INTO common_state (key, content) VALUES ('text', 'Welcome to kitty-help') ON CONFLICT DO NOTHING");
    await pool.query("INSERT INTO common_state (key, file_url) VALUES ('image', '') ON CONFLICT DO NOTHING");

    // 5. Snippets table (Hierarchical)
    await pool.query(`
      CREATE TABLE IF NOT EXISTS snippets (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        user_id UUID REFERENCES users(id) ON DELETE CASCADE,
        parent_id UUID REFERENCES snippets(id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        content TEXT,
        is_folder BOOLEAN DEFAULT false,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `);

    // 4. Common text history table (FIFO 10)
    await pool.query(`
      CREATE TABLE IF NOT EXISTS common_text_history (
        id SERIAL PRIMARY KEY,
        content TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        user_id UUID REFERENCES users(id) ON DELETE SET NULL
      )
    `);

    // 5. Calendar events table
    await pool.query(`
      CREATE TABLE IF NOT EXISTS calendar_events (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        user_id UUID REFERENCES users(id) ON DELETE CASCADE,
        event_date TEXT NOT NULL, -- Format: YYYY-MM-DD
        content TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, event_date)
      )
    `);
    console.log('Database tables initialized');
  } catch (err) {
    console.error('Error initializing database:', err);
  }
};

// initDb(); // Replaced with awaited call below

// --- API Endpoints ---

// Snippets CRUD
app.get('/api/snippets', async (req: Request, res: Response) => {
  const { userId, parentId, all } = req.query;
  try {
    let query = 'SELECT * FROM snippets WHERE user_id = $1';
    let params: any[] = [userId];

    if (all === 'true') {
      // Return everything for the user (useful for building tree view)
    } else if (parentId === 'root' || !parentId) {
      query += ' AND parent_id IS NULL';
    } else {
      query += ' AND parent_id = $2';
      params.push(parentId as string);
    }
    
    query += ' ORDER BY is_folder DESC, name ASC';
    const result = await pool.query(query, params);
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch snippets' });
  }
});

app.post('/api/snippets', async (req: Request, res: Response) => {
  const { userId, parentId, name, content, isFolder } = req.body;
  try {
    const result = await pool.query(
      'INSERT INTO snippets (user_id, parent_id, name, content, is_folder) VALUES ($1, $2, $3, $4, $5) RETURNING *',
      [userId, parentId === 'root' ? null : parentId, name, content, isFolder]
    );
    res.json(result.rows[0]);
  } catch (err) {
    res.status(500).json({ error: 'Snippet creation failed' });
  }
});

app.delete('/api/snippets/:id', async (req: Request, res: Response) => {
  const { id } = req.params;
  try {
    await pool.query('DELETE FROM snippets WHERE id = $1', [id]);
    res.json({ success: true });
  } catch (err) {
    res.status(500).json({ error: 'Delete failed' });
  }
});

app.put('/api/snippets/:id', async (req: Request, res: Response) => {
  const { id } = req.params;
  const { name, content } = req.body;
  try {
    const result = await pool.query(
      'UPDATE snippets SET name = $1, content = $2 WHERE id = $3 RETURNING *',
      [name, content, id]
    );
    res.json(result.rows[0]);
  } catch (err) {
    res.status(500).json({ error: 'Update failed' });
  }
});

// Users Management
app.get('/api/users', async (req: Request, res: Response) => {
  try {
    const result = await pool.query('SELECT * FROM users ORDER BY name');
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch users' });
  }
});

app.post('/api/users', async (req: Request, res: Response) => {
  const { name, isAdmin } = req.body;
  try {
    const result = await pool.query(
      'INSERT INTO users (name, is_admin) VALUES ($1, $2) RETURNING *',
      [name, isAdmin || false]
    );
    res.json(result.rows[0]);
    io.emit('usersUpdate');
  } catch (err) {
    res.status(500).json({ error: 'User creation failed' });
  }
});

// Device Management
app.get('/api/devices', async (req: Request, res: Response) => {
  try {
    const result = await pool.query(`
      SELECT d.*, u.name as user_name 
      FROM devices d 
      LEFT JOIN users u ON d.user_id = u.id 
      ORDER BY d.created_at DESC
    `);
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Database error' });
  }
});

 // Helper to check if a user is admin or subadmin
const hasAdminAccess = async (deviceId: string, adminEmail?: string) => {
  if (adminEmail === ADMIN_EMAIL) return true;
  if (!deviceId) return false;
  
  const res = await pool.query(`
    SELECT u.role 
    FROM devices d
    JOIN users u ON d.user_id = u.id
    WHERE d.id = $1 AND d.status = 'approved'
  `, [deviceId]);
  
  if (res.rows.length > 0) {
    const role = res.rows[0].role;
    return role === 'admin' || role === 'subadmin';
  }
  return false;
};

// Endpoints
app.post('/api/register', async (req, res) => {
  const { id, userAgent } = req.body;
  try {
    const check = await pool.query('SELECT * FROM devices WHERE id = $1', [id]);
    if (check.rows.length === 0) {
      await pool.query(
        'INSERT INTO devices (id, user_agent, status) VALUES ($1, $2, $3)',
        [id, userAgent, 'pending']
      );
      io.emit('newDevice', { id, userAgent });
    }
    
    // Update last_active and get current status + user role
    const updated = await pool.query(`
      UPDATE devices d
      SET last_active = CURRENT_TIMESTAMP
      FROM users u
      WHERE d.id = $1 AND d.user_id = u.id
      RETURNING d.*, u.role as user_role, u.name as user_name
    `, [id]);

    if (updated.rows.length > 0) {
      res.json(updated.rows[0]);
    } else {
      const basic = await pool.query('SELECT * FROM devices WHERE id = $1', [id]);
      res.json(basic.rows[0]);
    }
  } catch (err) {
    res.status(500).json({ error: 'Registration failed' });
  }
});

app.post('/api/devices/status', async (req: Request, res: Response) => {
  const { id, status, deviceName, userId } = req.body;
  try {
    await pool.query(
      'UPDATE devices SET status = $1, device_name = $2, user_id = $3 WHERE id = $4',
      [status, deviceName, userId, id]
    );
    res.json({ success: true });
    io.emit('deviceStatusUpdate', { id, status });
  } catch (err) {
    res.status(500).json({ error: 'Update failed' });
  }
});

app.delete('/api/devices/:id', async (req: Request, res: Response) => {
  const { id } = req.params;
  try {
    await pool.query('DELETE FROM devices WHERE id = $1', [id]);
    res.json({ success: true });
    io.emit('deviceRemoved', id);
  } catch (err) {
    res.status(500).json({ error: 'Delete failed' });
  }
});

// Common State Management
app.get('/api/common', async (req: Request, res: Response) => {
  try {
    const result = await pool.query('SELECT * FROM common_state');
    const state = result.rows.reduce((acc, curr) => {
      acc[curr.key] = curr;
      return acc;
    }, {} as any);
    res.json(state);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch common state' });
  }
});

app.get('/api/common/history', async (req: Request, res: Response) => {
  try {
    const result = await pool.query(`
      SELECT h.*, u.name as user_name 
      FROM common_text_history h
      LEFT JOIN users u ON h.user_id = u.id
      ORDER BY h.created_at DESC 
      LIMIT 10
    `);
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch history' });
  }
});

app.post('/api/common/update', async (req: Request, res: Response) => {
  const { key, content, fileUrl, fileName, userId } = req.body;
  try {
    await pool.query(
      'UPDATE common_state SET content = $1, file_url = $2, file_name = $3, updated_by = $4, updated_at = CURRENT_TIMESTAMP WHERE key = $5',
      [content, fileUrl, fileName, userId, key]
    );

    // If it's text, also add to history
    if (key === 'text' && content) {
      await pool.query(
        'INSERT INTO common_text_history (content, user_id) VALUES ($1, $2)',
        [content, userId]
      );
      
      // Keep only 10 items (Clean up)
      await pool.query(`
        DELETE FROM common_text_history 
        WHERE id NOT IN (
          SELECT id FROM common_text_history 
          ORDER BY created_at DESC 
          LIMIT 10
        )
      `);

      const historyResult = await pool.query(`
        SELECT h.*, u.name as user_name 
        FROM common_text_history h
        LEFT JOIN users u ON h.user_id = u.id
        ORDER BY h.created_at DESC 
        LIMIT 10
      `);
      io.emit('commonHistoryUpdate', historyResult.rows);
    }

    const result = await pool.query('SELECT * FROM common_state WHERE key = $1', [key]);
    const updated = result.rows[0];
    res.json(updated);
    io.emit('commonUpdate', updated);
  } catch (err) {
    console.error('Update failed:', err);
    res.status(500).json({ error: 'Common update failed' });
  }
});

app.post('/api/upload', upload.single('file'), (req: Request, res: Response) => {
  if (!req.file) {
    return res.status(400).json({ error: 'No file uploaded' });
  }
  const fileUrl = `/uploads/${req.file.filename}`;
  res.json({ url: fileUrl, name: req.file.originalname });
});

// Bulletin Board Endpoints
app.get('/api/bulletin', async (req, res) => {
  try {
    const result = await pool.query('SELECT message FROM bulletin ORDER BY updated_at DESC LIMIT 1');
    res.json(result.rows[0]);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch bulletin' });
  }
});

app.post('/api/bulletin', async (req, res) => {
  const { message, adminEmail, deviceId } = req.body;
  const authorized = await hasAdminAccess(deviceId, adminEmail);
  if (!authorized) {
    return res.status(403).json({ error: 'Permission denied' });
  }

  try {
    await pool.query('UPDATE bulletin SET message = $1, updated_at = CURRENT_TIMESTAMP', [message]);
    io.emit('bulletinUpdate', { message });
    res.json({ success: true });
  } catch (err) {
    res.status(500).json({ error: 'Failed to update bulletin' });
  }
});

// Role Management (Primary Admin only)
app.post('/api/users/role', async (req, res) => {
  const { userId, role, adminEmail } = req.body;
  if (adminEmail !== ADMIN_EMAIL) {
    return res.status(403).json({ error: 'Only primary admin can manage roles' });
  }

  try {
    await pool.query('UPDATE users SET role = $1 WHERE id = $2', [role, userId]);
    io.emit('usersUpdate');
    res.json({ success: true });
  } catch (err) {
    res.status(500).json({ error: 'Failed to update role' });
  }
});

// Calendar API
app.get('/api/calendar', async (req: Request, res: Response) => {
  try {
    const result = await pool.query(`
      SELECT c.*, u.name as user_name 
      FROM calendar_events c
      JOIN users u ON c.user_id = u.id
      ORDER BY c.event_date ASC
    `);
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Failed to fetch calendar events' });
  }
});

app.post('/api/calendar', async (req: Request, res: Response) => {
  const { userId, date, content } = req.body;
  try {
    const result = await pool.query(`
      INSERT INTO calendar_events (user_id, event_date, content, updated_at)
      VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
      ON CONFLICT (user_id, event_date) 
      DO UPDATE SET content = EXCLUDED.content, updated_at = CURRENT_TIMESTAMP
      RETURNING *
    `, [userId, date, content]);
    
    res.json(result.rows[0]);
    io.emit('calendarUpdate');
  } catch (err) {
    res.status(500).json({ error: 'Failed to update calendar' });
  }
});

// Latency Tracking Socket
io.on('connection', (socket) => {
  socket.on('clientPing', (data) => {
    socket.emit('serverPong', {
      clientTime: data.time,
      serverTime: Date.now()
    });
  });
});

const PORT = 3000;

// Initialize Database before starting the server
initDb().then(() => {
  server.listen(PORT, '0.0.0.0', () => {
    console.log(`Server running on all interfaces (0.0.0.0) at port ${PORT}`);
  });
}).catch(err => {
  console.error("Critical failure during DB initialization:", err);
  process.exit(1);
});
