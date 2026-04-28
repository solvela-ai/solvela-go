package solvela

import (
	"sync"
	"time"
)

// BalanceMonitor periodically checks wallet balance and fires callbacks on low balance.
type BalanceMonitor struct {
	fetchBalance        func() (float64, error)
	pollInterval        time.Duration
	lowBalanceThreshold *float64
	onLowBalance        func(float64)

	mu      sync.Mutex
	balance *float64
	wasLow  bool
	stopCh  chan struct{}
	stopped bool
}

// NewBalanceMonitor creates a new balance monitor.
func NewBalanceMonitor(
	fetchBalance func() (float64, error),
	pollInterval time.Duration,
	lowBalanceThreshold *float64,
	onLowBalance func(float64),
) *BalanceMonitor {
	return &BalanceMonitor{
		fetchBalance:        fetchBalance,
		pollInterval:        pollInterval,
		lowBalanceThreshold: lowBalanceThreshold,
		onLowBalance:        onLowBalance,
		stopCh:              make(chan struct{}),
	}
}

// Start begins polling in a background goroutine.
func (m *BalanceMonitor) Start() {
	go m.run()
}

// Stop halts the polling loop. Safe to call multiple times.
func (m *BalanceMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
}

// LastKnownBalance returns the most recently fetched balance, or nil if none.
func (m *BalanceMonitor) LastKnownBalance() *float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balance
}

func (m *BalanceMonitor) run() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	m.poll() // initial poll

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

func (m *BalanceMonitor) poll() {
	balance, err := m.fetchBalance()
	if err != nil {
		return // swallow errors
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.balance = &balance

	if m.lowBalanceThreshold != nil && m.onLowBalance != nil {
		isLow := balance < *m.lowBalanceThreshold
		if isLow && !m.wasLow {
			m.onLowBalance(balance)
		}
		m.wasLow = isLow
	}
}
