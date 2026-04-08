package bots

import (
	"context"
	"log"
	"sync"
)

type BotChannel interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	SendMessage(targetID string, text string) error
}

type Manager struct {
	bots map[string]BotChannel
	mu   sync.RWMutex
}

var BotManager *Manager

func InitManager() {
	BotManager = &Manager{
		bots: make(map[string]BotChannel),
	}
}

func (m *Manager) Register(name string, bot BotChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bots[name] = bot
}

func (m *Manager) StartAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, bot := range m.bots {
		log.Printf("🤖 Starting bot channel: %s", name)
		if err := bot.Start(ctx); err != nil {
			log.Printf("❌ Failed to start bot %s: %v", name, err)
		}
	}
}

func (m *Manager) StopAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, bot := range m.bots {
		log.Printf("🛑 Stopping bot channel: %s", name)
		_ = bot.Stop(ctx)
	}
}

func (m *Manager) Get(name string) (BotChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	bot, ok := m.bots[name]
	return bot, ok
}
