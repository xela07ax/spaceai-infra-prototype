package engine

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ListenStateResilient — универсальный цикл для "живучей" подписки на сигналы Redis.
// Обрабатывает переподключения, логирование и разбор сигналов.
func ListenStateResilient(
	ctx context.Context,
	rdb *redis.Client,
	logger *zap.Logger,
	channel string,
	onReconnect func() error, // Callback для синхронизации при переподключении
	onMessage func(id string, status bool), // Callback для обработки сообщения
) {
	for {
		pubsub := rdb.Subscribe(ctx, channel)

		// Проверка успешности подписки
		if _, err := pubsub.Receive(ctx); err != nil {
			logger.Error("failed to subscribe", zap.String("chan", channel), zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		// Вызываем синхронизацию (Init) при каждом успешном коннекте
		if err := onReconnect(); err != nil {
			logger.Error("sync failed on reconnect", zap.Error(err))
		}

		ch := pubsub.Channel()

	loop:
		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					break loop // Канал закрыт, идем на переподключение
				}

				// Разбор формата "agent_id:status"
				parts := strings.Split(msg.Payload, ":")
				if len(parts) != 2 {
					logger.Error("invalid signal format", zap.String("payload", msg.Payload))
					continue
				}

				agentID := parts[0]
				status := parts[1] == "true" || parts[1] == "on" // Гибкий парсинг

				onMessage(agentID, status)
			}
		}

		pubsub.Close()
		time.Sleep(1 * time.Second)
	}
}
