package engine

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryAuthInterceptor проверяет токен в метаданных gRPC вызова
func UnaryAuthInterceptor(u *UAGCore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 1. Извлекаем метаданные из контекста
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// 2. Ищем токен (в gRPC заголовки обычно в нижнем регистре)
		tokens := md.Get("x-devai-token")
		if len(tokens) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing access token")
		}

		token := tokens[0]

		// 3. Парсим Scopes (та же логика, что и в HTTP)
		scopes := make(map[string]bool)
		for _, s := range strings.Split(token, ",") {
			scopes[strings.TrimSpace(s)] = true
		}

		// 4. Обогащаем контекст для ProcessAction
		newCtx := context.WithValue(ctx, "user_scopes", scopes)

		// Идем дальше по цепочке
		return handler(newCtx, req)
	}
}
