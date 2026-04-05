# 🐱 Kitty-Help: Cross-Device Auxiliary Communication (PG Edition)

[![Frontend - Vue 3](https://img.shields.io/badge/Frontend-Vue%203-42b883?style=for-the-badge&logo=vue.js)](https://vuejs.org/)
[![Backend - Fiber](https://img.shields.io/badge/Backend-Go%20Fiber-00ADD8?style=for-the-badge&logo=go)](https://gofiber.io/)
[![Database - PostgreSQL](https://img.shields.io/badge/Database-PostgreSQL-336791?style=for-the-badge&logo=postgresql)](https://www.postgresql.org/)
[![Security - Firebase](https://img.shields.io/badge/Security-Firebase-FFCA28?style=for-the-badge&logo=firebase)](https://firebase.google.com/)

> **"A Professional Creative Suite for Family Knowledge Management."**

Kitty-Help is a high-fidelity, private knowledge management and communication infrastructure designed for seamless cross-device synchronization and high-security data custody. It bridges the gap between fragmented platform bots (Discord/Telegram/LINE) and a centralized, professional-grade web workstation.

## ✨ Highlights

### 🌌 Impression Knowledge Canvas
- **Bi-directional Synchronization**: Linked nodes are automatically mirrored in the Personal Snippets system.
- **Visual Discovery**: A high-density graph visualization (powered by Vis-network) for navigating complex knowledge relations.
- **Memory Recall (🎲 Random)**: A dedicated feature to stimulate cognitive recall by randomizing the graph focus.
- **Gallery Dock**: An intuitive "pull-tab" interface for managing the discovery queue.

### 🔐 Multi-Platform Security Custody
- **Dual-Bot Verification**: Access sensitive bookmarks and passwords requires real-time identity verification via Telegram/Discord bots.
- **Firebase Auth + JWT**: Industry-standard identity resolution paired with custom backend role management.
- **Grace Period Security**: Automated 2fa-unlocked windows for frictionless productivity after initial identity verification.

### 📋 Workstation Persistence
- **Seamless Resumption (無痛接續)**: The system remembers your exact navigation depth in file explorers, active tabs, and even your spatial coordinates on the knowledge graph.
- **Unified Snippet Explorer**: A hierarchical folder system that supports full Import/Export of knowledge structures.

## 🛠️ Tech Stack

### Frontend
- **Framework**: Vue 3 (Composition API) + Vite
- **Graphing**: Vis-network for massive node-edge visualization
- **State**: Reactive `localStorage` persistence patterns
- **Styling**: Vanilla high-end CSS with glassmorphism and adaptive dark mode

### Backend
- **Core**: Golang Fiber (High Performance)
- **Real-time**: Socket.io (Go implementation) with tunnel-optimized heartbeating
- **ORM**: pgx (Native PostgreSQL performance)
- **Bots**: Multi-threaded Bot Manager for Telegram, Discord, and LINE

## 🚀 Getting Started

1. **Clone the repository**
2. **Setup your `.env`**:
   - `DATABASE_URL` (PostgreSQL)
   - `FIREBASE_AUTH_JSON`
   - `TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`, etc.
3. **Run Services**:
   - Backend: `cd go-server && go run main.go` (or `air` for dev)
   - Frontend: `npm install && npm run dev`

---
*Created with ❤️ by **Toby** & **Antigravity AI**.*
