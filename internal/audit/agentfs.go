package audit

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// StorageInterface определяет, куда физически будут сохраняться логи
type StorageInterface interface {
	// WriteBatch сохраняет пачку событий за один раз
	WriteBatch(ctx context.Context, events []AuditEvent) error
}

type AgentFS struct {
	logger *zap.Logger      // Быстрый структурированный логгер
	ch     chan AuditEvent  // Буфер для асинхронности
	db     StorageInterface // Интерфейс для Postgres/ClickHouse
}

func NewAgentFS(db StorageInterface) *AgentFS {
	fs := &AgentFS{
		ch: make(chan AuditEvent, 10000), // Очередь на 10к событий
		db: db,
	}
	go fs.worker() // Запускаем фоновый процесс записи
	return fs
}

func (fs *AgentFS) Log(ev AuditEvent) {
	// Убеждаемся, что таймстемп всегда проставлен
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	select {
	case fs.ch <- ev:
	default:
		// Если канал переполнен (Backpressure), пишем в стандартный логгер
		// Чтобы не терять данные в критических ситуациях
		fs.logger.Error("audit_buffer_overflow",
			zap.String("agent_id", ev.AgentID),
			zap.String("trace_id", ev.TraceID),
		)
	}
}

func (fs *AgentFS) worker() {
	batch := make([]AuditEvent, 0, 100)
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case ev := <-fs.ch:
			batch = append(batch, ev)
			if len(batch) >= 100 {
				fs.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				fs.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (fs *AgentFS) flush(batch []AuditEvent) {
	if len(batch) == 0 {
		return
	}

	// Создаем контекст с таймаутом для записи
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if fs.db != nil {
		if err := fs.db.WriteBatch(ctx, batch); err != nil {
			fs.logger.Error("failed to write audit batch to storage",
				zap.Error(err),
				zap.Int("batch_size", len(batch)),
			)
			// Здесь можно реализовать retry-логику или сброс в файл
		}
	} else {
		// Если БД не подключена (как в нашем MVP main.go), просто выводим в stdout
		for _, ev := range batch {
			fs.logger.Info("audit_event_stub", zap.Any("event", ev))
		}
	}
}
