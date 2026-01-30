package engine

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// WarmupState — универсальная функция для прогрева L1 (RAM) и L2 (Redis) кэшей.
func WarmupState(
	ctx context.Context,
	rdb *redis.Client,
	logger *zap.Logger,
	ids []string,
	redisKey string,
	lockKey string,
	updateL1 func([]string), // Callback для обновления локальной мапы
) error {
	// 1. Обновляем локальный кэш (L1) через callback
	updateL1(ids)

	// 2. Распределенная блокировка (SetNX), чтобы только один инстанс обновлял Redis
	ok, err := rdb.SetNX(ctx, lockKey, "processing", 30*time.Second).Result()
	if err != nil || !ok {
		return nil // Либо ошибка сети, либо другой уже греет кэш
	}

	// 3. Проверка наполненности Redis
	count, err := rdb.SCard(ctx, redisKey).Result()
	if err != nil {
		count = 0
		logger.Warn("could not check Redis set size, proceeding with warm-up",
			zap.String("key", redisKey), zap.Error(err))
	}

	// 4. Если Redis пуст, а данные в БД есть — заливаем
	dbCount := len(ids)
	if count == 0 && dbCount > 0 {
		logger.Info("Redis cache is empty, performing warm-up from DB...",
			zap.String("key", redisKey), zap.Int("count", dbCount))

		pipe := rdb.Pipeline()
		for _, id := range ids {
			pipe.SAdd(ctx, redisKey, id)
		}
		_, err = pipe.Exec(ctx)
		return err
	}

	return nil
}
