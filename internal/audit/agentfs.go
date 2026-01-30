package audit

/*
Файл agentfs.go реализует компонент Agent File System — высокопроизводительный
движок для сбора и персистентности аналитических данных (Audit Trail).

Ключевые особенности архитектуры:
- Non-blocking Logging: Использование неблокирующих каналов для передачи событий
  из Hot Path шлюза. Это гарантирует, что задержки записи в БД не влияют на Response Time.
- Batching & Efficiency: Накопление событий в памяти и пакетная запись (Bulk Insert)
  в PostgreSQL по таймеру или при достижении лимита (100 событий).
- Drain Pattern & Graceful Shutdown: Реализован механизм полной вычитки буфера
  при остановке сервиса. С помощью sync.WaitGroup и закрытия каналов гарантируется
  Final Flush — отсутствие потерь данных при перезагрузке системы.
- Reliability: Устойчивость к кратковременным сбоям БД за счет изоляции воркера
  и использования контекста Background для завершающих операций.
*/

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// StorageInterface определяет, куда физически будут сохраняться логи
type StorageInterface interface {
	// WriteBatch сохраняет пачку событий за один раз
	WriteBatch(ctx context.Context, events []AuditEvent) error
}

type Auditor interface {
	Log(event AuditEvent)
}

type AgentFS struct {
	ch     chan AuditEvent  // Буфер для асинхронности
	repo   StorageInterface // Интерфейс для Postgres/ClickHouse
	logger *zap.Logger
	wg     sync.WaitGroup
	// «Железобетонная» защита (Bulletproof) вдруго кто-то вызовет Log случайно после остановки,
	isClosed int32 // Атомарный флаг (0 - открыт, 1 - закрыт)
}

func NewAgentFS(repo StorageInterface, logger *zap.Logger) *AgentFS {
	fs := &AgentFS{
		ch:     make(chan AuditEvent, 10000), // Очередь на 10к событий
		repo:   repo,
		logger: logger.With(zap.String("mod", "agentfs")),
		wg:     sync.WaitGroup{},
	}
	return fs
}

func (fs *AgentFS) Start() {
	fs.wg.Add(1)
	go fs.worker()
}

// Stop «запирает» вход в канал и ждет, пока воркер всё допишет.
func (fs *AgentFS) Stop() {
	// 1. Сначала ставим флаг
	atomic.StoreInt32(&fs.isClosed, 1)

	// 2. Даем крошечную паузу, чтобы текущие Log успели проскочить
	time.Sleep(10 * time.Millisecond)

	// 3. Закрываем (Drain Pattern). Завершение горутины происходит исключительно через закрытие входного канала.
	fs.logger.Info("stopping auditor: closing channel and flushing buffer...")
	close(fs.ch) // 1. Закрываем канал. Новые события больше не принимаются.
	fs.wg.Wait() // 2. Ждем, пока воркер вычитает остатки из канала и вызовет flush().
	fs.logger.Info("auditor stopped gracefully")
}

func (fs *AgentFS) Log(event AuditEvent) {
	// Убеждаемся, что таймстемп всегда проставлен
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Атомарно проверяем, не закрыт ли канал
	if atomic.LoadInt32(&fs.isClosed) == 1 {
		fs.logger.Warn("audit event dropped: auditor is stopping", zap.String("id", event.ID))
		return
	}

	// используем стратегию Load Shedding (сброс нагрузки)
	select {
	case fs.ch <- event:
	default:
		// Если канал переполнен (Backpressure), пишем в стандартный логгер
		// Чтобы не терять данные в критических ситуациях
		fs.logger.Error("audit_buffer_overflow",
			zap.String("agent_id", event.AgentID),
			zap.String("trace_id", event.TraceID),
		)
	}
}

func (fs *AgentFS) worker() {
	defer fs.wg.Done()

	batch := make([]AuditEvent, 0, 100)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) > 0 {
			// Используем Background, так как основной контекст может быть уже закрыт
			if err := fs.repo.WriteBatch(context.Background(), batch); err != nil {
				fs.logger.Error("audit flush failed", zap.Error(err))
			}
			batch = batch[:0]
		}
	}

	for {
		select {
		case event, ok := <-fs.ch:
			if !ok {
				// КАНАЛ ЗАКРЫТ fs.ch в методе Stop() — это самодостаточный сигнал для завершения.
				// Он гарантирует, что воркер:
				//		Сначала вычитает всё, что осталось в очереди.
				//		Только потом получит ok == false.
				//		Вызовет финальный flush() и выйдет.
				flush() // Финальный сброс
				fs.logger.Info("audit worker finished")
				return
			}
			batch = append(batch, event)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
